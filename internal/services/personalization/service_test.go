package personalization_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/models"
	"github.com/FischukSergey/otus-ms/internal/services/personalization"
	personalizationrepo "github.com/FischukSergey/otus-ms/internal/store/personalization"
)

func TestGetPreferences_WhenNotFound_ReturnsDefaults(t *testing.T) {
	commandRepo := &mockPreferencesRepository{
		getPreferencesErr: personalizationrepo.ErrPreferencesNotFound,
	}
	svc := personalization.NewService(commandRepo, &mockFeedRepository{})

	got, err := svc.GetPreferences(context.Background(), "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 168, got.FromHours)
	assert.Empty(t, got.PreferredCategories)
	assert.Empty(t, got.PreferredSources)
	assert.Empty(t, got.PreferredKeywords)
}

func TestUpdatePreferences_ValidatesLanguage(t *testing.T) {
	commandRepo := &mockPreferencesRepository{}
	svc := personalization.NewService(commandRepo, &mockFeedRepository{})

	err := svc.UpdatePreferences(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		personalization.UpdatePreferencesRequest{
			PreferredLanguage: "de",
			FromHours:         24,
		},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "preferredLanguage")
}

func TestUpdatePreferences_NormalizesValuesAndDefaults(t *testing.T) {
	commandRepo := &mockPreferencesRepository{}
	svc := personalization.NewService(commandRepo, &mockFeedRepository{})

	err := svc.UpdatePreferences(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		personalization.UpdatePreferencesRequest{
			PreferredCategories: []string{" Tech ", "tech", "  ", "SCIENCE"},
			PreferredSources:    []string{" source_2 ", "SOURCE_2", "source_3"},
			PreferredKeywords:   []string{"AI", " ai", "GoLang"},
			PreferredLanguage:   "RU",
			FromHours:           0,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, commandRepo.upsertedPreferences)

	assert.Equal(t, []string{"tech", "science"}, commandRepo.upsertedPreferences.PreferredCategories)
	assert.Equal(t, []string{"source_2", "source_3"}, commandRepo.upsertedPreferences.PreferredSources)
	assert.Equal(t, []string{"ai", "golang"}, commandRepo.upsertedPreferences.PreferredKeywords)
	assert.Equal(t, "ru", commandRepo.upsertedPreferences.PreferredLanguage)
	assert.Equal(t, 168, commandRepo.upsertedPreferences.FromHours)
}

func TestCreateEvent_Validation(t *testing.T) {
	commandRepo := &mockPreferencesRepository{}
	svc := personalization.NewService(commandRepo, &mockFeedRepository{})

	err := svc.CreateEvent(context.Background(), "11111111-1111-1111-1111-111111111111", personalization.CreateEventRequest{
		NewsID:    "not-uuid",
		EventType: "view",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newsId")

	err = svc.CreateEvent(context.Background(), "11111111-1111-1111-1111-111111111111", personalization.CreateEventRequest{
		NewsID:    "22222222-2222-2222-2222-222222222222",
		EventType: "open",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eventType")
}

func TestGetFeed_UsesPreferencesAndRequestOverrides(t *testing.T) {
	now := time.Now().UTC()
	commandRepo := &mockPreferencesRepository{
		preferences: &models.UserNewsPreferences{
			UserUUID:            "11111111-1111-1111-1111-111111111111",
			PreferredCategories: []string{"tech"},
			PreferredSources:    []string{"source_3"},
			PreferredKeywords:   []string{"ai"},
			PreferredLanguage:   "ru",
			FromHours:           48,
		},
	}
	feedRepo := &mockFeedRepository{
		feedItems: []models.PersonalizedNewsItem{
			{
				ID:        "33333333-3333-3333-3333-333333333333",
				Topic:     "AI updates",
				Source:    "Habr",
				SourceID:  "source_3",
				URL:       "https://example.com/ai",
				Score:     2.5,
				CreatedAt: now,
			},
		},
	}
	svc := personalization.NewService(commandRepo, feedRepo)

	got, err := svc.GetFeed(context.Background(), "11111111-1111-1111-1111-111111111111", personalization.FeedRequest{
		Limit:     200, // проверка cap до 100
		Offset:    -10, // проверка нормализации до 0
		FromHours: 24,
		Query:     "ai chips",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, feedRepo.lastFeedFilters)

	assert.Equal(t, 100, feedRepo.lastFeedFilters.Limit)
	assert.Equal(t, 0, feedRepo.lastFeedFilters.Offset)
	assert.Equal(t, 24, feedRepo.lastFeedFilters.FromHours)
	assert.Equal(t, []string{"tech"}, feedRepo.lastFeedFilters.PreferredCategories)
	assert.Equal(t, []string{"source_3"}, feedRepo.lastFeedFilters.PreferredSources)
	assert.Equal(t, []string{"ai"}, feedRepo.lastFeedFilters.PreferredKeywords)
	assert.Equal(t, "ru", feedRepo.lastFeedFilters.PreferredLanguage)
	assert.Equal(t, "ai chips", feedRepo.lastFeedFilters.Query)
}

type mockPreferencesRepository struct {
	preferences         *models.UserNewsPreferences
	getPreferencesErr   error
	upsertedPreferences *models.UserNewsPreferences
	insertedEvent       *models.UserNewsEvent
}

func (m *mockPreferencesRepository) GetPreferences(_ context.Context, _ string) (*models.UserNewsPreferences, error) {
	if m.getPreferencesErr != nil {
		return nil, m.getPreferencesErr
	}
	if m.preferences == nil {
		return nil, personalizationrepo.ErrPreferencesNotFound
	}
	return m.preferences, nil
}

func (m *mockPreferencesRepository) UpsertPreferences(_ context.Context, prefs models.UserNewsPreferences) error {
	m.upsertedPreferences = &prefs
	return nil
}

func (m *mockPreferencesRepository) InsertEvent(_ context.Context, event models.UserNewsEvent) error {
	if event.ID == "" {
		return errors.New("event id is empty")
	}
	m.insertedEvent = &event
	return nil
}

type mockFeedRepository struct {
	feedItems       []models.PersonalizedNewsItem
	feedErr         error
	lastFeedFilters *models.PersonalizedFeedFilters
}

func (m *mockFeedRepository) GetPersonalizedFeed(
	_ context.Context,
	filters models.PersonalizedFeedFilters,
) ([]models.PersonalizedNewsItem, error) {
	m.lastFeedFilters = &filters
	if m.feedErr != nil {
		return nil, m.feedErr
	}
	return m.feedItems, nil
}
