package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/FischukSergey/otus-ms/internal/config"
)

// APIServer представляет простой HTTP API сервер.
type APIServer struct {
	server *http.Server
	logger *slog.Logger
}

// APIServerDeps содержит зависимости для инициализации API сервера.
type APIServerDeps struct {
	Addr   string
	Config config.Config
	Logger *slog.Logger
}

// NewAPIServer создает и настраивает простой API сервер с chi роутером.
func NewAPIServer(deps APIServerDeps) *APIServer {
	router := chi.NewRouter()

	// Middleware
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)

	// API сервер
	apiSrv := &APIServer{
		logger: deps.Logger,
	}

	// Роуты
	router.Get("/", apiSrv.handleRoot)
	router.Get("/health", apiSrv.handleHealth)

	server := &http.Server{
		Addr:              deps.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	apiSrv.server = server
	return apiSrv
}

// Start запускает HTTP сервер.
func (s *APIServer) Start() error {
	s.logger.Info("API server starting", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop останавливает HTTP сервер с graceful shutdown.
func (s *APIServer) Stop(ctx context.Context) error {
	s.logger.Info("API server stopping...")
	return s.server.Shutdown(ctx)
}

// handleRoot обрабатывает запросы к корневому пути.
func (s *APIServer) handleRoot(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"message": "Welcome to OtusMS Microservice!",
		"version": "1.0.0",
		"status":  "running",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", "error", err)
	}
}

// handleHealth обрабатывает healthcheck запросы.
func (s *APIServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode health response", "error", err)
	}
}
