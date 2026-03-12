package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/FischukSergey/otus-ms/api/authproxy" // swagger docs
	"github.com/FischukSergey/otus-ms/internal/clients/mainservice"
	"github.com/FischukSergey/otus-ms/internal/config"
	authhandler "github.com/FischukSergey/otus-ms/internal/handlers/auth"
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	custommiddleware "github.com/FischukSergey/otus-ms/internal/middleware"
	"github.com/FischukSergey/otus-ms/internal/ratelimiter"
)

// APIServer представляет HTTP API сервер для Auth-Proxy.
type APIServer struct {
	server         *http.Server
	logger         *slog.Logger
	keycloakClient *keycloak.Client
}

// APIServerDeps содержит зависимости для инициализации API сервера.
type APIServerDeps struct {
	Addr              string
	Config            *config.Config
	Logger            *slog.Logger
	KeycloakClient    *keycloak.Client
	MainServiceClient *mainservice.Client
	RateLimiter       *ratelimiter.Limiter // nil если Redis не настроен
}

// NewAPIServer создает и настраивает API сервер с chi роутером.
func NewAPIServer(deps *APIServerDeps) *APIServer {
	router := chi.NewRouter()

	// Middleware - порядок важен!
	router.Use(middleware.RequestID)                           // 1. Генерируем request_id
	router.Use(middleware.RealIP)                              // 2. Определяем реальный IP
	router.Use(custommiddleware.LoggerMiddleware(deps.Logger)) // 3. Логируем запросы с request_id
	router.Use(middleware.Recoverer)                           // 4. Восстанавливаемся от паник

	// API сервер
	apiSrv := &APIServer{
		logger:         deps.Logger,
		keycloakClient: deps.KeycloakClient,
	}

	// Инициализация Auth Handler
	authHandler := authhandler.NewHandler(
		deps.KeycloakClient,
		deps.MainServiceClient,
		deps.Logger,
	)

	// Роуты
	router.Get("/", apiSrv.handleRoot)
	router.Get("/health", apiSrv.handleHealth)

	// Swagger UI
	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API роуты для авторизации
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register) // Публичный endpoint - регистрация
		// /login защищён Rate Limiter (если Redis настроен)
		if deps.RateLimiter != nil {
			r.With(custommiddleware.RateLimitMiddleware(deps.RateLimiter, deps.Logger)).Post("/login", authHandler.Login)
		} else {
			r.Post("/login", authHandler.Login)
		}
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
	})

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
		"message": "Auth-Proxy Service",
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
