package alerting

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/FischukSergey/otus-ms/internal/models"
)

const (
	defaultCooldownSeconds = 300
	defaultEventsLimit     = 50
	maxEventsLimit         = 200
	maxActiveRulesPerUser  = 20
	telegramChannelType    = "telegram"
)

var allowedStatuses = []string{"pending", "sent", "failed", "dropped"}

// ErrRuleNotFound возвращается, когда правило не найдено.
var ErrRuleNotFound = errors.New("alert rule not found")

// Repository определяет интерфейс доступа к данным alerting.
type Repository interface {
	ListRules(ctx context.Context, userUUID string) ([]models.AlertRule, error)
	CountActiveRules(ctx context.Context, userUUID string) (int, error)
	CreateRule(ctx context.Context, rule models.AlertRule) error
	UpdateRule(ctx context.Context, rule models.AlertRule) (bool, error)
	DeleteRule(ctx context.Context, userUUID, ruleID string) (bool, error)
	ListEvents(ctx context.Context, userUUID string, limit, offset int, status string) ([]models.AlertEvent, error)
}

// Service реализует бизнес-логику alerting MVP.
type Service struct {
	repo Repository
}

// NewService создает сервис alerting.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListRules возвращает все правила пользователя.
func (s *Service) ListRules(ctx context.Context, userUUID string) ([]RuleResponse, error) {
	if strings.TrimSpace(userUUID) == "" {
		return nil, errors.New("user uuid is required")
	}

	rules, err := s.repo.ListRules(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}

	result := make([]RuleResponse, 0, len(rules))
	for i := range rules {
		result = append(result, RuleResponse{
			ID:              rules[i].ID,
			UserUUID:        rules[i].UserUUID,
			Keyword:         rules[i].Keyword,
			IsActive:        rules[i].IsActive,
			ChannelType:     rules[i].ChannelType,
			ChannelTarget:   rules[i].ChannelTarget,
			CooldownSeconds: rules[i].CooldownSeconds,
			CreatedAt:       rules[i].CreatedAt,
			UpdatedAt:       rules[i].UpdatedAt,
		})
	}

	return result, nil
}

// CreateRule создает правило алертинга.
func (s *Service) CreateRule(ctx context.Context, userUUID string, req CreateRuleRequest) (*RuleResponse, error) {
	keyword := normalizeKeyword(req.Keyword)
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}
	if strings.TrimSpace(userUUID) == "" {
		return nil, errors.New("user uuid is required")
	}

	activeCount, err := s.repo.CountActiveRules(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("count active alert rules: %w", err)
	}
	if activeCount >= maxActiveRulesPerUser {
		return nil, fmt.Errorf("active rules limit reached: max %d", maxActiveRulesPerUser)
	}

	cooldown, err := normalizeCooldown(req.CooldownSeconds)
	if err != nil {
		return nil, err
	}

	rule := models.AlertRule{
		ID:              uuid.NewString(),
		UserUUID:        userUUID,
		Keyword:         keyword,
		IsActive:        true,
		ChannelType:     normalizeChannelType(req.ChannelType),
		ChannelTarget:   strings.TrimSpace(req.ChannelTarget),
		CooldownSeconds: cooldown,
	}
	if rule.ChannelType != telegramChannelType {
		return nil, fmt.Errorf("channelType must be %s", telegramChannelType)
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}

	created := &RuleResponse{
		ID:              rule.ID,
		UserUUID:        rule.UserUUID,
		Keyword:         rule.Keyword,
		IsActive:        rule.IsActive,
		ChannelType:     rule.ChannelType,
		ChannelTarget:   rule.ChannelTarget,
		CooldownSeconds: rule.CooldownSeconds,
	}

	return created, nil
}

// UpdateRule обновляет правило алертинга пользователя.
func (s *Service) UpdateRule(
	ctx context.Context,
	userUUID, ruleID string,
	req UpdateRuleRequest,
) error {
	if strings.TrimSpace(userUUID) == "" {
		return errors.New("user uuid is required")
	}
	if _, err := uuid.Parse(ruleID); err != nil {
		return errors.New("rule id must be a valid UUID")
	}

	keyword := normalizeKeyword(req.Keyword)
	if keyword == "" {
		return errors.New("keyword is required")
	}

	cooldown, err := normalizeCooldown(req.CooldownSeconds)
	if err != nil {
		return err
	}

	channelType := normalizeChannelType(req.ChannelType)
	if channelType != telegramChannelType {
		return fmt.Errorf("channelType must be %s", telegramChannelType)
	}

	rule := models.AlertRule{
		ID:              ruleID,
		UserUUID:        userUUID,
		Keyword:         keyword,
		IsActive:        req.IsActive,
		ChannelType:     channelType,
		ChannelTarget:   strings.TrimSpace(req.ChannelTarget),
		CooldownSeconds: cooldown,
	}

	updated, err := s.repo.UpdateRule(ctx, rule)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	if !updated {
		return ErrRuleNotFound
	}

	return nil
}

// DeleteRule удаляет правило пользователя.
func (s *Service) DeleteRule(ctx context.Context, userUUID, ruleID string) error {
	if strings.TrimSpace(userUUID) == "" {
		return errors.New("user uuid is required")
	}
	if _, err := uuid.Parse(ruleID); err != nil {
		return errors.New("rule id must be a valid UUID")
	}

	deleted, err := s.repo.DeleteRule(ctx, userUUID, ruleID)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if !deleted {
		return ErrRuleNotFound
	}

	return nil
}

// ListEvents возвращает историю событий алертинга пользователя.
func (s *Service) ListEvents(ctx context.Context, userUUID string, req ListEventsRequest) ([]EventResponse, error) {
	if strings.TrimSpace(userUUID) == "" {
		return nil, errors.New("user uuid is required")
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && !slices.Contains(allowedStatuses, status) {
		return nil, errors.New("status must be one of: pending, sent, failed, dropped")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultEventsLimit
	}
	limit = min(limit, maxEventsLimit)

	offset := max(req.Offset, 0)

	events, err := s.repo.ListEvents(ctx, userUUID, limit, offset, status)
	if err != nil {
		return nil, fmt.Errorf("list alert events: %w", err)
	}

	result := make([]EventResponse, 0, len(events))
	for i := range events {
		result = append(result, EventResponse{
			ID:             events[i].ID,
			RuleID:         events[i].RuleID,
			NewsID:         events[i].NewsID,
			UserUUID:       events[i].UserUUID,
			Keyword:        events[i].Keyword,
			DeliveryStatus: events[i].DeliveryStatus,
			ErrorMessage:   events[i].ErrorMessage,
			SentAt:         events[i].SentAt,
			CreatedAt:      events[i].CreatedAt,
		})
	}

	return result, nil
}

func normalizeKeyword(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeChannelType(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return telegramChannelType
	}
	return trimmed
}

func normalizeCooldown(value int) (int, error) {
	if value <= 0 {
		return defaultCooldownSeconds, nil
	}
	if value > 86400 {
		return 0, errors.New("cooldownSeconds must be <= 86400")
	}
	return value, nil
}
