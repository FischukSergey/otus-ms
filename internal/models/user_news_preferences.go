package models

import "time"

// UserNewsPreferences хранит пользовательские настройки персонализации.
type UserNewsPreferences struct {
	UserUUID            string    `json:"userUuid" db:"user_uuid"`
	PreferredCategories []string  `json:"preferredCategories" db:"preferred_categories"`
	PreferredSources    []string  `json:"preferredSources" db:"preferred_sources"`
	PreferredKeywords   []string  `json:"preferredKeywords" db:"preferred_keywords"`
	PreferredLanguage   string    `json:"preferredLanguage,omitempty" db:"preferred_language"`
	FromHours           int       `json:"fromHours" db:"from_hours"`
	CreatedAt           time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt           time.Time `json:"updatedAt" db:"updated_at"`
}
