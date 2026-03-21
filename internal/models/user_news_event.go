package models

import "time"

// UserNewsEvent фиксирует факт взаимодействия пользователя с новостью.
type UserNewsEvent struct {
	ID        string         `json:"id" db:"id"`
	UserUUID  string         `json:"userUuid" db:"user_uuid"`
	NewsID    string         `json:"newsId" db:"news_id"`
	EventType string         `json:"eventType" db:"event_type"`
	Metadata  map[string]any `json:"metadata,omitempty" db:"metadata"`
	CreatedAt time.Time      `json:"createdAt" db:"created_at"`
}
