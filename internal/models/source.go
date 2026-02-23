package models

import (
	"database/sql"
	"time"
)

// Source представляет источник новостей (RSS/Atom feed).
type Source struct {
	ID               string         `json:"id"                db:"id"`
	Name             string         `json:"name"              db:"name"`
	URL              string         `json:"url"               db:"url"`
	Language         string         `json:"language"          db:"language"`
	Category         string         `json:"category"          db:"category"`
	FetchInterval    int            `json:"fetch_interval"    db:"fetch_interval"`
	IsActive         bool           `json:"is_active"         db:"is_active"`
	LastCollectedAt  sql.NullTime   `json:"last_collected_at" db:"last_collected_at"`
	LastError        sql.NullString `json:"last_error"        db:"last_error"`
	ErrorCount       int            `json:"error_count"       db:"error_count"`
	CreatedAt        time.Time      `json:"created_at"        db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"        db:"updated_at"`
}

// NextFetchAt вычисляет время следующего запланированного сбора.
func (s *Source) NextFetchAt() time.Time {
	if !s.LastCollectedAt.Valid {
		return time.Now()
	}
	return s.LastCollectedAt.Time.Add(time.Duration(s.FetchInterval) * time.Second)
}

// IsDue проверяет, пора ли собирать новости из этого источника.
func (s *Source) IsDue() bool {
	return s.IsActive && time.Now().After(s.NextFetchAt())
}
