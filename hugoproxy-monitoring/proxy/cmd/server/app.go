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
	"gitlab.com/s.izotov81/hugoproxy/internal/interface/http/handler"
	authHandler "gitlab.com/s.izotov81/hugoproxy/internal/interface/http/handler/auth"
	authMiddleware "gitlab.com/s.izotov81/hugoproxy/internal/interface/http/middleware/auth"
	"gitlab.com/s.izotov81/hugoproxy/internal/interface/persistence"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/cache"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db/adapter"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/geo_proxy"
	customMiddleware "gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/middleware"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/pprof"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/worker"
	"gitlab.com/s.izotov81/hugoproxy/internal/usecase/geo"
	"gitlab.com/s.izotov81/hugoproxy/internal/usecase/user"
	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"

	_ "gitlab.com/s.izotov81/hugoproxy/docs"
)

type authHandlerKey struct{}

type App struct {
	cfg          *config.Config
	db           *sqlx.DB
	server       *http.Server
	logger       *zap.Logger
	dependencies *Dependencies
	shutdown     *ShutdownManager
}

type Dependencies struct {
	userController    *handler.UserController
	geoController     *handler.GeoController
	pprofController   *pprof.PprofController
	authMiddleware    func(next http.Handler) http.Handler
	worker            *worker.Worker
}

func NewApp() (*App, error) {
	app := &App{}
	return app, nil
}

func (a *App) Initialize() error {
	a.logger = logger.Get()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	a.cfg = cfg

	dbConn, err := db.NewPostgresDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	a.db = dbConn

	if err := db.RunMigrations(dbConn, cfg.Database.MigrationsPath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	a.dependencies = a.initializeDependencies()

	r := a.setupRouter()
	r.Handle("/metrics", promhttp.Handler())

	a.server = &http.Server{
		Addr:         a.cfg.Server.Addr(),
		Handler:      r,
		ReadTimeout:  a.cfg.Server.ReadTimeout,
		WriteTimeout: a.cfg.Server.WriteTimeout,
	}

	a.shutdown = NewShutdownManager(a.cfg.Server.ShutdownTimeout, a.dependencies.worker, a.logger)

	return nil
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Server starting", zap.String("addr", a.cfg.Server.Addr()))

	listener, err := net.Listen("tcp", a.cfg.Server.Addr())
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		if err := a.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("could not start server: %w", err)
		}
		close(errCh)
	}()

	<-a.shutdown.WaitForShutdown(ctx)

	a.logger.Info("Server is shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.logger.Error("Server shutdown failed", zap.Error(err))
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	a.logger.Info("Server stopped gracefully")

	if err := <-errCh; err != nil {
		return err
	}

	return nil
}

func (a *App) Cleanup() error {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}

	if err := logger.Get().Sync(); err != nil {
		return fmt.Errorf("failed to sync logger: %w", err)
	}

	return nil
}

func (a *App) initializeDependencies() *Dependencies {
	sqlAdapter := adapter.NewSQLAdapter(a.db)
	userRepo := persistence.NewUserRepository(sqlAdapter, a.db)
	userService := user.NewUserService(userRepo)
	jsonResponder := responder.NewJSONResponder()
	userController := handler.NewUserController(userService, jsonResponder)

	pprofController := pprof.NewPprofController(jsonResponder)

	realGeoService := geo.NewGeoService(a.cfg.Dadata.APIKey, a.cfg.Dadata.SecretKey)
	memoryCache := cache.NewInMemoryCache()
	geoService := geo_proxy.NewGeoServiceProxy(realGeoService, memoryCache, 5*time.Minute)
	geoController := handler.NewGeoController(geoService, jsonResponder)

	authMiddlewareFunc := authMiddleware.NewMiddleware(a.cfg.Auth.JWTSecret)

	var workerInstance *worker.Worker
	if a.cfg.Worker.Enabled {
		workerInstance = worker.NewWorker(a.cfg.Worker.FilePath, a.cfg.Worker.Interval)
		workerInstance.Start()
	}

	return &Dependencies{
		userController:    userController,
		geoController:     geoController,
		pprofController:   pprofController,
		authMiddleware:    authMiddlewareFunc,
		worker:            workerInstance,
	}
}

func (a *App) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(customMiddleware.RequestID)
	r.Use(customMiddleware.SecurityHeaders)
	r.Use(customMiddleware.CORS(customMiddleware.DefaultCORSConfig()))
	r.Use(customMiddleware.RateLimit(customMiddleware.DefaultRateLimitConfig()))
	r.Use(metrics.HTTPMetricsMiddleware)
	r.Use(middleware.Recoverer)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	r.Get("/swagger-ui/*", http.StripPrefix("/swagger-ui/", http.FileServer(http.Dir("./static"))).ServeHTTP)

	authHandlerInstance := authHandler.NewHandler(
		persistence.NewUserRepository(adapter.NewSQLAdapter(a.db), a.db),
		a.cfg.Auth.JWTSecret,
	)

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), authHandlerKey{}, authHandlerInstance)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Post("/api/register", authHandler.RegisterHandler)
		r.Post("/api/login", authHandler.LoginHandler)
	})

	r.Group(func(r chi.Router) {
		r.Use(a.dependencies.authMiddleware)
		r.Get("/api/users", a.dependencies.userController.ListUsers)
		r.Post("/api/users", a.dependencies.userController.RegisterUser)
		r.Get("/api/users/{id}", a.dependencies.userController.GetUser)
		r.Put("/api/users/{id}", a.dependencies.userController.UpdateUser)
		r.Delete("/api/users/{id}", a.dependencies.userController.DeleteUser)
		r.Get("/api/users/email", a.dependencies.userController.GetUserByEmail)
	})

	r.Group(func(r chi.Router) {
		r.Use(a.dependencies.authMiddleware)
		r.Post("/api/address/search", a.dependencies.geoController.Search)
		r.Post("/api/address/geocode", a.dependencies.geoController.Geocode)
	})

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
