package personalization

import (
	"context"

	"github.com/FischukSergey/otus-ms/internal/models"
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

// PreferencesRepository определяет интерфейс command-операций preferences/events.
type PreferencesRepository interface {
	GetPreferences(ctx context.Context, userUUID string) (*models.UserNewsPreferences, error)
	UpsertPreferences(ctx context.Context, prefs models.UserNewsPreferences) error
	InsertEvent(ctx context.Context, event models.UserNewsEvent) error
}

// FeedRepository определяет интерфейс query-операций персонализированной ленты.
type FeedRepository interface {
	GetPersonalizedFeed(ctx context.Context, filters models.PersonalizedFeedFilters) ([]models.PersonalizedNewsItem, error)
}

// Service — фасад personalization, сохраняет совместимый API для хендлеров.
type Service struct {
	command *PreferencesEventCommandService
	query   *FeedQueryService
}

// NewService создает фасадный сервис personalization.
func NewService(commandRepo PreferencesRepository, feedRepo FeedRepository) *Service {
	commandService := NewPreferencesEventCommandService(commandRepo)
	return &Service{
		command: commandService,
		query:   NewFeedQueryService(commandService, feedRepo),
	}
}

// GetPreferences возвращает предпочтения пользователя.
func (s *Service) GetPreferences(ctx context.Context, userUUID string) (*PreferencesResponse, error) {
	return s.command.GetPreferences(ctx, userUUID)
}

// UpdatePreferences обновляет предпочтения пользователя.
func (s *Service) UpdatePreferences(ctx context.Context, userUUID string, req UpdatePreferencesRequest) error {
	return s.command.UpdatePreferences(ctx, userUUID, req)
}

// GetFeed возвращает персонализированную ленту.
func (s *Service) GetFeed(ctx context.Context, userUUID string, req FeedRequest) ([]FeedItemResponse, error) {
	return s.query.GetFeed(ctx, userUUID, req)
}

// CreateEvent сохраняет событие взаимодействия пользователя с новостью.
func (s *Service) CreateEvent(ctx context.Context, userUUID string, req CreateEventRequest) error {
	return s.command.CreateEvent(ctx, userUUID, req)
}
