package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// HTTPMetricsMiddleware middleware для сбора HTTP метрик
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log := logger.FromContext(r.Context())
				log.Error("[PANIC] HTTPMetricsMiddleware recovered", zap.Any("panic", rec))
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		statusCode := strconv.Itoa(ww.Status())
		path := r.URL.Path
		method := r.Method

		log := logger.FromContext(r.Context())
		log.Debug("HTTP request",
			zap.String("path", path),
			zap.String("method", method),
			zap.String("status", statusCode),
			zap.Duration("duration", duration))

		ObserveHTTPRequest(path, method, statusCode, duration)
	})
}
