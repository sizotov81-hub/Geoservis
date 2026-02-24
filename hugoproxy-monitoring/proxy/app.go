package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
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

// App представляет приложение
type App struct {
	cfg          *config.Config
	db           *sqlx.DB
	server       *http.Server
	logger       *zap.Logger
	dependencies *Dependencies
	shutdown     *ShutdownManager
}

// Dependencies содержит все зависимости приложения
type Dependencies struct {
	userController    *controller.UserController
	geoController     *controller.GeoController
	pprofController   *pprof.PprofController
	authMiddleware    func(next http.Handler) http.Handler
	worker            *worker.Worker
}

// NewApp создает новое приложение
func NewApp() (*App, error) {
	app := &App{}
	return app, nil
}

// Initialize инициализирует приложение
func (a *App) Initialize() error {
	a.logger = logger.Get()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	a.cfg = cfg

	// Initialize database
	dbConn, err := db.NewPostgresDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	a.db = dbConn

	// Run migrations
	if err := db.RunMigrations(dbConn, cfg.Database.MigrationsPath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize dependencies
	a.dependencies = a.initializeDependencies()

	// Initialize router
	r := a.setupRouter()
	r.Handle("/metrics", promhttp.Handler())

	a.server = &http.Server{
		Addr:         a.cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  a.cfg.Server.ReadTimeout,
		WriteTimeout: a.cfg.Server.WriteTimeout,
	}

	// Initialize shutdown manager
	a.shutdown = NewShutdownManager(a.cfg.Server.ShutdownTimeout, a.dependencies.worker, a.logger)

	return nil
}

// Run запускает приложение
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Server starting", zap.String("addr", a.cfg.Server.Addr()))

	// Create listener
	listener, err := net.Listen("tcp", a.cfg.Server.Addr())
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	defer listener.Close()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("could not start server: %w", err)
		}
		close(errCh)
	}()

	// Wait for shutdown signal
	<-a.shutdown.WaitForShutdown(ctx)

	a.logger.Info("Server is shutting down...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("Server shutdown failed", zap.Error(err))
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	a.logger.Info("Server stopped gracefully")

	// Check for server errors
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}

// Cleanup очищает ресурсы приложения
func (a *App) Cleanup() error {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}

	// Sync logger
	if err := logger.Get().Sync(); err != nil {
		return fmt.Errorf("failed to sync logger: %w", err)
	}

	return nil
}

// initializeDependencies инициализирует все зависимости приложения
func (a *App) initializeDependencies() *Dependencies {
	sqlAdapter := adapter.NewSQLAdapter(a.db)
	userRepo := repository.NewUserRepository(sqlAdapter, a.db)
	userService := service.NewUserService(userRepo)
	jsonResponder := responder.NewJSONResponder()
	userController := controller.NewUserController(userService, jsonResponder)

	pprofController := pprof.NewPprofController(jsonResponder)

	// Initialize geo service with caching
	realGeoService := service.NewGeoService(a.cfg.Dadata.APIKey, a.cfg.Dadata.SecretKey)
	memoryCache := cache.NewInMemoryCache()
	geoService := geo_proxy.NewGeoServiceProxy(realGeoService, memoryCache, 5*time.Minute)
	geoController := controller.NewGeoController(geoService, jsonResponder)

	// Initialize auth middleware
	authMiddleware := newAuthMiddleware(a.cfg.Auth.JWTSecret)

	// Initialize and start worker if enabled
	var workerInstance *worker.Worker
	if a.cfg.Worker.Enabled {
		workerInstance = worker.NewWorker(a.cfg.Worker.FilePath, a.cfg.Worker.Interval)
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

// setupRouter настраивает маршрутизатор
func (a *App) setupRouter() *chi.Mux {
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
		r.Use(a.dependencies.authMiddleware)
		r.Get("/api/users", a.dependencies.userController.ListUsers)
		r.Post("/api/users", a.dependencies.userController.RegisterUser)
		r.Get("/api/users/{id}", a.dependencies.userController.GetUser)
		r.Put("/api/users/{id}", a.dependencies.userController.UpdateUser)
		r.Delete("/api/users/{id}", a.dependencies.userController.DeleteUser)
		r.Get("/api/users/email", a.dependencies.userController.GetUserByEmail)
	})

	// Geo routes
	r.Group(func(r chi.Router) {
		r.Use(a.dependencies.authMiddleware)
		r.Post("/api/address/search", a.dependencies.geoController.Search)
		r.Post("/api/address/geocode", a.dependencies.geoController.Geocode)
	})

	// Protected pprof routes
	r.Group(func(r chi.Router) {
		r.Use(a.dependencies.authMiddleware)
		r.Mount("/mycustompath/pprof", pprof.Handler())
		r.Post("/api/pprof/cpu/start", a.dependencies.pprofController.StartCPUProfile)
		r.Post("/api/pprof/heap", a.dependencies.pprofController.TakeHeapProfile)
		r.Post("/api/pprof/trace/start", a.dependencies.pprofController.StartTraceProfile)
		r.Get("/api/pprof/profiles", a.dependencies.pprofController.ListProfiles)
	})

	return r
}
