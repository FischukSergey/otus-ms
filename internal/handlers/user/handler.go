package user

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	userService "github.com/FischukSergey/otus-ms/internal/services/user"
	userRepo "github.com/FischukSergey/otus-ms/internal/store/user"
)

// Service определяет интерфейс для работы с сервисом пользователей.
type Service interface {
	CreateUser(ctx context.Context, req userService.CreateRequest) error
	GetUser(ctx context.Context, uuid string) (*userService.Response, error)
	DeleteUser(ctx context.Context, uuid string) error
}

// Handler обрабатывает HTTP запросы для работы с пользователями.
type Handler struct {
	service Service
	logger  *slog.Logger
}

// NewHandler создает новый экземпляр обработчика пользователей.
func NewHandler(service Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeError отправляет JSON ответ с ошибкой.
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
	}
}

// Create обрабатывает POST /api/v1/users - создание нового пользователя.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req userService.CreateRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request body", "error", err)
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Вызываем сервис для создания пользователя
	if err := h.service.CreateUser(r.Context(), req); err != nil {
		h.logger.Error("Failed to create user", "error", err)
		// Проверяем тип ошибки для определения статус-кода
		if errors.Is(err, errors.New("validation error")) ||
			err.Error()[:len("validation error")] == "validation error" {
			h.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем 201 Created без тела ответа
	w.WriteHeader(http.StatusCreated)
}

// Get обрабатывает GET /api/v1/users/{uuid} - получение пользователя по UUID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем UUID из URL
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		h.writeError(w, http.StatusBadRequest, "UUID is required")
		return
	}

	// Получаем пользователя
	user, err := h.service.GetUser(r.Context(), uuid)
	if err != nil {
		if errors.Is(err, userService.ErrInvalidUUID) {
			h.writeError(w, http.StatusBadRequest, "Invalid UUID format")
			return
		}
		if errors.Is(err, userRepo.ErrUserNotFound) {
			h.writeError(w, http.StatusNotFound, "User not found")
			return
		}
		h.logger.Error("Failed to get user", "error", err, "uuid", uuid)
		h.writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем данные пользователя (включая deleted=true если удален)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(user); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
	}
}

// Delete обрабатывает DELETE /api/v1/users/{uuid} - мягкое удаление пользователя.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	// Извлекаем UUID из URL
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		h.writeError(w, http.StatusBadRequest, "UUID is required")
		return
	}

	// Удаляем пользователя (мягкое удаление)
	if err := h.service.DeleteUser(r.Context(), uuid); err != nil {
		if errors.Is(err, userService.ErrInvalidUUID) {
			h.writeError(w, http.StatusBadRequest, "Invalid UUID format")
			return
		}
		if errors.Is(err, userRepo.ErrUserNotFound) {
			h.writeError(w, http.StatusNotFound, "User not found")
			return
		}
		h.logger.Error("Failed to delete user", "error", err, "uuid", uuid)
		h.writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
