package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/FischukSergey/otus-ms/internal/keycloak"
	"github.com/FischukSergey/otus-ms/internal/middleware"
)

// KeycloakClient определяет интерфейс для работы с Keycloak.
type KeycloakClient interface {
	Login(ctx context.Context, username, password string) (*keycloak.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*keycloak.TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
}

// Handler обрабатывает HTTP запросы для авторизации.
type Handler struct {
	keycloakClient KeycloakClient
	logger         *slog.Logger
	validator      *validator.Validate
}

// NewHandler создаёт новый экземпляр обработчика авторизации.
func NewHandler(keycloakClient KeycloakClient, logger *slog.Logger) *Handler {
	return &Handler{
		keycloakClient: keycloakClient,
		logger:         logger,
		validator:      validator.New(),
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

// Login обрабатывает POST /api/v1/auth/login - аутентификация пользователя.
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

// Refresh обрабатывает POST /api/v1/auth/refresh - обновление access token.
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

// Logout обрабатывает POST /api/v1/auth/logout - logout пользователя.
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
