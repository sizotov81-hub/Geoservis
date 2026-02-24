package db

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/config"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// NewPostgresDB создает новое подключение к базе данных PostgreSQL
func NewPostgresDB(cfg config.DatabaseConfig) (*sqlx.DB, error) {
	var db *sqlx.DB
	var err error

	maxAttempts := 10
	log := logger.Get()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err = sqlx.Connect("postgres", cfg.DSN())
		if err == nil {
			break
		}
		log.Warn("Failed to connect to database",
			zap.Int("attempt", attempt),
			zap.Int("maxAttempts", maxAttempts),
			zap.Error(err))
		if attempt < maxAttempts {
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxAttempts, err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Successfully connected to database")
	return db, nil
}

// RunMigrations выполняет миграции базы данных
func RunMigrations(db *sqlx.DB, migrationsPath string) error {
	log := logger.Get()

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	migrationsDir := migrationsPath
	if !filepath.IsAbs(migrationsDir) {
		var err error
		migrationsDir, err = filepath.Abs(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for migrations: %w", err)
		}
	}

	if err := goose.Up(db.DB, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Info("Migrations applied successfully", zap.String("migrationsPath", migrationsDir))
	return nil
}
