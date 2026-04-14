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
	alertinghandler "github.com/FischukSergey/otus-ms/internal/handlers/alerting"
	newshttphandler "github.com/FischukSergey/otus-ms/internal/handlers/newshttp"
	personalizationhandler "github.com/FischukSergey/otus-ms/internal/handlers/personalization"
	userhandler "github.com/FischukSergey/otus-ms/internal/handlers/user"
	"github.com/FischukSergey/otus-ms/internal/jwks"
	"github.com/FischukSergey/otus-ms/internal/metrics"
	custommiddleware "github.com/FischukSergey/otus-ms/internal/middleware"
	alertingservice "github.com/FischukSergey/otus-ms/internal/services/alerting"
	newsservice "github.com/FischukSergey/otus-ms/internal/services/news"
	personalizationservice "github.com/FischukSergey/otus-ms/internal/services/personalization"
	userservice "github.com/FischukSergey/otus-ms/internal/services/user"
	"github.com/FischukSergey/otus-ms/internal/store"
	alertingrepo "github.com/FischukSergey/otus-ms/internal/store/alerting"
	newsrepo "github.com/FischukSergey/otus-ms/internal/store/news"
	personalizationrepo "github.com/FischukSergey/otus-ms/internal/store/personalization"
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
	newsRepository := newsrepo.NewRepository(deps.Storage.DB())
	newsService := newsservice.NewService(newsRepository)
	newsHandler := newshttphandler.NewHandler(newsService, deps.Logger)
	personalizationWriteRepository := personalizationrepo.NewRepository(deps.Storage.DB())
	personalizationFeedQueryRepository := personalizationrepo.NewFeedQueryRepository(deps.Storage.DB())
	personalizationService := personalizationservice.NewService(
		personalizationWriteRepository,
		personalizationFeedQueryRepository,
	)
	personalizationHandler := personalizationhandler.NewHandler(personalizationService, deps.Logger)
	alertingRepository := alertingrepo.NewRepository(deps.Storage.DB())
	alertingService := alertingservice.NewService(alertingRepository)
	alertingHandler := alertinghandler.NewHandler(alertingService, deps.Logger)

	// Роуты
	router.Get("/", apiSrv.handleRoot)
	router.Get("/health", apiSrv.handleHealth)

	// Swagger UI
	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.UIConfig(map[string]string{
			// Автоматически добавляет префикс "Bearer " если его нет
			"requestInterceptor": `(req) => {
				const auth = req.headers['Authorization'];
				if (auth && !auth.startsWith('Bearer ')) {
					req.headers['Authorization'] = 'Bearer ' + auth;
				}
				return req;
			}`,
		}),
	))

	// API роуты
	router.Route("/api/v1", func(r chi.Router) {
		// Публичные роуты (без JWT) - если понадобятся
		// r.Post("/register", someHandler.Register)

		// Защищённые роуты - требуется валидный JWT
		r.Group(func(r chi.Router) {
			// JWT валидация для всех роутов в группе
			// Применяется если JWT настроен (production) или в тестовом режиме
			if deps.Config.JWT.IsConfigured() {
				// Определяем issuer и audience
				issuer := deps.Config.JWT.Issuer
				audience := deps.Config.JWT.Audience

				// Если не указаны явно, пытаемся взять из Keycloak конфига
				if issuer == "" && deps.Config.Keycloak.IsConfigured() {
					issuer = deps.Config.Keycloak.Issuer()
				}
				if audience == "" && deps.Config.Keycloak.IsConfigured() {
					audience = deps.Config.Keycloak.ClientID
				}

				r.Use(custommiddleware.ValidateJWT(
					custommiddleware.JWTConfig{
						Issuer:     issuer,
						Audience:   audience,
						SkipVerify: deps.Config.JWT.SkipVerify,
					},
					deps.JWKSManager, // Может быть nil в тестовом режиме
					deps.Logger,
				))

				// Чтение новостей доступно только администраторам.
				r.Group(func(r chi.Router) {
					r.Use(custommiddleware.RequireAdmin(deps.Logger))
					r.Get("/news", newsHandler.List)
				})

				// Персонализация доступна для user и admin.
				r.Group(func(r chi.Router) {
					r.Use(custommiddleware.RequireUser(deps.Logger))
					r.Get("/news/feed", personalizationHandler.GetFeed)
					r.Post("/news/events", personalizationHandler.CreateEvent)
					r.Get("/alerts/rules", alertingHandler.ListRules)
					r.Post("/alerts/rules", alertingHandler.CreateRule)
					r.Put("/alerts/rules/{id}", alertingHandler.UpdateRule)
					r.Delete("/alerts/rules/{id}", alertingHandler.DeleteRule)
					r.Get("/alerts/events", alertingHandler.ListEvents)
				})

				// Роуты для работы с пользователями
				r.Route("/users", func(r chi.Router) {
					// Создание пользователя доступно для service account (Auth-Proxy) и admin
					r.Group(func(r chi.Router) {
						r.Use(custommiddleware.RequireRole([]string{"service-account", "admin"}, deps.Logger))
						r.Post("/", userHandler.Create) // Создать пользователя
					})

					// Только для администраторов
					r.Group(func(r chi.Router) {
						r.Use(custommiddleware.RequireAdmin(deps.Logger))
						r.Get("/", userHandler.List)            // Получить список всех пользователей
						r.Get("/{uuid}", userHandler.Get)       // Получить любого пользователя
						r.Delete("/{uuid}", userHandler.Delete) // Удалить пользователя
					})

					// Для пользователей с ролью user или admin (если понадобятся)
					r.Group(func(r chi.Router) {
						r.Use(custommiddleware.RequireUser(deps.Logger))
						r.Get("/me/preferences", personalizationHandler.GetPreferences)
						r.Put("/me/preferences", personalizationHandler.UpdatePreferences)
					})
				})
			} else {
				deps.Logger.Warn("JWT not configured - API endpoints are UNPROTECTED! This should only happen in development.")
			}
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
