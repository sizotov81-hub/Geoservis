package main

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/joho/godotenv"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		logger.Get().Warn("couldn't load .env file", zap.Error(err))
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logPath := os.Getenv("LOG_FILE_PATH")
	if err := logger.Init(logLevel, logPath); err != nil {
		logger.Get().Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer func() {
		if syncErr := logger.Get().Sync(); syncErr != nil {
			logger.Get().Warn("Failed to sync logger", zap.Error(syncErr))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := NewApp()
	if err != nil {
		logger.Get().Fatal("Failed to create application", zap.Error(err))
	}

	if err := app.Initialize(); err != nil {
		logger.Get().Fatal("Failed to initialize application", zap.Error(err))
	}

	defer func() {
		if err := app.Cleanup(); err != nil {
			logger.Get().Error("Failed to cleanup application", zap.Error(err))
		}
	}()

	if err := app.Run(ctx); err != nil {
		logger.Get().Error("Application error", zap.Error(err))
		os.Exit(1)
	}
}
