package metrics

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// HTTPMetricsMiddleware middleware для сбора HTTP метрик
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] HTTPMetricsMiddleware recovered: %v", r)
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

		ObserveHTTPRequest(path, method, statusCode, duration)
	})
}
