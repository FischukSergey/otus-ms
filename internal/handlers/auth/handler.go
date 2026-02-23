// Package auth реализует HTTP-хендлеры для аутентификации пользователей.
package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/FischukSergey/otus-ms/internal/clients/mainservice"
	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/middleware"
)

// KeycloakClient определяет интерфейс для работы с Keycloak.
type KeycloakClient interface {
	Login(ctx context.Context, username, password string) (*keycloak.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*keycloak.TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	CreateUser(ctx context.Context, user keycloak.User) (string, error)
	DeleteUser(ctx context.Context, userID string) error
}

// MainServiceClient определяет интерфейс для работы с Main Service API.
type MainServiceClient interface {
	CreateUser(ctx context.Context, req mainservice.CreateUserRequest) error
}

// Handler обрабатывает HTTP запросы для авторизации.
type Handler struct {
	keycloakClient    KeycloakClient
	mainServiceClient MainServiceClient
	logger            *slog.Logger
	validator         *validator.Validate
}

// NewHandler создаёт новый экземпляр обработчика авторизации.
func NewHandler(
	keycloakClient KeycloakClient,
	mainServiceClient MainServiceClient,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		keycloakClient:    keycloakClient,
		mainServiceClient: mainServiceClient,
		logger:            logger,
		validator:         validator.New(),
	}
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeError отправляет JSON ответ с ошибкой.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	logger := middleware.LoggerFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		logger.Error("Failed to encode error response", "error", err)
	}
}

// writeJSON отправляет JSON ответ с данными.
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, statusCode int, data any) {
	logger := middleware.LoggerFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode response", "error", err)
	}
}

// getClientIP извлекает IP адрес клиента из запроса.
func getClientIP(r *http.Request) string {
	// Проверяем заголовок X-Real-IP
	ip := r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	// Проверяем заголовок X-Forwarded-For
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Берём первый IP из списка
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Используем RemoteAddr
	return r.RemoteAddr
}

// Login выполняет аутентификацию пользователя через Keycloak.
//
// @Summary      Вход в систему
// @Description  Аутентифицирует пользователя и возвращает access и refresh токены.
// @Description  Токены получаются от Keycloak.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      keycloak.LoginRequest   true  "Учетные данные пользователя"
// @Success      200          {object}  keycloak.TokenResponse  "Токены успешно получены"
// @Failure      400          {object}  keycloak.ErrorResponse  "Невалидный запрос или отсутствуют обязательные поля"
// @Failure      401          {object}  keycloak.ErrorResponse  "Неверные учетные данные"
// @Failure      500          {object}  keycloak.ErrorResponse  "Внутренняя ошибка сервера или ошибка Keycloak"
// @Router       /api/v1/auth/login [post].
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())
	clientIP := getClientIP(r)

	var req keycloak.LoginRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode login request", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидируем запрос
	if err := h.validator.Struct(req); err != nil {
		logger.Error("Login request validation failed", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Вызываем Keycloak для аутентификации
	tokens, err := h.keycloakClient.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		logger.Error("Login failed",
			"error", err,
			"username", req.Username,
			"ip", clientIP,
		)
		h.writeError(w, r, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Логируем успешный логин
	logger.Info("User logged in successfully",
		"username", req.Username,
		"ip", clientIP,
	)

	// Возвращаем токены
	h.writeJSON(w, r, http.StatusOK, tokens)
}

// Refresh обновляет access токен используя refresh токен.
//
// @Summary      Обновление токена
// @Description  Обменивает refresh токен на новую пару access и refresh токенов.
// @Description  Позволяет продлить сессию пользователя без повторного ввода пароля.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token    body      keycloak.RefreshRequest  true  "Refresh токен"
// @Success      200      {object}  keycloak.TokenResponse   "Новые токены успешно получены"
// @Failure      400      {object}  keycloak.ErrorResponse   "Невалидный запрос или отсутствует refresh token"
// @Failure      401      {object}  keycloak.ErrorResponse   "Невалидный или истекший refresh токен"
// @Failure      500      {object}  keycloak.ErrorResponse   "Внутренняя ошибка сервера или ошибка Keycloak"
// @Router       /api/v1/auth/refresh [post].
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())
	clientIP := getClientIP(r)

	var req keycloak.RefreshRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode refresh request", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидируем запрос
	if err := h.validator.Struct(req); err != nil {
		logger.Error("Refresh request validation failed", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Refresh token is required")
		return
	}

	// Вызываем Keycloak для обновления токена
	tokens, err := h.keycloakClient.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		logger.Error("Token refresh failed",
			"error", err,
			"ip", clientIP,
		)
		h.writeError(w, r, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Логируем успешное обновление
	logger.Info("Token refreshed successfully", "ip", clientIP)

	// Возвращаем новые токены
	h.writeJSON(w, r, http.StatusOK, tokens)
}

// Logout завершает сессию пользователя и инвалидирует токены.
//
// @Summary      Выход из системы
// @Description  Инвалидирует refresh токен в Keycloak, завершая сессию пользователя.
// @Description  После logout токены становятся недействительными.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token    body  keycloak.LogoutRequest  true  "Refresh токен для инвалидации"
// @Success      204      "Выход выполнен успешно"
// @Failure      400      {object}  keycloak.ErrorResponse  "Невалидный запрос или отсутствует refresh token"
// @Failure      500      {object}  keycloak.ErrorResponse  "Ошибка при выполнении logout в Keycloak"
// @Router       /api/v1/auth/logout [post].
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())
	clientIP := getClientIP(r)

	var req keycloak.LogoutRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode logout request", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидируем запрос
	if err := h.validator.Struct(req); err != nil {
		logger.Error("Logout request validation failed", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Refresh token is required")
		return
	}

	// Вызываем Keycloak для logout
	if err := h.keycloakClient.Logout(r.Context(), req.RefreshToken); err != nil {
		logger.Error("Logout failed",
			"error", err,
			"ip", clientIP,
		)
		h.writeError(w, r, http.StatusInternalServerError, "Logout failed")
		return
	}

	// Логируем успешный logout
	logger.Info("User logged out successfully", "ip", clientIP)

	// Возвращаем 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// Register регистрирует нового пользователя с ролью user.
//
// @Summary      Регистрация нового пользователя
// @Description  Создаёт пользователя в Keycloak и Main Service с ролью user.
// @Description  После регистрации необходимо выполнить login для получения токенов.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user  body  keycloak.RegisterRequest  true  "Данные для регистрации"
// @Success      201   "Пользователь успешно зарегистрирован"
// @Failure      400   {object}  keycloak.ErrorResponse  "Невалидные данные или ошибка валидации"
// @Failure      409   {object}  keycloak.ErrorResponse  "Пользователь с таким email уже существует"
// @Failure      500   {object}  keycloak.ErrorResponse  "Внутренняя ошибка сервера"
// @Router       /api/v1/auth/register [post].
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())
	clientIP := getClientIP(r)

	var req keycloak.RegisterRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode register request", "error", err, "ip", clientIP)
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидируем запрос
	if err := h.validator.Struct(req); err != nil {
		logger.Error("Register request validation failed",
			"error", err,
			"email", req.Email,
			"ip", clientIP,
		)
		h.writeError(w, r, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	// 1. Создаём пользователя в Keycloak
	logger.Info("Creating user in Keycloak",
		"email", req.Email,
		"ip", clientIP,
	)

	keycloakUserID, err := h.keycloakClient.CreateUser(r.Context(), keycloak.User(req))
	if err != nil {
		logger.Error("Failed to create user in Keycloak",
			"error", err,
			"email", req.Email,
			"ip", clientIP,
		)

		// Проверяем тип ошибки (409 Conflict = пользователь существует)
		errMsg := err.Error()
		if strings.Contains(errMsg, "409") ||
			strings.Contains(errMsg, "already exists") ||
			strings.Contains(errMsg, "User exists") {
			h.writeError(w, r, http.StatusConflict, "User with this email already exists")
			return
		}

		h.writeError(w, r, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// 2. Создаём пользователя в Main Service
	logger.Info("Creating user in Main Service",
		"uuid", keycloakUserID,
		"email", req.Email,
		"ip", clientIP,
	)

	// Подготавливаем запрос для Main Service
	var middleName *string
	if req.MiddleName != "" {
		middleName = &req.MiddleName
	}

	mainServiceReq := mainservice.CreateUserRequest{
		UUID:       keycloakUserID,
		Email:      req.Email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: middleName,
	}

	// Вызываем Main Service API
	// Клиент автоматически получит service account токен от Keycloak
	err = h.mainServiceClient.CreateUser(r.Context(), mainServiceReq)
	if err != nil {
		logger.Error("Failed to create user in Main Service",
			"error", err,
			"uuid", keycloakUserID,
			"email", req.Email,
			"ip", clientIP,
		)

		// Rollback: удаляем пользователя из Keycloak
		logger.Warn("Rolling back: deleting user from Keycloak",
			"uuid", keycloakUserID,
			"email", req.Email,
		)

		if delErr := h.keycloakClient.DeleteUser(r.Context(), keycloakUserID); delErr != nil {
			logger.Error("Failed to rollback user from Keycloak",
				"error", delErr,
				"uuid", keycloakUserID,
			)
		}

		h.writeError(w, r, http.StatusInternalServerError, "Failed to complete registration")
		return
	}

	// Успешная регистрация
	logger.Info("User registered successfully",
		"uuid", keycloakUserID,
		"email", req.Email,
		"ip", clientIP,
	)

	// Возвращаем 201 Created (БЕЗ токенов - пользователь должен выполнить login)
	w.WriteHeader(http.StatusCreated)
}
