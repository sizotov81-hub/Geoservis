package logger

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Init инициализирует глобальный логгер
// logPath - путь к файлу логов (если пустой, логи пишутся в stdout)
func Init(level string, logPath string) error {
	cfg := zap.NewProductionConfig()

	cfg.Level = zap.NewAtomicLevelAt(parseLevel(level))
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder

	var err error
	if logPath != "" {
		// Создаём директорию для логов, если не существует
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// Пишем логи в файл и stdout
		fileWriter, fileErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fileErr != nil {
			return fileErr
		}

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg.EncoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(fileWriter)),
			cfg.Level,
		)
		log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		log, err = cfg.Build()
	}

	return err
}

// parseLevel преобразует строку в уровень логирования
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Get возвращает глобальный логгер
func Get() *zap.Logger {
	if log == nil {
		log, _ = zap.NewProduction()
	}
	return log
}

// FromContext возвращает логгер из контекста или глобальный логгер
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*zap.Logger); ok {
		return logger
	}
	return Get()
}

// WithContext добавляет логгер в контекст
func WithContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

type loggerContextKey struct{}
