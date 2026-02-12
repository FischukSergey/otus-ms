package middleware

import (
	"context"
	"log/slog"
)

// Ключ для хранения логгера в контексте.
type contextKey string

const loggerKey contextKey = "logger"

// SetLogger добавляет логгер в контекст запроса.
func SetLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext извлекает логгер из контекста запроса.
// Если логгер не найден, возвращает дефолтный логгер.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	// Возвращаем дефолтный логгер, если не найден в контексте
	return slog.Default()
}
