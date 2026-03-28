package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/FischukSergey/otus-ms/internal/config"
)

// ObjectStore определяет интерфейс удаления устаревших объектов из S3.
type ObjectStore interface {
	ListAndDeleteOld(ctx context.Context, prefix string, olderThan time.Time) (int, error)
}

// S3Cleaner периодически удаляет из S3-бакета объекты старше заданного срока.
type S3Cleaner struct {
	store     ObjectStore
	prefix    string
	retention config.RetentionConfig
	logger    *slog.Logger
}

// NewS3Cleaner создаёт новый S3Cleaner.
func NewS3Cleaner(store ObjectStore, prefix string, retention config.RetentionConfig, logger *slog.Logger) *S3Cleaner {
	return &S3Cleaner{
		store:     store,
		prefix:    prefix,
		retention: retention,
		logger:    logger,
	}
}

// Run запускает цикл очистки и блокируется до отмены ctx.
func (c *S3Cleaner) Run(ctx context.Context) {
	interval := c.retention.GetCleanupInterval()
	retentionDays := c.retention.GetNewsRetentionDays()

	c.logger.Info("s3 retention cleaner started",
		"prefix", c.prefix,
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
			c.logger.Info("s3 retention cleaner stopped")
			return
		case <-ticker.C:
			c.runOnce(ctx, retentionDays)
		}
	}
}

func (c *S3Cleaner) runOnce(ctx context.Context, retentionDays int) {
	olderThan := time.Now().UTC().AddDate(0, 0, -retentionDays)

	deleted, err := c.store.ListAndDeleteOld(ctx, c.prefix, olderThan)
	if err != nil {
		c.logger.Error("s3 retention cleanup failed", "error", err, "prefix", c.prefix, "older_than", olderThan)
		return
	}

	if deleted > 0 {
		c.logger.Info("s3 retention cleanup done",
			"deleted_objects", deleted,
			"prefix", c.prefix,
			"older_than", olderThan.Format(time.RFC3339),
		)
	} else {
		c.logger.Debug("s3 retention cleanup: nothing to delete",
			"prefix", c.prefix,
			"older_than", olderThan.Format(time.RFC3339),
		)
	}
}
