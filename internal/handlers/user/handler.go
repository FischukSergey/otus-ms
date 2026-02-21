package user

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/FischukSergey/otus-ms/internal/middleware"
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
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	logger := middleware.LoggerFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		logger.Error("Failed to encode error response", "error", err)
	}
}

// Create создает нового пользователя.
//
// @Summary      Создать пользователя (только admin)
// @Description  Создаёт нового пользователя. UUID и email должны быть уникальными. Требуется роль admin.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user  body      userService.CreateRequest  true  "Данные нового пользователя"
// @Success      201
// @Failure      400  {object}  ErrorResponse  "Невалидный запрос или ошибка валидации"
// @Failure      401  {object}  ErrorResponse  "Не авторизован - отсутствует или невалидный JWT токен"
// @Failure      403  {object}  ErrorResponse  "Доступ запрещён - недостаточно прав (требуется роль admin)"
// @Failure      500  {object}  ErrorResponse  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/users [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())
	var req userService.CreateRequest

	// Парсим JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request body", "error", err)
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Вызываем сервис для создания пользователя
	if err := h.service.CreateUser(r.Context(), req); err != nil {
		logger.Error("Failed to create user", "error", err)
		// Проверяем тип ошибки для определения статус-кода
		if errors.Is(err, errors.New("validation error")) ||
			err.Error()[:len("validation error")] == "validation error" {
			h.writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем 201 Created без тела ответа
	w.WriteHeader(http.StatusCreated)
}

// Get возвращает пользователя по UUID.
//
// @Summary      Получить пользователя (только admin)
// @Description  Возвращает данные пользователя по UUID. Мягко удалённый вернётся с deleted=true. Требуется роль admin.
// @Tags         users
// @Produce      json
// @Param        uuid  path      string  true  "UUID пользователя (формат: uuid4)"
// @Success      200   {object}  userService.Response  "Данные пользователя"
// @Failure      400   {object}  ErrorResponse         "Невалидный UUID"
// @Failure      401   {object}  ErrorResponse         "Не авторизован - отсутствует или невалидный JWT токен"
// @Failure      403   {object}  ErrorResponse         "Доступ запрещён - недостаточно прав (требуется роль admin)"
// @Failure      404   {object}  ErrorResponse         "Пользователь не найден"
// @Failure      500   {object}  ErrorResponse         "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/users/{uuid} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())

	// Извлекаем UUID из URL
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		h.writeError(w, r, http.StatusBadRequest, "UUID is required")
		return
	}

	// Получаем пользователя
	user, err := h.service.GetUser(r.Context(), uuid)
	if err != nil {
		if errors.Is(err, userService.ErrInvalidUUID) {
			h.writeError(w, r, http.StatusBadRequest, "Invalid UUID format")
			return
		}
		if errors.Is(err, userRepo.ErrUserNotFound) {
			h.writeError(w, r, http.StatusNotFound, "User not found")
			return
		}
		logger.Error("Failed to get user", "error", err, "uuid", uuid)
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем данные пользователя (включая deleted=true если удален)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(user); err != nil {
		logger.Error("Failed to encode response", "error", err)
	}
}

// Delete удаляет пользователя (мягкое удаление).
//
// @Summary      Удалить пользователя (только admin)
// @Description  Мягкое удаление (soft delete). Запись остаётся в БД с флагом deleted=true. Требуется роль admin.
// @Tags         users
// @Produce      json
// @Param        uuid  path  string  true  "UUID пользователя (формат: uuid4)"
// @Success      204   "Пользователь успешно удалён"
// @Failure      400   {object}  ErrorResponse  "Невалидный UUID"
// @Failure      401   {object}  ErrorResponse  "Не авторизован - отсутствует или невалидный JWT токен"
// @Failure      403   {object}  ErrorResponse  "Доступ запрещён - недостаточно прав (требуется роль admin)"
// @Failure      404   {object}  ErrorResponse  "Пользователь не найден"
// @Failure      500   {object}  ErrorResponse  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/users/{uuid} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())

	// Извлекаем UUID из URL
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		h.writeError(w, r, http.StatusBadRequest, "UUID is required")
		return
	}

	// Удаляем пользователя (мягкое удаление)
	if err := h.service.DeleteUser(r.Context(), uuid); err != nil {
		if errors.Is(err, userService.ErrInvalidUUID) {
			h.writeError(w, r, http.StatusBadRequest, "Invalid UUID format")
			return
		}
		if errors.Is(err, userRepo.ErrUserNotFound) {
			h.writeError(w, r, http.StatusNotFound, "User not found")
			return
		}
		logger.Error("Failed to delete user", "error", err, "uuid", uuid)
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Возвращаем 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
