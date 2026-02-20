package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/controller"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/repository"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/cache"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/db/adapter"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/geo_proxy"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/pprof"
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
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: couldn't load .env file: %v", err)
	}

	// Check JWT_SECRET environment variable
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatalf("JWT_SECRET environment variable is required but not set")
	}

	// Initialize database
	dbConn, err := db.NewPostgresDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	// Run migrations
	if err := db.RunMigrations(dbConn); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize dependencies
	sqlAdapter := adapter.NewSQLAdapter(dbConn)
	userRepo := repository.NewUserRepository(sqlAdapter, dbConn)
	userService := service.NewUserService(userRepo)
	jsonResponder := responder.NewJSONResponder()
	userController := controller.NewUserController(userService, jsonResponder)

	// Initialize pprof controller
	pprofController := pprof.NewPprofController(jsonResponder)

	// Initialize router
	r := setupRouter(userController, pprofController)
	// Добавляем обработчик для метрик
	r.Handle("/metrics", promhttp.Handler())
	// Start server
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}

	server := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start server: %s\n", err)
		}
	}()

	go WorkerTest()

	<-done
	log.Println("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown failed: %v\n", err)
	}

	log.Println("Server stopped gracefully")
}

func setupRouter(userController *controller.UserController, pprofController *pprof.PprofController) *chi.Mux {
	r := chi.NewRouter()

	// Добавляем middleware для метрик HTTP
	r.Use(metrics.HTTPMetricsMiddleware)

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	// Initialize geo service
	realGeoService := service.NewGeoService(
		os.Getenv("DADATA_API_KEY"),
		os.Getenv("DADATA_SECRET_KEY"),
	)

	// Create cache
	memoryCache := cache.NewInMemoryCache()

	// Wrap with caching proxy
	geoService := geo_proxy.NewGeoServiceProxy(realGeoService, memoryCache, 5*time.Minute)

	jsonResponder := responder.NewJSONResponder()
	geoController := controller.NewGeoController(geoService, jsonResponder)

	// Auth routes
	r.Group(func(r chi.Router) {
		r.Post("/api/register", RegisterHandler)
		r.Post("/api/login", LoginHandler)
	})

	// User routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Get("/api/users", userController.ListUsers)
		r.Post("/api/users", userController.RegisterUser)
		r.Get("/api/users/{id}", userController.GetUser)
		r.Put("/api/users/{id}", userController.UpdateUser)
		r.Delete("/api/users/{id}", userController.DeleteUser)
		r.Get("/api/users/email", userController.GetUserByEmail)
	})

	// Geo routes
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Post("/api/address/search", geoController.Search)
		r.Post("/api/address/geocode", geoController.Geocode)
	})

	// Protected pprof routes - не документируем в Swagger
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)

		// Web interface pprof routes
		r.Mount("/mycustompath/pprof", pprof.Handler())

		// API endpoints for pprof control
		r.Post("/api/pprof/cpu/start", pprofController.StartCPUProfile)
		r.Post("/api/pprof/heap", pprofController.TakeHeapProfile)
		r.Post("/api/pprof/trace/start", pprofController.StartTraceProfile)
		r.Get("/api/pprof/profiles", pprofController.ListProfiles)
	})

	return r
}

const content = `# Test Page

This is a test page. Byte: %d`

func WorkerTest() {
	t := time.NewTicker(1 * time.Second)
	var b byte = 0
	for {
		select {
		case <-t.C:
			err := os.WriteFile("/app/static/_index.md", []byte(fmt.Sprintf(content, b)), 0644)
			if err != nil {
				log.Println(err)
			}
			b++
		}
	}
}
