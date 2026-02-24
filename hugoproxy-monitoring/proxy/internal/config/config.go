package config

import (
	"fmt"
	"os"
	"time"
)

// Config хранит конфигурацию приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Dadata   DadataConfig
	Worker   WorkerConfig
}

// ServerConfig конфигурация HTTP сервера
type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig конфигурация базы данных
type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
	MigrationsPath string
}

// AuthConfig конфигурация аутентификации
type AuthConfig struct {
	JWTSecret string
}

// DadataConfig конфигурация внешнего API
type DadataConfig struct {
	APIKey    string
	SecretKey string
}

// WorkerConfig конфигурация воркера
type WorkerConfig struct {
	Enabled      bool
	FilePath     string
	Interval     time.Duration
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	return &Config{
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 5*time.Second),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        os.Getenv("DB_PASSWORD"),
			DBName:          getEnv("DB_NAME", "geoservice"),
			MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			MigrationsPath:  getEnv("MIGRATIONS_PATH", "migrations"),
		},
		Auth: AuthConfig{
			JWTSecret: jwtSecret,
		},
		Dadata: DadataConfig{
			APIKey:    os.Getenv("DADATA_API_KEY"),
			SecretKey: os.Getenv("DADATA_SECRET_KEY"),
		},
		Worker: WorkerConfig{
			Enabled:  getBoolEnv("WORKER_ENABLED", false),
			FilePath: getEnv("WORKER_FILE_PATH", "/app/static/_index.md"),
			Interval: getDurationEnv("WORKER_INTERVAL", 1*time.Second),
		},
	}, nil
}

// DSN возвращает строку подключения к базе данных
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.DBName,
	)
}

// Addr возвращает адрес сервера
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// Вспомогательные функции

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
