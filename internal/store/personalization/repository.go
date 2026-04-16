// Package personalization реализует репозиторий персонализации новостной ленты.
package personalization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// ErrPreferencesNotFound возвращается, когда настройки пользователя не найдены.
var ErrPreferencesNotFound = errors.New("preferences not found")

// Repository реализует доступ к данным personalization MVP.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository создает новый экземпляр репозитория personalization.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetPreferences возвращает предпочтения пользователя.
func (r *Repository) GetPreferences(ctx context.Context, userUUID string) (*models.UserNewsPreferences, error) {
	const query = `
		SELECT
			user_uuid,
			preferred_categories,
			preferred_sources,
			preferred_keywords,
			COALESCE(preferred_language, ''),
			from_hours,
			created_at,
			updated_at
		FROM user_news_preferences
		WHERE user_uuid = $1
	`

	var prefs models.UserNewsPreferences
	err := r.db.QueryRow(ctx, query, userUUID).Scan(
		&prefs.UserUUID,
		&prefs.PreferredCategories,
		&prefs.PreferredSources,
		&prefs.PreferredKeywords,
		&prefs.PreferredLanguage,
		&prefs.FromHours,
		&prefs.CreatedAt,
		&prefs.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPreferencesNotFound
		}
		return nil, fmt.Errorf("get preferences by user uuid: %w", err)
	}

	return &prefs, nil
}

// UpsertPreferences создает или обновляет предпочтения пользователя.
func (r *Repository) UpsertPreferences(ctx context.Context, prefs models.UserNewsPreferences) error {
	const query = `
		INSERT INTO user_news_preferences (
			user_uuid,
			preferred_categories,
			preferred_sources,
			preferred_keywords,
			preferred_language,
			from_hours
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		ON CONFLICT (user_uuid) DO UPDATE SET
			preferred_categories = EXCLUDED.preferred_categories,
			preferred_sources = EXCLUDED.preferred_sources,
			preferred_keywords = EXCLUDED.preferred_keywords,
			preferred_language = EXCLUDED.preferred_language,
			from_hours = EXCLUDED.from_hours,
			updated_at = NOW()
	`

	_, err := r.db.Exec(
		ctx,
		query,
		prefs.UserUUID,
		prefs.PreferredCategories,
		prefs.PreferredSources,
		prefs.PreferredKeywords,
		prefs.PreferredLanguage,
		prefs.FromHours,
	)
	if err != nil {
		return fmt.Errorf("upsert preferences: %w", err)
	}

	return nil
}

// InsertEvent сохраняет пользовательское событие взаимодействия с новостью.
func (r *Repository) InsertEvent(ctx context.Context, event models.UserNewsEvent) error {
	const query = `
		INSERT INTO user_news_events (
			id, user_uuid, news_id, event_type, metadata
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	_, err := r.db.Exec(ctx, query, event.ID, event.UserUUID, event.NewsID, event.EventType, metadata)
	if err != nil {
		return fmt.Errorf("insert user event: %w", err)
	}

	return nil
}
