// Package metrics содержит Prometheus-метрики приложения.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal - счетчик общего количества HTTP запросов.
	// Labels: method (GET, POST, DELETE), path (/api/v1/users, /health), status (200, 201, 404, 500).
	HTTPRequestsTotal *prometheus.CounterVec

	// HTTPRequestDuration - гистограмма времени выполнения HTTP запросов в секундах.
	// Labels: method, path.
	HTTPRequestDuration *prometheus.HistogramVec

	// HTTPRequestSizeBytes - гистограмма размера тела HTTP запроса в байтах.
	// Labels: method, path.
	HTTPRequestSizeBytes *prometheus.HistogramVec

	// HTTPResponseSizeBytes - гистограмма размера тела HTTP ответа в байтах.
	// Labels: method, path.
	HTTPResponseSizeBytes *prometheus.HistogramVec
)

// Init инициализирует все Prometheus метрики.
// Использует promauto для автоматической регистрации метрик в DefaultRegisterer.
// Должна быть вызвана один раз при старте приложения.
func Init() {
	// Counter для общего количества запросов
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// Histogram для времени выполнения запросов
	// Buckets: 1ms, 5ms, 10ms, 50ms, 100ms, 500ms, 1s, 5s, 10s
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10},
		},
		[]string{"method", "path"},
	)

	// Histogram для размера запроса
	// Buckets: 100B, 1KB, 10KB, 100KB, 1MB, 10MB
	HTTPRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)

	// Histogram для размера ответа
	// Buckets: 100B, 1KB, 10KB, 100KB, 1MB, 10MB
	HTTPResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"method", "path"},
	)
}
