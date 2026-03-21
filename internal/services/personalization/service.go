package personalization

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/FischukSergey/otus-ms/internal/models"
	personalizationrepo "github.com/FischukSergey/otus-ms/internal/store/personalization"
)

const (
	defaultFromHours = 168
	defaultLimit     = 50
	maxLimit         = 100
	maxFromHours     = 720
)

var (
	allowedEventTypes = []string{"view", "click", "like", "dislike", "hide"}
	allowedLanguages  = []string{"", "ru", "en"}
)

// Repository определяет интерфейс доступа к данным personalization.
type Repository interface {
	GetPreferences(ctx context.Context, userUUID string) (*models.UserNewsPreferences, error)
	UpsertPreferences(ctx context.Context, prefs models.UserNewsPreferences) error
	InsertEvent(ctx context.Context, event models.UserNewsEvent) error
	GetPersonalizedFeed(ctx context.Context, filters models.PersonalizedFeedFilters) ([]models.PersonalizedNewsItem, error)
}

// Service реализует бизнес-логику personalization MVP.
type Service struct {
	repo Repository
}

// NewService создает новый сервис personalization.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetPreferences возвращает предпочтения пользователя.
func (s *Service) GetPreferences(ctx context.Context, userUUID string) (*PreferencesResponse, error) {
	prefs, err := s.repo.GetPreferences(ctx, userUUID)
	if err != nil {
		if errors.Is(err, personalizationrepo.ErrPreferencesNotFound) {
			return &PreferencesResponse{
				PreferredCategories: []string{},
				PreferredSources:    []string{},
				PreferredKeywords:   []string{},
				PreferredLanguage:   "",
				FromHours:           defaultFromHours,
			}, nil
		}
		return nil, fmt.Errorf("get preferences: %w", err)
	}

	return &PreferencesResponse{
		PreferredCategories: slices.Clone(prefs.PreferredCategories),
		PreferredSources:    slices.Clone(prefs.PreferredSources),
		PreferredKeywords:   slices.Clone(prefs.PreferredKeywords),
		PreferredLanguage:   strings.Clone(prefs.PreferredLanguage),
		FromHours:           prefs.FromHours,
		UpdatedAt:           prefs.UpdatedAt,
	}, nil
}

// UpdatePreferences обновляет предпочтения пользователя.
func (s *Service) UpdatePreferences(ctx context.Context, userUUID string, req UpdatePreferencesRequest) error {
	if userUUID == "" {
		return errors.New("user uuid is required")
	}

	language := strings.ToLower(strings.TrimSpace(req.PreferredLanguage))
	if !slices.Contains(allowedLanguages, language) {
		return errors.New("preferredLanguage must be one of: ru, en")
	}

	fromHours := req.FromHours
	if fromHours <= 0 {
		fromHours = defaultFromHours
	}
	if fromHours > maxFromHours {
		return fmt.Errorf("fromHours must be <= %d", maxFromHours)
	}

	prefs := models.UserNewsPreferences{
		UserUUID:            userUUID,
		PreferredCategories: normalizeStrings(req.PreferredCategories),
		PreferredSources:    normalizeStrings(req.PreferredSources),
		PreferredKeywords:   normalizeStrings(req.PreferredKeywords),
		PreferredLanguage:   language,
		FromHours:           fromHours,
	}

	if err := s.repo.UpsertPreferences(ctx, prefs); err != nil {
		return fmt.Errorf("update preferences: %w", err)
	}

	return nil
}

// GetFeed возвращает персонализированную ленту.
func (s *Service) GetFeed(ctx context.Context, userUUID string, req FeedRequest) ([]FeedItemResponse, error) {
	prefs, err := s.GetPreferences(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("load preferences for feed: %w", err)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	offset := max(req.Offset, 0)
	fromHours := req.FromHours
	if fromHours <= 0 {
		fromHours = prefs.FromHours
	}
	if fromHours <= 0 {
		fromHours = defaultFromHours
	}
	fromHours = min(fromHours, maxFromHours)

	items, err := s.repo.GetPersonalizedFeed(ctx, models.PersonalizedFeedFilters{
		UserUUID:            userUUID,
		Limit:               limit,
		Offset:              offset,
		FromHours:           fromHours,
		PreferredCategories: slices.Clone(prefs.PreferredCategories),
		PreferredSources:    slices.Clone(prefs.PreferredSources),
		PreferredKeywords:   slices.Clone(prefs.PreferredKeywords),
		PreferredLanguage:   strings.Clone(prefs.PreferredLanguage),
		Query:               strings.TrimSpace(req.Query),
	})
	if err != nil {
		return nil, fmt.Errorf("get personalized feed: %w", err)
	}

	resp := make([]FeedItemResponse, 0, len(items))
	for i := range items {
		resp = append(resp, FeedItemResponse{
			ID:          items[i].ID,
			Topic:       items[i].Topic,
			Source:      items[i].Source,
			SourceID:    items[i].SourceID,
			URL:         items[i].URL,
			Category:    items[i].Category,
			Tags:        slices.Clone(items[i].Tags),
			PublishedAt: items[i].PublishedAt,
			ProcessedAt: items[i].ProcessedAt,
			CreatedAt:   items[i].CreatedAt,
			Score:       items[i].Score,
		})
	}

	return resp, nil
}

// CreateEvent сохраняет событие взаимодействия пользователя с новостью.
func (s *Service) CreateEvent(ctx context.Context, userUUID string, req CreateEventRequest) error {
	if userUUID == "" {
		return errors.New("user uuid is required")
	}
	if _, err := uuid.Parse(req.NewsID); err != nil {
		return errors.New("newsId must be a valid UUID")
	}

	eventType := strings.ToLower(strings.TrimSpace(req.EventType))
	if !slices.Contains(allowedEventTypes, eventType) {
		return errors.New("eventType must be one of: view, click, like, dislike, hide")
	}

	event := models.UserNewsEvent{
		ID:        uuid.NewString(),
		UserUUID:  userUUID,
		NewsID:    req.NewsID,
		EventType: eventType,
		Metadata:  mapsOrEmpty(req.Metadata),
	}

	if err := s.repo.InsertEvent(ctx, event); err != nil {
		return fmt.Errorf("create user event: %w", err)
	}

	return nil
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "" || slices.Contains(normalized, trimmed) {
			continue
		}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func mapsOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}

	return value
}
