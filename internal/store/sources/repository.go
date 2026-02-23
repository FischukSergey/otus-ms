package sources

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// Repository реализует доступ к таблице news_sources.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository создаёт новый репозиторий источников.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetAll возвращает все источники новостей из БД.
func (r *Repository) GetAll(ctx context.Context) ([]models.Source, error) {
	const query = `
		SELECT
			id, name, url, language, category,
			fetch_interval, is_active,
			last_collected_at, last_error, error_count,
			created_at, updated_at
		FROM news_sources
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query news_sources: %w", err)
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		var s models.Source
		err := rows.Scan(
			&s.ID, &s.Name, &s.URL, &s.Language, &s.Category,
			&s.FetchInterval, &s.IsActive,
			&s.LastCollectedAt, &s.LastError, &s.ErrorCount,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan news_source row: %w", err)
		}
		sources = append(sources, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news_sources rows: %w", err)
	}

	return sources, nil
}

// GetActive возвращает только активные источники новостей.
func (r *Repository) GetActive(ctx context.Context) ([]models.Source, error) {
	const query = `
		SELECT
			id, name, url, language, category,
			fetch_interval, is_active,
			last_collected_at, last_error, error_count,
			created_at, updated_at
		FROM news_sources
		WHERE is_active = TRUE
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query active news_sources: %w", err)
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		var s models.Source
		err := rows.Scan(
			&s.ID, &s.Name, &s.URL, &s.Language, &s.Category,
			&s.FetchInterval, &s.IsActive,
			&s.LastCollectedAt, &s.LastError, &s.ErrorCount,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan news_source row: %w", err)
		}
		sources = append(sources, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news_sources rows: %w", err)
	}

	return sources, nil
}
