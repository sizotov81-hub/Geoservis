package pprof

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"time"
)

// CPUProfileOptions настройки для CPU профилирования
type CPUProfileOptions struct {
	Duration int    `json:"duration"` // Длительность в миллисекундах
	FilePath string `json:"filePath"`
}

// StartCPUProfile запускает CPU профилирование в файл
func StartCPUProfile(ctx context.Context, opts CPUProfileOptions) error {
	if opts.FilePath == "" {
		// Сохраняем в директорию pprof в контейнере
		opts.FilePath = fmt.Sprintf("/app/pprof/cpu_profile_%s.pprof", time.Now().Format("20060102_150405"))
	}

	// Преобразуем миллисекунды в time.Duration
	duration := time.Duration(opts.Duration) * time.Millisecond
	if duration == 0 {
		duration = 30 * time.Second
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Создаем полный путь к файлу
	fullPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create profile file: %w", err)
	}

	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return fmt.Errorf("start CPU profile: %w", err)
	}

	go func() {
		select {
		case <-ctx.Done():
			pprof.StopCPUProfile()
			file.Close()
			log.Printf("CPU profile stopped and saved to: %s", fullPath)
		case <-time.After(duration):
			pprof.StopCPUProfile()
			file.Close()
			log.Printf("CPU profile completed and saved to: %s", fullPath)
		}
	}()

	log.Printf("CPU profiling started for %v, will save to: %s", duration, fullPath)
	return nil
}

// HeapProfileOptions настройки для Heap профилирования
type HeapProfileOptions struct {
	FilePath string `json:"filePath"`
}

// TakeHeapProfile создает снимок heap профиля
func TakeHeapProfile(opts HeapProfileOptions) error {
	if opts.FilePath == "" {
		// Сохраняем в директорию pprof в контейнере
		opts.FilePath = fmt.Sprintf("/app/pprof/heap_profile_%s.pprof", time.Now().Format("20060102_150405"))
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Создаем полный путь к файлу
	fullPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create heap profile file: %w", err)
	}
	defer file.Close()

	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("write heap profile: %w", err)
	}

	log.Printf("Heap profile saved to: %s", fullPath)
	return nil
}

// TraceProfileOptions настройки для Trace профилирования
type TraceProfileOptions struct {
	Duration int    `json:"duration"` // Длительность в миллисекундах
	FilePath string `json:"filePath"`
}

// StartTraceProfile запускает сбор trace данных
func StartTraceProfile(ctx context.Context, opts TraceProfileOptions) error {
	if opts.FilePath == "" {
		// Сохраняем в директорию pprof в контейнере
		opts.FilePath = fmt.Sprintf("/app/pprof/trace_%s.out", time.Now().Format("20060102_150405"))
	}

	// Преобразуем миллисекунды в time.Duration
	duration := time.Duration(opts.Duration) * time.Millisecond
	if duration == 0 {
		duration = 5 * time.Second
	}

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Создаем полный путь к файлу
	fullPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create trace file: %w", err)
	}

	if err := trace.Start(file); err != nil {
		file.Close()
		return fmt.Errorf("start trace: %w", err)
	}

	go func() {
		select {
		case <-ctx.Done():
			trace.Stop()
			file.Close()
			log.Printf("Trace stopped and saved to: %s", fullPath)
		case <-time.After(duration):
			trace.Stop()
			file.Close()
			log.Printf("Trace completed and saved to: %s", fullPath)
		}
	}()

	log.Printf("Trace profiling started for %v, will save to: %s", duration, fullPath)
	return nil
}

// ProfileInfo представляет информацию о доступных профилях
type ProfileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetAvailableProfiles возвращает список доступных профилей
func GetAvailableProfiles() []ProfileInfo {
	return []ProfileInfo{
		{Name: "allocs", Path: "/mycustompath/pprof/allocs"},
		{Name: "block", Path: "/mycustompath/pprof/block"},
		{Name: "cmdline", Path: "/mycustompath/pprof/cmdline"},
		{Name: "goroutine", Path: "/mycustompath/pprof/goroutine"},
		{Name: "heap", Path: "/mycustompath/pprof/heap"},
		{Name: "mutex", Path: "/mycustompath/pprof/mutex"},
		{Name: "profile", Path: "/mycustompath/pprof/profile"},
		{Name: "threadcreate", Path: "/mycustompath/pprof/threadcreate"},
		{Name: "trace", Path: "/mycustompath/pprof/trace"},
	}
}
