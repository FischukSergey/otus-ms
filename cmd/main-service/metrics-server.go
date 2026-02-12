package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer представляет HTTP сервер для метрик Prometheus.
type MetricsServer struct {
	server *http.Server
	logger *slog.Logger
}

// MetricsServerDeps содержит зависимости для инициализации Metrics сервера.
type MetricsServerDeps struct {
	Addr   string
	Logger *slog.Logger
}

// NewMetricsServer создает и настраивает metrics сервер с endpoint /metrics.
func NewMetricsServer(deps *MetricsServerDeps) *MetricsServer {
	router := chi.NewRouter()

	// Минимальный middleware - только для самого metrics сервера
	router.Use(middleware.Recoverer)

	metricsSrv := &MetricsServer{
		logger: deps.Logger,
	}

	// Главная страница с информацией
	router.Get("/", metricsSrv.handleMetricsIndex)

	// Health check для самого metrics сервера
	router.Get("/health", metricsSrv.handleMetricsHealth)

	// Endpoint для Prometheus - предоставляет собранные метрики
	router.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              deps.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	metricsSrv.server = server
	return metricsSrv
}

// Start запускает HTTP сервер метрик.
func (s *MetricsServer) Start() error {
	s.logger.Info("Metrics server starting", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop останавливает HTTP сервер метрик с graceful shutdown.
func (s *MetricsServer) Stop(ctx context.Context) error {
	s.logger.Info("Metrics server stopping...")
	return s.server.Shutdown(ctx)
}

// handleMetricsIndex обрабатывает запросы к корневому пути metrics сервера.
func (s *MetricsServer) handleMetricsIndex(w http.ResponseWriter, _ *http.Request) {
	response := map[string]any{
		"service": "OtusMS Metrics Server",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"/metrics": "Prometheus metrics endpoint",
			"/health":  "Health check endpoint",
		},
		"info": "This server provides Prometheus metrics for monitoring",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode metrics index response", "error", err)
	}
}

// handleMetricsHealth обрабатывает healthcheck запросы для metrics сервера.
func (s *MetricsServer) handleMetricsHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode metrics health response", "error", err)
	}
}
