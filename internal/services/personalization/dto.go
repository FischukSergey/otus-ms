// Package personalization содержит DTO и бизнес-логику personalization MVP.
package personalization

import "time"

// PreferencesResponse представляет предпочтения пользователя.
type PreferencesResponse struct {
	PreferredCategories []string  `json:"preferredCategories"`
	PreferredSources    []string  `json:"preferredSources"`
	PreferredKeywords   []string  `json:"preferredKeywords"`
	PreferredLanguage   string    `json:"preferredLanguage,omitempty"`
	FromHours           int       `json:"fromHours"`
	UpdatedAt           time.Time `json:"updatedAt,omitempty"`
}

// UpdatePreferencesRequest представляет запрос на обновление предпочтений.
type UpdatePreferencesRequest struct {
	PreferredCategories []string `json:"preferredCategories"`
	PreferredSources    []string `json:"preferredSources"`
	PreferredKeywords   []string `json:"preferredKeywords"`
	PreferredLanguage   string   `json:"preferredLanguage"`
	FromHours           int      `json:"fromHours"`
}

// FeedRequest задает параметры выборки персонализированной ленты.
type FeedRequest struct {
	Limit     int
	Offset    int
	FromHours int
	Query     string
}

// FeedItemResponse представляет элемент персонализированной ленты.
type FeedItemResponse struct {
	ID          string     `json:"id"`
	Topic       string     `json:"topic"`
	Source      string     `json:"source"`
	SourceID    string     `json:"sourceId"`
	URL         string     `json:"url"`
	Category    string     `json:"category,omitempty"`
	Tags        []string   `json:"tags"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	ProcessedAt time.Time  `json:"processedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	Score       float64    `json:"score"`
}

// CreateEventRequest представляет запрос на создание пользовательского события.
type CreateEventRequest struct {
	NewsID    string         `json:"newsId"`
	EventType string         `json:"eventType"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
