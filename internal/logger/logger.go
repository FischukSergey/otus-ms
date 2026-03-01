// Package logger предоставляет инициализацию структурированного логгера slog.
package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"

	"github.com/FischukSergey/otus-ms/internal/config"
)

// NewLogger создает новый логгер на основе конфигурации.
// Для формата "text" использует tint с цветным выводом (для локальной разработки),
// для формата "json" использует стандартный JSON handler (для продакшена).
func NewLogger(cfg config.LogConfig) *slog.Logger {
	var handler slog.Handler

	level := parseLevel(cfg.Level)

	switch cfg.Format {
	case "json":
		// JSON формат для продакшена - структурированные логи для Loki/ELK
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true, // добавляет файл и строку в логи
		})

	case "text":
		// Text формат с цветным выводом для локальной разработки
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.TimeOnly, // короткий формат времени 15:04:05
			AddSource:  true,          // показывает файл и строку
			NoColor:    false,         // включаем цвета
		})

	default:
		// По умолчанию используем JSON
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	}

	// Создаем логгер с базовым атрибутом service_name
	// Это позволит идентифицировать логи от разных микросервисов
	logger := slog.New(handler)
	logger = logger.With("service", cfg.ServiceName)

	return logger
}

// parseLevel преобразует строковое значение уровня логирования в slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
