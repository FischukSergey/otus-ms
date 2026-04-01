// Package newshttp реализует HTTP-хендлеры для чтения новостей.
package newshttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/FischukSergey/otus-ms/internal/middleware"
	newsService "github.com/FischukSergey/otus-ms/internal/services/news"
)

const defaultLimit = 50

// Service определяет интерфейс сервиса новостей.
type Service interface {
	ListLatest(ctx context.Context, limit int) ([]newsService.Response, error)
}

// Handler обрабатывает HTTP-запросы списка новостей.
type Handler struct {
	service Service
	logger  *slog.Logger
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewsResponse представляет запись новости для Swagger-схемы.
type NewsResponse struct {
	Topic     string    `json:"topic"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewHandler создаёт HTTP-хендлер новостей.
func NewHandler(service Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	logger := middleware.LoggerFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		logger.Error("Failed to encode error response", "error", err)
	}
}

// List возвращает последние новости.
//
// @Summary      Получить список новостей (только admin)
// @Description  Возвращает последние новости с полями topic, source и url. Требуется роль admin.
// @Tags         news
// @Produce      json
// @Param        limit  query     int   false  "Лимит записей (1..500), по умолчанию 50"
// @Success      200    {array}   NewsResponse          "Список новостей"
// @Failure      400    {object}  ErrorResponse         "Некорректный query-параметр limit"
// @Failure      401    {object}  ErrorResponse         "Не авторизован - отсутствует или невалидный JWT токен"
// @Failure      403    {object}  ErrorResponse         "Доступ запрещён - требуется роль admin"
// @Failure      500    {object}  ErrorResponse         "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/news [get].
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	logger := middleware.LoggerFromContext(r.Context())

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
		limit = parsed
	}

	news, err := h.service.ListLatest(r.Context(), limit)
	if err != nil {
		logger.Error("Failed to get latest news", "error", err, "limit", limit)
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(news); err != nil {
		logger.Error("Failed to encode response", "error", err)
	}
}
