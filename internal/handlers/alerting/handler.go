// Package alerting реализует HTTP-хендлеры alerting API.
package alerting

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/FischukSergey/otus-ms/internal/middleware"
	alertingservice "github.com/FischukSergey/otus-ms/internal/services/alerting"
)

const defaultLimit = 50

// Service определяет интерфейс сервиса alerting.
type Service interface {
	ListRules(ctx context.Context, userUUID string) ([]alertingservice.RuleResponse, error)
	CreateRule(ctx context.Context, userUUID string, req alertingservice.CreateRuleRequest) (*alertingservice.RuleResponse, error)
	UpdateRule(ctx context.Context, userUUID, ruleID string, req alertingservice.UpdateRuleRequest) error
	DeleteRule(ctx context.Context, userUUID, ruleID string) error
	ListEvents(ctx context.Context, userUUID string, req alertingservice.ListEventsRequest) ([]alertingservice.EventResponse, error)
}

// Handler обрабатывает HTTP-запросы alerting API.
type Handler struct {
	service Service
	logger  *slog.Logger
}

// ErrorResponse представляет структуру ответа с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewHandler создает новый хендлер alerting API.
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

// ListRules возвращает список правил алертинга текущего пользователя.
//
// @Summary      Получить правила алертинга
// @Description  Возвращает список правил алертинга текущего пользователя (роль user/admin).
// @Tags         alerts
// @Produce      json
// @Success      200  {array}   alertingservice.RuleResponse  "Список правил"
// @Failure      401  {object}  ErrorResponse                 "Не авторизован"
// @Failure      500  {object}  ErrorResponse                 "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/alerts/rules [get].
func (h *Handler) ListRules(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userIDFromContext(r)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	items, err := h.service.ListRules(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		logger := middleware.LoggerFromContext(r.Context())
		logger.Error("Failed to encode alert rules response", "error", err)
	}
}

// CreateRule создает новое правило алертинга.
//
// @Summary      Создать правило алертинга
// @Description  Создает правило алертинга текущего пользователя (роль user/admin).
// @Tags         alerts
// @Accept       json
// @Produce      json
// @Param        payload  body      alertingservice.CreateRuleRequest  true  "Данные правила"
// @Success      201      {object}  alertingservice.RuleResponse        "Созданное правило"
// @Failure      400      {object}  ErrorResponse                       "Невалидный payload"
// @Failure      401      {object}  ErrorResponse                       "Не авторизован"
// @Failure      500      {object}  ErrorResponse                       "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/alerts/rules [post].
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userIDFromContext(r)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req alertingservice.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.service.CreateRule(r.Context(), userID, req)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(rule); err != nil {
		logger := middleware.LoggerFromContext(r.Context())
		logger.Error("Failed to encode create alert rule response", "error", err)
	}
}

// UpdateRule обновляет правило алертинга пользователя.
//
// @Summary      Обновить правило алертинга
// @Description  Обновляет keyword/cooldown/isActive правила текущего пользователя (роль user/admin).
// @Tags         alerts
// @Accept       json
// @Produce      json
// @Param        id       path      string                               true  "UUID правила"
// @Param        payload  body      alertingservice.UpdateRuleRequest    true  "Обновленные данные правила"
// @Success      204
// @Failure      400      {object}  ErrorResponse                        "Невалидный payload"
// @Failure      401      {object}  ErrorResponse                        "Не авторизован"
// @Failure      404      {object}  ErrorResponse                        "Правило не найдено"
// @Failure      500      {object}  ErrorResponse                        "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/alerts/rules/{id} [put].
func (h *Handler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userIDFromContext(r)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	ruleID := chi.URLParam(r, "id")
	if ruleID == "" {
		h.writeError(w, r, http.StatusBadRequest, "Rule ID is required")
		return
	}

	var req alertingservice.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	err = h.service.UpdateRule(r.Context(), userID, ruleID, req)
	if err != nil {
		if errors.Is(err, alertingservice.ErrRuleNotFound) {
			h.writeError(w, r, http.StatusNotFound, "Alert rule not found")
			return
		}
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteRule удаляет правило алертинга пользователя.
//
// @Summary      Удалить правило алертинга
// @Description  Удаляет правило текущего пользователя (роль user/admin).
// @Tags         alerts
// @Produce      json
// @Param        id  path      string         true  "UUID правила"
// @Success      204
// @Failure      400  {object}  ErrorResponse  "Невалидный UUID"
// @Failure      401  {object}  ErrorResponse  "Не авторизован"
// @Failure      404  {object}  ErrorResponse  "Правило не найдено"
// @Failure      500  {object}  ErrorResponse  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/alerts/rules/{id} [delete].
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userIDFromContext(r)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	ruleID := chi.URLParam(r, "id")
	if ruleID == "" {
		h.writeError(w, r, http.StatusBadRequest, "Rule ID is required")
		return
	}

	err = h.service.DeleteRule(r.Context(), userID, ruleID)
	if err != nil {
		if errors.Is(err, alertingservice.ErrRuleNotFound) {
			h.writeError(w, r, http.StatusNotFound, "Alert rule not found")
			return
		}
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListEvents возвращает историю событий алертинга.
//
// @Summary      Получить историю алертов
// @Description  Возвращает историю доставок алертов текущего пользователя (роль user/admin).
// @Tags         alerts
// @Produce      json
// @Param        limit   query     int     false  "Лимит (default 50, max 200)"
// @Param        offset  query     int     false  "Смещение (default 0)"
// @Param        status  query     string  false  "Статус (pending, sent, failed, dropped)"
// @Success      200     {array}   alertingservice.EventResponse  "История событий"
// @Failure      400     {object}  ErrorResponse                  "Невалидные query-параметры"
// @Failure      401     {object}  ErrorResponse                  "Не авторизован"
// @Failure      500     {object}  ErrorResponse                  "Внутренняя ошибка сервера"
// @Security     BearerAuth
// @Router       /api/v1/alerts/events [get].
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userIDFromContext(r)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
		limit = value
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil {
			h.writeError(w, r, http.StatusBadRequest, "Invalid offset parameter")
			return
		}
		offset = value
	}

	items, err := h.service.ListEvents(r.Context(), userID, alertingservice.ListEventsRequest{
		Limit:  limit,
		Offset: offset,
		Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		logger := middleware.LoggerFromContext(r.Context())
		logger.Error("Failed to encode alert events response", "error", err)
	}
}
