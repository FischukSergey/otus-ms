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

		tag, err := r.db.Exec(ctx, query,
			n.ID,
			n.SourceID,
			n.Title,
			n.Summary,
			n.URL,
			n.Category,
			n.Tags,
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
