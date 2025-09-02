package pprof

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
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
			log.Printf("PPROF %s %s %d %v", r.Method, r.URL.Path, ww.Status(), duration)
		}
	})
}
