// Package retention реализует периодическую очистку устаревших данных.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
)

// NewsRepository определяет интерфейс удаления устаревших новостей из БД.
type NewsRepository interface {
	DeleteOlderThan(ctx context.Context, threshold time.Time) (int64, error)
}

// DBCleaner периодически удаляет из PostgreSQL новости старше заданного срока.
type DBCleaner struct {
	repo      NewsRepository
	retention config.RetentionConfig
	logger    *slog.Logger
}

// NewDBCleaner создаёт новый DBCleaner.
func NewDBCleaner(repo NewsRepository, retention config.RetentionConfig, logger *slog.Logger) *DBCleaner {
	return &DBCleaner{
		repo:      repo,
		retention: retention,
		logger:    logger,
	}
}

// Run запускает цикл очистки и блокируется до отмены ctx.
func (c *DBCleaner) Run(ctx context.Context) {
	interval := c.retention.GetCleanupInterval()
	retentionDays := c.retention.GetNewsRetentionDays()

	c.logger.Info("db retention cleaner started",
		"retention_days", retentionDays,
		"cleanup_interval", interval,
	)

	// Первый запуск сразу при старте, затем по тикеру.
	c.runOnce(ctx, retentionDays)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("db retention cleaner stopped")
			return
		case <-ticker.C:
			c.runOnce(ctx, retentionDays)
		}
	}
}

func (c *DBCleaner) runOnce(ctx context.Context, retentionDays int) {
	threshold := time.Now().UTC().AddDate(0, 0, -retentionDays)

	deleted, err := c.repo.DeleteOlderThan(ctx, threshold)
	if err != nil {
		c.logger.Error("db retention cleanup failed", "error", err, "threshold", threshold)
		return
	}

	if deleted > 0 {
		c.logger.Info("db retention cleanup done",
			"deleted_news", deleted,
			"threshold", threshold.Format(time.RFC3339),
		)
	} else {
		c.logger.Debug("db retention cleanup: nothing to delete",
			"threshold", threshold.Format(time.RFC3339),
		)
	}
}
