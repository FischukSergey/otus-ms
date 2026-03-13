// Package news реализует репозиторий для работы с обработанными новостями в PostgreSQL.
package news

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// Repository реализует доступ к таблице news.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository создаёт новый репозиторий новостей.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListLatest возвращает последние новости для UI.
func (r *Repository) ListLatest(ctx context.Context, limit int) ([]models.NewsBrief, error) {
	const query = `
		SELECT
			n.title AS topic,
			COALESCE(ns.name, n.source_id) AS source,
			n.url,
			n.created_at
		FROM news n
		LEFT JOIN news_sources ns ON ns.id = n.source_id
		ORDER BY n.processed_at DESC, n.created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest news: %w", err)
	}
	defer rows.Close()

	result := make([]models.NewsBrief, 0, limit)
	for rows.Next() {
		var item models.NewsBrief
		if err := rows.Scan(
			&item.Topic,
			&item.Source,
			&item.URL,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan latest news row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest news rows: %w", err)
	}

	return result, nil
}

// UpsertBatch сохраняет пачку обработанных новостей.
// Дублирующиеся URL игнорируются (ON CONFLICT DO NOTHING).
// Возвращает количество реально вставленных записей.
func (r *Repository) UpsertBatch(ctx context.Context, news []models.ProcessedNews) (int, error) {
	if len(news) == 0 {
		return 0, nil
	}

	const query = `
		INSERT INTO news (id, source_id, title, summary, url, category, tags, published_at, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (url) DO NOTHING
	`

	var saved int
	for i := range news {
		n := &news[i]

		// Гарантируем непустой слайс: pgx передаёт nil как SQL NULL,
		// что нарушает NOT NULL DEFAULT '[]' на колонке tags (JSONB).
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}

		tag, err := r.db.Exec(ctx, query,
			n.ID,
			n.SourceID,
			n.Title,
			n.Summary,
			n.URL,
			n.Category,
			tags,
			n.PublishedAt,
			n.ProcessedAt,
		)
		if err != nil {
			return saved, fmt.Errorf("insert news id=%s url=%s: %w", n.ID, n.URL, err)
		}
		saved += int(tag.RowsAffected())
	}

	return saved, nil
}
