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

	_ "github.com/FischukSergey/otus-ms/api/mainservice" // swagger docs
	"github.com/FischukSergey/otus-ms/internal/config"
	userhandler "github.com/FischukSergey/otus-ms/internal/handlers/user"
	"github.com/FischukSergey/otus-ms/internal/jwks"
	"github.com/FischukSergey/otus-ms/internal/metrics"
	custommiddleware "github.com/FischukSergey/otus-ms/internal/middleware"
	userservice "github.com/FischukSergey/otus-ms/internal/services/user"
	"github.com/FischukSergey/otus-ms/internal/store"
	userrepo "github.com/FischukSergey/otus-ms/internal/store/user"
)

// APIServer представляет простой HTTP API сервер.
type APIServer struct {
	server  *http.Server
	logger  *slog.Logger
	storage *store.Storage
}

// APIServerDeps содержит зависимости для инициализации API сервера.
type APIServerDeps struct {
	Addr        string
	Config      config.Config
	Logger      *slog.Logger
	Storage     *store.Storage
	JWKSManager *jwks.Manager // JWKS Manager для валидации JWT (может быть nil)
}

// NewAPIServer создает и настраивает простой API сервер с chi роутером.
func NewAPIServer(deps *APIServerDeps) *APIServer {
	router := chi.NewRouter()

	// Middleware - порядок важен!
	router.Use(middleware.RequestID)                           // 1. Генерируем request_id
	router.Use(middleware.RealIP)                              // 2. Определяем реальный IP
	router.Use(custommiddleware.LoggerMiddleware(deps.Logger)) // 3. Логируем запросы с request_id
	router.Use(metrics.PrometheusMiddleware())                 // 4. Собираем метрики для Prometheus
	router.Use(middleware.Recoverer)                           // 5. Восстанавливаемся от паник

	// API сервер
	apiSrv := &APIServer{
		logger:  deps.Logger,
		storage: deps.Storage,
	}

	// Инициализация слоев для работы с пользователями
	userRepository := userrepo.NewRepository(deps.Storage.DB())
	userService := userservice.NewService(userRepository)
	userHandler := userhandler.NewHandler(userService, deps.Logger)

	// Роуты
	router.Get("/", apiSrv.handleRoot)
	router.Get("/health", apiSrv.handleHealth)

	// Swagger UI
	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API роуты
	router.Route("/api/v1", func(r chi.Router) {
		// Публичные роуты (без JWT) - если понадобятся
		// r.Post("/register", someHandler.Register)

		// Защищённые роуты - требуется валидный JWT
		r.Group(func(r chi.Router) {
			// JWT валидация для всех роутов в группе
			// ВАЖНО: JWKSManager должен быть инициализирован в main.go
			if deps.JWKSManager != nil {
				r.Use(custommiddleware.ValidateJWT(
					custommiddleware.JWTConfig{
						Issuer:   deps.Config.Keycloak.Issuer(),
						Audience: deps.Config.Keycloak.ClientID,
					},
					deps.JWKSManager,
					deps.Logger,
				))
			}

			// Роуты для работы с пользователями
			r.Route("/users", func(r chi.Router) {
				// Только для администраторов
				r.Group(func(r chi.Router) {
					r.Use(custommiddleware.RequireAdmin(deps.Logger))
					r.Post("/", userHandler.Create)        // Создать пользователя
					r.Get("/{uuid}", userHandler.Get)      // Получить любого пользователя
					r.Delete("/{uuid}", userHandler.Delete) // Удалить пользователя
				})

				// Для пользователей с ролью user или admin (если понадобятся)
				// r.Group(func(r chi.Router) {
				//     r.Use(custommiddleware.RequireUser(deps.Logger))
				//     r.Get("/me", userHandler.GetMe)     // Получить свой профиль
				//     r.Put("/me", userHandler.UpdateMe)  // Обновить свой профиль
				// })
			})
		})
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
