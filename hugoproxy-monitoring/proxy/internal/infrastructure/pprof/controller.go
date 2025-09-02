package pprof

import (
	"net/http"
	"time"

	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"
)

// PprofController контроллер для управления профилирования
type PprofController struct {
	responder responder.Responder
}

// NewPprofController создает новый контроллер pprof
func NewPprofController(responder responder.Responder) *PprofController {
	return &PprofController{
		responder: responder,
	}
}

// StartCPUProfile запускает CPU профилирование
func (c *PprofController) StartCPUProfile(w http.ResponseWriter, r *http.Request) {
	var opts CPUProfileOptions
	if err := c.responder.Decode(r, &opts); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if err := StartCPUProfile(r.Context(), opts); err != nil {
		c.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Преобразуем миллисекунды в time.Duration для получения строкового представления
	duration := time.Duration(opts.Duration) * time.Millisecond
	if opts.Duration == 0 {
		duration = 30 * time.Second
	}

	c.responder.Respond(w, http.StatusOK, map[string]string{
		"status":   "started",
		"file":     opts.FilePath,
		"duration": duration.String(),
	})
}

// TakeHeapProfile создает снимок heap профиля
func (c *PprofController) TakeHeapProfile(w http.ResponseWriter, r *http.Request) {
	var opts HeapProfileOptions
	if err := c.responder.Decode(r, &opts); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if err := TakeHeapProfile(opts); err != nil {
		c.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	c.responder.Respond(w, http.StatusOK, map[string]string{
		"status": "completed",
		"file":   opts.FilePath,
	})
}

// StartTraceProfile запускает сбор trace данных
func (c *PprofController) StartTraceProfile(w http.ResponseWriter, r *http.Request) {
	var opts TraceProfileOptions
	if err := c.responder.Decode(r, &opts); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if err := StartTraceProfile(r.Context(), opts); err != nil {
		c.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Преобразуем миллисекунды в time.Duration для получения строкового представления
	duration := time.Duration(opts.Duration) * time.Millisecond
	if opts.Duration == 0 {
		duration = 5 * time.Second
	}

	c.responder.Respond(w, http.StatusOK, map[string]string{
		"status":   "started",
		"file":     opts.FilePath,
		"duration": duration.String(),
	})
}

// ListProfiles возвращает список доступных pprof профилей
func (c *PprofController) ListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := GetAvailableProfiles()
	c.responder.Respond(w, http.StatusOK, profiles)
}
