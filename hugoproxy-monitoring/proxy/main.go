package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/config"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/controller"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/repository"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/cache"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db/adapter"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/geo_proxy"
	customMiddleware "gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/middleware"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/pprof"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/worker"
	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"

	_ "gitlab.com/s.izotov81/hugoproxy/docs"
)

// @title Геосервис API
// @version 1.0
// @description API для поиска адресов и геокодирования
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите JWT токен в формате: Bearer {your_token}
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Get().Warn("couldn't load .env file", zap.Error(err))
	}

	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logPath := os.Getenv("LOG_FILE_PATH")
	if err := logger.Init(logLevel, logPath); err != nil {
		logger.Get().Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer func() {
		if syncErr := logger.Get().Sync(); syncErr != nil {
			logger.Get().Warn("Failed to sync logger", zap.Error(syncErr))
		}
	}()

	log := logger.Get()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database
	dbConn, err := db.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbConn.Close()

	// Run migrations
	if err := db.RunMigrations(dbConn, cfg.Database.MigrationsPath); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize dependencies
	dependencies := initializeDependencies(cfg, dbConn)

	// Initialize router
	r := setupRouter(cfg, dependencies)
	r.Handle("/metrics", promhttp.Handler())

	// Create listener
	listener, err := net.Listen("tcp", cfg.Server.Addr())
	if err != nil {
		log.Fatal("Failed to create listener", zap.Error(err))
	}

	server := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	done := setupGracefulShutdown(server, cfg.Server.ShutdownTimeout, dependencies.worker, log)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal("Could not start server", zap.Error(err))
		}
	}()

	log.Info("Server starting", zap.String("addr", cfg.Server.Addr()))
	<-done
	log.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server shutdown failed", zap.Error(err))
	}

	log.Info("Server stopped gracefully")
}

// Dependencies содержит все зависимости приложения
type Dependencies struct {
	userController    *controller.UserController
	geoController     *controller.GeoController
	pprofController   *pprof.PprofController
	authMiddleware    func(next http.Handler) http.Handler
	worker            *worker.Worker
}

// initializeDependencies инициализирует все зависимости приложения
func initializeDependencies(cfg *config.Config, dbConn *sqlx.DB) *Dependencies {
	sqlAdapter := adapter.NewSQLAdapter(dbConn)
	userRepo := repository.NewUserRepository(sqlAdapter, dbConn)
	userService := service.NewUserService(userRepo)
	jsonResponder := responder.NewJSONResponder()
	userController := controller.NewUserController(userService, jsonResponder)

	pprofController := pprof.NewPprofController(jsonResponder)

	// Initialize geo service with caching
	realGeoService := service.NewGeoService(cfg.Dadata.APIKey, cfg.Dadata.SecretKey)
	memoryCache := cache.NewInMemoryCache()
	geoService := geo_proxy.NewGeoServiceProxy(realGeoService, memoryCache, 5*time.Minute)
	geoController := controller.NewGeoController(geoService, jsonResponder)

	// Initialize auth middleware
	authMiddleware := newAuthMiddleware(cfg.Auth.JWTSecret)

	// Initialize and start worker if enabled
	var workerInstance *worker.Worker
	if cfg.Worker.Enabled {
		workerInstance = worker.NewWorker(cfg.Worker.FilePath, cfg.Worker.Interval)
		workerInstance.Start()
	}

	return &Dependencies{
		userController:    userController,
		geoController:     geoController,
		pprofController:   pprofController,
		authMiddleware:    authMiddleware,
		worker:            workerInstance,
	}
}

// setupGracefulShutdown настраивает корректное завершение работы
func setupGracefulShutdown(server *http.Server, timeout time.Duration, worker *worker.Worker, log *zap.Logger) chan os.Signal {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-done
		if worker != nil {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			worker.Stop(ctx)
		}
	}()

	return done
}

func setupRouter(cfg *config.Config, deps *Dependencies) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(customMiddleware.RequestID)
	r.Use(metrics.HTTPMetricsMiddleware)
	r.Use(middleware.Recoverer)

	// Swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	// Custom Swagger UI with auto-auth
	r.Get("/swagger-ui/*", http.StripPrefix("/swagger-ui/", http.FileServer(http.Dir("./static"))).ServeHTTP)

	// Auth routes
	r.Group(func(r chi.Router) {
		r.Post("/api/register", RegisterHandler)
		r.Post("/api/login", LoginHandler)
	})

	// User routes
	r.Group(func(r chi.Router) {
		r.Use(deps.authMiddleware)
		r.Get("/api/users", deps.userController.ListUsers)
		r.Post("/api/users", deps.userController.RegisterUser)
		r.Get("/api/users/{id}", deps.userController.GetUser)
		r.Put("/api/users/{id}", deps.userController.UpdateUser)
		r.Delete("/api/users/{id}", deps.userController.DeleteUser)
		r.Get("/api/users/email", deps.userController.GetUserByEmail)
	})

	// Geo routes
	r.Group(func(r chi.Router) {
		r.Use(deps.authMiddleware)
		r.Post("/api/address/search", deps.geoController.Search)
		r.Post("/api/address/geocode", deps.geoController.Geocode)
	})

	// Protected pprof routes
	r.Group(func(r chi.Router) {
		r.Use(deps.authMiddleware)
		r.Mount("/mycustompath/pprof", pprof.Handler())
		r.Post("/api/pprof/cpu/start", deps.pprofController.StartCPUProfile)
		r.Post("/api/pprof/heap", deps.pprofController.TakeHeapProfile)
		r.Post("/api/pprof/trace/start", deps.pprofController.StartTraceProfile)
		r.Get("/api/pprof/profiles", deps.pprofController.ListProfiles)
	})

	return r
}
