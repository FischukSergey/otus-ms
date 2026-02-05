package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// LoggerMiddleware создает middleware для логирования HTTP запросов.
// Использует request_id из chi middleware для корреляции запросов.
// Добавляет логгер с контекстными полями в context запроса.
func LoggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Получаем request_id из chi middleware (должен быть установлен до этого)
			requestID := middleware.GetReqID(r.Context())

			// Создаем логгер с контекстными полями для этого запроса
			reqLogger := logger.With(
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			// Логируем начало обработки запроса
			reqLogger.Info("request started")

			// Оборачиваем ResponseWriter для захвата статус-кода и размера ответа
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Добавляем логгер в контекст запроса для использования в handlers
			ctx := SetLogger(r.Context(), reqLogger)

			// Вызываем следующий handler
			next.ServeHTTP(ww, r.WithContext(ctx))

			// Логируем завершение запроса с метриками
			duration := time.Since(start)
			reqLogger.Info("request completed",
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}
