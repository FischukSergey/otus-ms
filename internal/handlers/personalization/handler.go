// Package personalization реализует HTTP-хендлеры personalization MVP.
package personalization

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/FischukSergey/otus-ms/internal/middleware"
	personalizationservice "github.com/FischukSergey/otus-ms/internal/services/personalization"
)

const (
	defaultLimit = 50
)

// Service определяет интерфейс сервиса personalization.
type Service interface {
	GetPreferences(ctx context.Context, userUUID string) (*personalizationservice.PreferencesResponse, error)
	UpdatePreferences(ctx context.Context, userUUID string, req personalizationservice.UpdatePreferencesRequest) error
	GetFeed(
		ctx context.Context,
		userUUID string,
		req personalizationservice.FeedRequest,
	) ([]personalizationservice.FeedItemResponse, error)
	CreateEvent(ctx context.Context, userUUID string, req personalizationservice.CreateEventRequest) error
}

// Handler обрабатывает HTTP-запросы personalization API.
type Handler struct {
	service Service
	logger  *slog.Logger
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

// PreferencesResponseSchema используется в Swagger для ответа предпочтений.
type PreferencesResponseSchema struct {
	PreferredCategories []string `json:"preferredCategories"`
	PreferredSources    []string `json:"preferredSources"`
	PreferredKeywords   []string `json:"preferredKeywords"`
	PreferredLanguage   string   `json:"preferredLanguage,omitempty"`
	FromHours           int      `json:"fromHours"`
	UpdatedAt           string   `json:"updatedAt,omitempty"`
}

// UpdatePreferencesRequestSchema используется в Swagger для запроса обновления предпочтений.
type UpdatePreferencesRequestSchema struct {
	PreferredCategories []string `json:"preferredCategories"`
	PreferredSources    []string `json:"preferredSources"`
	PreferredKeywords   []string `json:"preferredKeywords"`
	PreferredLanguage   string   `json:"preferredLanguage"`
	FromHours           int      `json:"fromHours"`
}

// FeedItemResponseSchema используется в Swagger для элемента персонализированной ленты.
type FeedItemResponseSchema struct {
	ID          string   `json:"id"`
	Topic       string   `json:"topic"`
	Source      string   `json:"source"`
	SourceID    string   `json:"sourceId"`
	URL         string   `json:"url"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags"`
	PublishedAt string   `json:"publishedAt,omitempty"`
	ProcessedAt string   `json:"processedAt"`
	CreatedAt   string   `json:"createdAt"`
	Score       float64  `json:"score"`
}

// CreateEventRequestSchema используется в Swagger для события пользователя.
type CreateEventRequestSchema struct {
	NewsID    string         `json:"newsId"`
	EventType string         `json:"eventType"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// NewHandler создает новый хендлер personalization API.
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

func (h *Handler) userIDFromContext(r *http.Request) (string, error) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok || userID == "" {
		return "", errors.New("user id not found in JWT context")
	}

	return userID, nil
}

func (h *Handler) resolveTargetUserID(r *http.Request) (string, int, string) {
	currentUserID, err := h.userIDFromContext(r)
	if err != nil {
		return "", http.StatusUnauthorized, "Authentication required"
	}

	targetUserID := strings.TrimSpace(r.URL.Query().Get("userUuid"))
	if targetUserID == "" || targetUserID == currentUserID {
		return currentUserID, 0, ""
	}

	claims, ok := middleware.GetClaimsFromContext(r.Context())
	if !ok || !claims.HasRole("admin") {
		return "", http.StatusForbidden, "Access denied - admin role required for userUuid override"
	}

	return targetUserID, 0, ""
}

// GetPreferences возвращает предпочтения авторизованного пользователя.
//
// @Summary      Получить предпочтения пользователя
// @Description  Возвращает настройки personalization для текущего пользователя (роль user/admin).
// @Tags         personalization
// @Produce      json
// @Param        userUuid  query     string  false  "UUID целевого пользователя (только для admin)"
// @Success      200  {object}  PreferencesResponseSchema                   "Предпочтения пользователя"
// @Failure      401  {object}  ErrorResponse                               "Не авторизован"
// @Failure      403  {object}  ErrorResponse                               "Недостаточно прав"
// @Failure      500  {object}  ErrorResponse                               "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/users/me/preferences [get].
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID, code, message := h.resolveTargetUserID(r)
	if code != 0 {
		h.writeError(w, r, code, message)
		return
	}

	prefs, err := h.service.GetPreferences(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(prefs); err != nil {
		logger := middleware.LoggerFromContext(r.Context())
		logger.Error("Failed to encode preferences response", "error", err)
	}
}

// UpdatePreferences обновляет предпочтения авторизованного пользователя.
//
// @Summary      Обновить предпочтения пользователя
// @Description  Создает или обновляет настройки personalization текущего пользователя (роль user/admin).
// @Tags         personalization
// @Accept       json
// @Produce      json
// @Param        userUuid  query     string                          false  "UUID целевого пользователя (только для admin)"
// @Param        payload  body      UpdatePreferencesRequestSchema                 true  "Настройки personalization"
// @Success      204
// @Failure      400  {object}  ErrorResponse  "Невалидный payload"
// @Failure      401  {object}  ErrorResponse  "Не авторизован"
// @Failure      403  {object}  ErrorResponse  "Недостаточно прав"
// @Failure      500  {object}  ErrorResponse  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/users/me/preferences [put].
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, code, message := h.resolveTargetUserID(r)
	if code != 0 {
		h.writeError(w, r, code, message)
		return
	}

	var req personalizationservice.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.UpdatePreferences(r.Context(), userID, req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetFeed возвращает персонализированную ленту пользователя.
//
// @Summary      Получить персонализированную ленту
// @Description  Возвращает новости с фильтрацией и ранжированием score для текущего пользователя (роль user/admin).
// @Tags         personalization
// @Produce      json
// @Param        userUuid   query     string  false  "UUID целевого пользователя (только для admin)"
// @Param        limit      query     int     false  "Лимит (1..100), по умолчанию 50"
// @Param        offset     query     int     false  "Смещение, по умолчанию 0"
// @Param        fromHours  query     int     false  "Окно выдачи в часах"
// @Param        q          query     string  false  "FTS запрос (websearch syntax)"
// @Success      200        {array}   FeedItemResponseSchema                    "Персонализированная лента"
// @Failure      400        {object}  ErrorResponse                            "Невалидные query-параметры"
// @Failure      401        {object}  ErrorResponse                            "Не авторизован"
// @Failure      403        {object}  ErrorResponse                            "Недостаточно прав"
// @Failure      500        {object}  ErrorResponse                            "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/news/feed [get].
func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	userID, code, message := h.resolveTargetUserID(r)
	if code != 0 {
		h.writeError(w, r, code, message)
		return
	}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
		limit = value
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid offset parameter")
			return
		}
		offset = value
	}

	fromHours := 0
	if raw := r.URL.Query().Get("fromHours"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid fromHours parameter")
			return
		}
		fromHours = value
	}

	items, err := h.service.GetFeed(r.Context(), userID, personalizationservice.FeedRequest{
		Limit:     limit,
		Offset:    offset,
		FromHours: fromHours,
		Query:     r.URL.Query().Get("q"),
	})
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		logger := middleware.LoggerFromContext(r.Context())
		logger.Error("Failed to encode feed response", "error", err)
	}
}

// CreateEvent сохраняет пользовательское событие по новости.
//
// @Summary      Отправить пользовательское событие
// @Description  Принимает события view/click/like/dislike/hide для текущего пользователя (роль user/admin).
// @Tags         personalization
// @Accept       json
// @Produce      json
// @Param        userUuid  query     string                    false  "UUID целевого пользователя (только для admin)"
// @Param        payload  body      CreateEventRequestSchema                   true  "Событие пользователя"
// @Success      202
// @Failure      400  {object}  ErrorResponse  "Невалидный payload"
// @Failure      401  {object}  ErrorResponse  "Не авторизован"
// @Failure      403  {object}  ErrorResponse  "Недостаточно прав"
// @Failure      500  {object}  ErrorResponse  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/news/events [post].
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, code, message := h.resolveTargetUserID(r)
	if code != 0 {
		h.writeError(w, r, code, message)
		return
	}

	var req personalizationservice.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.CreateEvent(r.Context(), userID, req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
