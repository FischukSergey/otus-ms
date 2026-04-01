package models

import "time"

// PersonalizedNewsItem представляет новость в персонализированной ленте.
type PersonalizedNewsItem struct {
	ID          string     `json:"id" db:"id"`
	Topic       string     `json:"topic" db:"topic"`
	Source      string     `json:"source" db:"source"`
	SourceID    string     `json:"sourceId" db:"source_id"`
	URL         string     `json:"url" db:"url"`
	Category    string     `json:"category,omitempty" db:"category"`
	Tags        []string   `json:"tags" db:"tags"`
	PublishedAt *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	ProcessedAt time.Time  `json:"processedAt" db:"processed_at"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	Score       float64    `json:"score" db:"score"`
}

// PersonalizedFeedFilters задает фильтры для выдачи персонализированной ленты.
type PersonalizedFeedFilters struct {
	UserUUID            string
	Limit               int
	Offset              int
	FromHours           int
	PreferredCategories []string
	PreferredSources    []string
	PreferredKeywords   []string
	PreferredLanguage   string
	Query               string
}
