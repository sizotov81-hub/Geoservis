package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// Worker представляет фоновый воркер для записи файлов
type Worker struct {
	filePath string
	interval time.Duration
	mu       sync.RWMutex
	content  byte
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
}

// NewWorker создает новый экземпляр воркера
func NewWorker(filePath string, interval time.Duration) *Worker {
	return &Worker{
		filePath: filePath,
		interval: interval,
		content:  0,
		stopCh:   make(chan struct{}),
	}
}

// Start запускает воркер
func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.run()
	log := logger.Get()
	log.Info("Worker started", zap.String("filePath", w.filePath), zap.Duration("interval", w.interval))
}

// Stop останавливает воркер
func (w *Worker) Stop(ctx context.Context) {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	log := logger.Get()
	select {
	case <-done:
		log.Info("Worker stopped gracefully")
	case <-ctx.Done():
		log.Warn("Worker stopped by context timeout")
	}
}

// IsRunning возвращает статус воркера
func (w *Worker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *Worker) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	const contentTemplate = `# Test Page

This is a test page. Byte: %d`

	log := logger.Get()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.mu.Lock()
			content := fmt.Sprintf(contentTemplate, w.content)
			w.content++
			w.mu.Unlock()

			if err := os.WriteFile(w.filePath, []byte(content), 0644); err != nil {
				log.Error("Worker failed to write file", zap.String("filePath", w.filePath), zap.Error(err))
			}
		}
	}
}
