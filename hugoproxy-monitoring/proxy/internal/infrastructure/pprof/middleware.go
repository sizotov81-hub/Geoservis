package pprof

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// Middleware для логирования запросов к pprof
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		// Логируем только pprof запросы
		if strings.Contains(r.URL.Path, "/mycustompath/pprof/") {
			duration := time.Since(start)
			log := logger.FromContext(r.Context())
			log.Info("PPROF request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", duration))
		}
	})
}
