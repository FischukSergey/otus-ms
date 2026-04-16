package personalization

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// FeedQueryRepository реализует query-операции для персонализированной ленты.
type FeedQueryRepository struct {
	db *pgxpool.Pool
}

// NewFeedQueryRepository создает новый query-репозиторий персонализированной ленты.
func NewFeedQueryRepository(db *pgxpool.Pool) *FeedQueryRepository {
	return &FeedQueryRepository{db: db}
}

// GetPersonalizedFeed возвращает персонализированную ленту с MVP-scoring.
func (r *FeedQueryRepository) GetPersonalizedFeed(
	ctx context.Context,
	filters models.PersonalizedFeedFilters,
) ([]models.PersonalizedNewsItem, error) {
	const query = `
		WITH user_events AS (
			SELECT
				news_id,
				SUM(
					CASE event_type
						WHEN 'like' THEN 1.5
						WHEN 'dislike' THEN -1.5
						WHEN 'click' THEN 0.3
						WHEN 'view' THEN 0.1
						ELSE 0
					END
				)::DOUBLE PRECISION AS event_boost,
				BOOL_OR(event_type = 'hide') AS is_hidden
			FROM user_news_events
			WHERE user_uuid = $1
			GROUP BY news_id
		),
		input_query AS (
			SELECT CASE
				WHEN $9 = '' THEN NULL
				ELSE websearch_to_tsquery('russian', $9)
			END AS ts_query
		)
		SELECT
			n.id,
			n.title AS topic,
			COALESCE(ns.name, n.source_id) AS source,
			n.source_id,
			n.url,
			COALESCE(n.category, '') AS category,
			n.tags,
			n.published_at,
			n.processed_at,
			n.created_at,
			(
				GREATEST(
					0.0,
					2.0 - EXTRACT(EPOCH FROM (NOW() - n.processed_at)) / 86400.0
				)
				+ CASE
					WHEN array_length($5::text[], 1) IS NOT NULL AND n.category = ANY($5::text[]) THEN 1.0
					ELSE 0.0
				END
				+ CASE
					WHEN array_length($6::text[], 1) IS NOT NULL AND n.source_id = ANY($6::text[]) THEN 0.8
					ELSE 0.0
				END
				+ CASE
					WHEN array_length($8::text[], 1) IS NULL THEN 0.0
					ELSE LEAST(
						2.0,
						COALESCE(
							ts_rank(
								nsi.search_vector,
								websearch_to_tsquery('russian', array_to_string($8::text[], ' '))
							),
							0.0
						) * 0.6
					)
				END
				+ CASE
					WHEN iq.ts_query IS NULL THEN 0.0
					ELSE COALESCE(ts_rank(nsi.search_vector, iq.ts_query), 0.0)
				END
				+ COALESCE(ue.event_boost, 0.0)
			) AS score
		FROM news n
		LEFT JOIN news_sources ns ON ns.id = n.source_id
		LEFT JOIN user_events ue ON ue.news_id = n.id
		LEFT JOIN news_search_index nsi ON nsi.news_id = n.id
		CROSS JOIN input_query iq
		WHERE
			n.processed_at >= NOW() - make_interval(hours => $4)
			AND (array_length($5::text[], 1) IS NULL OR n.category = ANY($5::text[]))
			AND (array_length($6::text[], 1) IS NULL OR n.source_id = ANY($6::text[]))
			AND ($7 = '' OR ns.language = $7)
			AND (iq.ts_query IS NULL OR nsi.search_vector @@ iq.ts_query)
			AND COALESCE(ue.is_hidden, FALSE) = FALSE
		ORDER BY score DESC, n.processed_at DESC, n.created_at DESC
		LIMIT $2 OFFSET $3
	`

	normalized := normalizeFeedFilters(filters)
	rows, err := r.db.Query(
		ctx,
		query,
		normalized.UserUUID,
		normalized.Limit,
		normalized.Offset,
		normalized.FromHours,
		normalized.PreferredCategories,
		normalized.PreferredSources,
		normalized.PreferredLanguage,
		normalized.PreferredKeywords,
		normalized.Query,
	)
	if err != nil {
		return nil, fmt.Errorf("query personalized feed: %w", err)
	}
	defer rows.Close()

	result := make([]models.PersonalizedNewsItem, 0, normalized.Limit)
	for rows.Next() {
		var (
			item      models.PersonalizedNewsItem
			rawTags   []byte
			published *time.Time
		)

		if err := rows.Scan(
			&item.ID,
			&item.Topic,
			&item.Source,
			&item.SourceID,
			&item.URL,
			&item.Category,
			&rawTags,
			&published,
			&item.ProcessedAt,
			&item.CreatedAt,
			&item.Score,
		); err != nil {
			return nil, fmt.Errorf("scan personalized feed row: %w", err)
		}

		if published != nil {
			value := *published
			item.PublishedAt = &value
		}
		if len(rawTags) > 0 {
			if err := json.Unmarshal(rawTags, &item.Tags); err != nil {
				return nil, fmt.Errorf("unmarshal tags for news_id=%s: %w", item.ID, err)
			}
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personalized feed rows: %w", err)
	}

	return result, nil
}

func normalizeFeedFilters(filters models.PersonalizedFeedFilters) models.PersonalizedFeedFilters {
	out := models.PersonalizedFeedFilters{
		UserUUID:            strings.Clone(filters.UserUUID),
		Limit:               filters.Limit,
		Offset:              filters.Offset,
		FromHours:           filters.FromHours,
		PreferredCategories: slices.Clone(filters.PreferredCategories),
		PreferredSources:    slices.Clone(filters.PreferredSources),
		PreferredKeywords:   slices.Clone(filters.PreferredKeywords),
		PreferredLanguage:   strings.Clone(filters.PreferredLanguage),
		Query:               strings.TrimSpace(filters.Query),
	}

	if out.Limit <= 0 {
		out.Limit = 50
	}
	if out.Limit > 100 {
		out.Limit = 100
	}
	if out.Offset < 0 {
		out.Offset = 0
	}
	if out.FromHours <= 0 {
		out.FromHours = 168
	}

	return out
}
