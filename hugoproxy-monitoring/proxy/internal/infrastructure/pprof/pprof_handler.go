package pprof

import (
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler возвращает защищенный роутер для pprof
func Handler() http.Handler {
	r := chi.NewRouter()

	// Редирект с корня на индекс
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.RequestURI+"/", http.StatusMovedPermanently)
	})

	// Обработка всех pprof эндпоинтов
	r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		handlePprofRequest(w, r)
	})

	return r
}

// handlePprofRequest обрабатывает запросы к pprof
func handlePprofRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/mycustompath/pprof/")

	switch path {
	case "cmdline":
		pprof.Cmdline(w, r)
	case "profile":
		pprof.Profile(w, r)
	case "symbol":
		pprof.Symbol(w, r)
	case "trace":
		pprof.Trace(w, r)
	case "":
		pprof.Index(w, r)
	default:
		// Для остальных профилей (allocs, block, goroutine, heap, mutex, threadcreate)
		handler := pprof.Handler(path)
		handler.ServeHTTP(w, r)
	}
}
