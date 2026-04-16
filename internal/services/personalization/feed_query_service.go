package personalization

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// PreferencesReader определяет минимальный read-контракт предпочтений для ленты.
type PreferencesReader interface {
	GetPreferences(ctx context.Context, userUUID string) (*PreferencesResponse, error)
}

// FeedQueryService реализует read-path персонализированной ленты.
type FeedQueryService struct {
	preferences PreferencesReader
	repo        FeedRepository
}

// NewFeedQueryService создает query-сервис персонализированной ленты.
func NewFeedQueryService(preferences PreferencesReader, repo FeedRepository) *FeedQueryService {
	return &FeedQueryService{
		preferences: preferences,
		repo:        repo,
	}
}

// GetFeed возвращает персонализированную ленту.
func (s *FeedQueryService) GetFeed(ctx context.Context, userUUID string, req FeedRequest) ([]FeedItemResponse, error) {
	prefs, err := s.preferences.GetPreferences(ctx, userUUID)
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
