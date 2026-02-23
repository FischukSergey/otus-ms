package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// PrometheusMiddleware собирает HTTP-метрики: количество запросов, латентность и размер ответа.
func PrometheusMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Оборачиваем ResponseWriter для захвата статус-кода и размера ответа
			// Используем middleware.WrapResponseWriter из chi для совместимости
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Измеряем размер запроса (если есть Content-Length)
			requestSize := float64(0)
			if r.ContentLength > 0 {
				requestSize = float64(r.ContentLength)
			}

			// Вызываем следующий handler
			next.ServeHTTP(ww, r)

			// Записываем метрики после завершения обработки
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())
			method := r.Method
			path := r.URL.Path

			// Счетчик запросов с labels: method, path, status
			HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()

			// Время выполнения запроса
			HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)

			// Размер запроса
			if requestSize > 0 {
				HTTPRequestSizeBytes.WithLabelValues(method, path).Observe(requestSize)
			}

			// Размер ответа
			responseSize := float64(ww.BytesWritten())
			if responseSize > 0 {
				HTTPResponseSizeBytes.WithLabelValues(method, path).Observe(responseSize)
			}
		})
	}
}
