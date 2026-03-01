package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// APIServer представляет HTTP сервер news-collector.
type APIServer struct {
	server *http.Server
	logger *slog.Logger
}

// APIServerDeps содержит зависимости для инициализации API сервера.
type APIServerDeps struct {
	Addr   string
	Logger *slog.Logger
}

// NewAPIServer создаёт и настраивает HTTP сервер.
func NewAPIServer(deps *APIServerDeps) *APIServer {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	apiSrv := &APIServer{logger: deps.Logger}

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

// handleHealth обрабатывает healthcheck запросы.
func (s *APIServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"status":  "ok",
		"service": "news-collector",
		"time":    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode health response", "error", err)
	}
}
