// Package alerting реализует репозиторий для alerting API.
package alerting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// Repository реализует доступ к таблицам alert_rules и alert_events.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository создает новый экземпляр репозитория alerting.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ListRules возвращает все правила пользователя.
func (r *Repository) ListRules(ctx context.Context, userUUID string) ([]models.AlertRule, error) {
	const query = `
		SELECT
			id,
			user_uuid,
			keyword,
			is_active,
			channel_type,
			COALESCE(channel_target, ''),
			cooldown_seconds,
			created_at,
			updated_at
		FROM alert_rules
		WHERE user_uuid = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userUUID)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	defer rows.Close()

	result := make([]models.AlertRule, 0)
	for rows.Next() {
		var item models.AlertRule
		if err := rows.Scan(
			&item.ID,
			&item.UserUUID,
			&item.Keyword,
			&item.IsActive,
			&item.ChannelType,
			&item.ChannelTarget,
			&item.CooldownSeconds,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rule rows: %w", err)
	}

	return result, nil
}

// CountActiveRules возвращает количество активных правил пользователя.
func (r *Repository) CountActiveRules(ctx context.Context, userUUID string) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM alert_rules
		WHERE user_uuid = $1 AND is_active = TRUE
	`

	var count int
	if err := r.db.QueryRow(ctx, query, userUUID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active alert rules: %w", err)
	}

	return count, nil
}

// CreateRule создает новое правило.
func (r *Repository) CreateRule(ctx context.Context, rule models.AlertRule) error {
	const query = `
		INSERT INTO alert_rules (
			id,
			user_uuid,
			keyword,
			is_active,
			channel_type,
			channel_target,
			cooldown_seconds
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		rule.ID,
		rule.UserUUID,
		rule.Keyword,
		rule.IsActive,
		rule.ChannelType,
		rule.ChannelTarget,
		rule.CooldownSeconds,
	)
	if err != nil {
		return fmt.Errorf("create alert rule: %w", err)
	}

	return nil
}

// UpdateRule обновляет существующее правило пользователя.
func (r *Repository) UpdateRule(ctx context.Context, rule models.AlertRule) (bool, error) {
	const query = `
		UPDATE alert_rules
		SET
			keyword = $1,
			is_active = $2,
			channel_type = $3,
			channel_target = NULLIF($4, ''),
			cooldown_seconds = $5,
			updated_at = NOW()
		WHERE id = $6 AND user_uuid = $7
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		rule.Keyword,
		rule.IsActive,
		rule.ChannelType,
		rule.ChannelTarget,
		rule.CooldownSeconds,
		rule.ID,
		rule.UserUUID,
	)
	if err != nil {
		return false, fmt.Errorf("update alert rule: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// DeleteRule удаляет правило пользователя.
func (r *Repository) DeleteRule(ctx context.Context, userUUID, ruleID string) (bool, error) {
	const query = `DELETE FROM alert_rules WHERE id = $1 AND user_uuid = $2`
	tag, err := r.db.Exec(ctx, query, ruleID, userUUID)
	if err != nil {
		return false, fmt.Errorf("delete alert rule: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// ListEvents возвращает историю событий пользователя с пагинацией.
func (r *Repository) ListEvents(
	ctx context.Context,
	userUUID string,
	limit, offset int,
	status string,
) ([]models.AlertEvent, error) {
	baseQuery := `
		SELECT
			id,
			rule_id,
			news_id,
			user_uuid,
			keyword,
			delivery_status,
			COALESCE(error_message, ''),
			sent_at,
			created_at
		FROM alert_events
		WHERE user_uuid = $1
	`

	args := []any{userUUID, limit, offset}
	query := baseQuery
	if status != "" {
		query += " AND delivery_status = $2"
		args = []any{userUUID, status, limit, offset}
		query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
	} else {
		query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alert events: %w", err)
	}
	defer rows.Close()

	result := make([]models.AlertEvent, 0, limit)
	for rows.Next() {
		var item models.AlertEvent
		if err := rows.Scan(
			&item.ID,
			&item.RuleID,
			&item.NewsID,
			&item.UserUUID,
			&item.Keyword,
			&item.DeliveryStatus,
			&item.ErrorMessage,
			&item.SentAt,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert event row: %w", err)
		}
		item.DeliveryStatus = strings.ToLower(item.DeliveryStatus)
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert event rows: %w", err)
	}

	return result, nil
}

// ListActiveRules возвращает активные правила всех пользователей для news-processor.
func (r *Repository) ListActiveRules(ctx context.Context) ([]models.AlertRule, error) {
	const query = `
		SELECT
			id,
			user_uuid,
			keyword,
			is_active,
			channel_type,
			COALESCE(channel_target, ''),
			cooldown_seconds,
			created_at,
			updated_at
		FROM alert_rules
		WHERE is_active = TRUE
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query active alert rules: %w", err)
	}
	defer rows.Close()

	result := make([]models.AlertRule, 0)
	for rows.Next() {
		var item models.AlertRule
		if err := rows.Scan(
			&item.ID,
			&item.UserUUID,
			&item.Keyword,
			&item.IsActive,
			&item.ChannelType,
			&item.ChannelTarget,
			&item.CooldownSeconds,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active alert rule row: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active alert rule rows: %w", err)
	}

	return result, nil
}

// CreatePendingEvent вставляет запись события со статусом pending.
// Возвращает false, если это дубликат по (rule_id, news_id).
func (r *Repository) CreatePendingEvent(ctx context.Context, event models.NewsAlertEvent) (bool, error) {
	const query = `
		INSERT INTO alert_events (
			id, rule_id, news_id, user_uuid, keyword, delivery_status
		)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		ON CONFLICT (rule_id, news_id) DO NOTHING
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		event.EventID,
		event.RuleID,
		event.NewsID,
		event.UserUUID,
		event.Keyword,
	)
	if err != nil {
		return false, fmt.Errorf("insert pending alert event: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// GetRuleCooldownSeconds возвращает cooldown_seconds для правила.
func (r *Repository) GetRuleCooldownSeconds(ctx context.Context, ruleID string) (int, error) {
	const query = `SELECT cooldown_seconds FROM alert_rules WHERE id = $1`

	var cooldown int
	if err := r.db.QueryRow(ctx, query, ruleID).Scan(&cooldown); err != nil {
		return 0, fmt.Errorf("query cooldown for rule_id=%s: %w", ruleID, err)
	}

	return cooldown, nil
}

// GetLastSentAt возвращает время последней успешной отправки по правилу.
func (r *Repository) GetLastSentAt(ctx context.Context, ruleID string) (*time.Time, error) {
	const query = `
		SELECT sent_at
		FROM alert_events
		WHERE rule_id = $1 AND delivery_status = 'sent' AND sent_at IS NOT NULL
		ORDER BY sent_at DESC
		LIMIT 1
	`

	var sentAt time.Time
	if err := r.db.QueryRow(ctx, query, ruleID).Scan(&sentAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query last sent_at for rule_id=%s: %w", ruleID, err)
	}

	return &sentAt, nil
}

// MarkDropped помечает событие как dropped.
func (r *Repository) MarkDropped(ctx context.Context, eventID, reason string) error {
	const query = `
		UPDATE alert_events
		SET delivery_status = 'dropped', error_message = $1
		WHERE id = $2
	`

	if _, err := r.db.Exec(ctx, query, reason, eventID); err != nil {
		return fmt.Errorf("mark alert event dropped: %w", err)
	}

	return nil
}

// MarkSent помечает событие как sent.
func (r *Repository) MarkSent(ctx context.Context, eventID string, sentAt time.Time) error {
	const query = `
		UPDATE alert_events
		SET delivery_status = 'sent', sent_at = $1, error_message = NULL
		WHERE id = $2
	`

	if _, err := r.db.Exec(ctx, query, sentAt, eventID); err != nil {
		return fmt.Errorf("mark alert event sent: %w", err)
	}

	return nil
}

// MarkFailed помечает событие как failed.
func (r *Repository) MarkFailed(ctx context.Context, eventID, errMsg string) error {
	const query = `
		UPDATE alert_events
		SET delivery_status = 'failed', error_message = $1
		WHERE id = $2
	`

	if _, err := r.db.Exec(ctx, query, errMsg, eventID); err != nil {
		return fmt.Errorf("mark alert event failed: %w", err)
	}

	return nil
}
