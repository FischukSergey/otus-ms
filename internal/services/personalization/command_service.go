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

// PreferencesEventCommandService реализует command-операции personalization.
type PreferencesEventCommandService struct {
	repo PreferencesRepository
}

// NewPreferencesEventCommandService создает command-сервис personalization.
func NewPreferencesEventCommandService(repo PreferencesRepository) *PreferencesEventCommandService {
	return &PreferencesEventCommandService{repo: repo}
}

// GetPreferences возвращает предпочтения пользователя.
func (s *PreferencesEventCommandService) GetPreferences(
	ctx context.Context,
	userUUID string,
) (*PreferencesResponse, error) {
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
func (s *PreferencesEventCommandService) UpdatePreferences(
	ctx context.Context,
	userUUID string,
	req UpdatePreferencesRequest,
) error {
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

// CreateEvent сохраняет событие взаимодействия пользователя с новостью.
func (s *PreferencesEventCommandService) CreateEvent(
	ctx context.Context,
	userUUID string,
	req CreateEventRequest,
) error {
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
