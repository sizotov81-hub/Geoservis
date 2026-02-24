package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/worker"
)

type ShutdownManager struct {
	timeout         time.Duration
	worker          *worker.Worker
	logger          *zap.Logger
	shutdownChan    chan os.Signal
	shutdownOnce    sync.Once
	shutdownStarted bool
	mu              sync.RWMutex
}

func NewShutdownManager(
	timeout time.Duration,
	worker *worker.Worker,
	logger *zap.Logger,
) *ShutdownManager {
	return &ShutdownManager{
		timeout:      timeout,
		worker:       worker,
		logger:       logger,
		shutdownChan: make(chan os.Signal, 1),
	}
}

func (s *ShutdownManager) WaitForShutdown(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	signal.Notify(s.shutdownChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer close(done)
		<-s.shutdownChan

		s.mu.Lock()
		s.shutdownStarted = true
		s.mu.Unlock()

		s.logger.Info("Shutdown signal received")

		if s.worker != nil {
			workerCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			s.worker.Stop(workerCtx)
		}
	}()

	return done
}

func (s *ShutdownManager) IsShuttingDown() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shutdownStarted
}

func (s *ShutdownManager) ForceShutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownChan)
	})
}
