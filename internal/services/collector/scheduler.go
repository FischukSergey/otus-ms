package collector

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// Scheduler периодически запускает сбор новостей и обновление списка источников.
type Scheduler struct {
	cron    *cron.Cron
	service *Service
	logger  *slog.Logger
}

// NewScheduler создаёт Scheduler с поддержкой секундного разрешения cron-выражений.
func NewScheduler(service *Service, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		service: service,
		logger:  logger,
	}
}

// Start регистрирует задачи и запускает планировщик.
// Задачи выполняются в горутинах — не блокируют друг друга.
//
// Расписание:
//   - каждую минуту  → CollectFromDueSources (сбор источников с истёкшим FetchInterval)
//   - каждые 5 минут → RefreshSources (обновление списка источников от main-service)
func (s *Scheduler) Start(ctx context.Context) error {
	if _, err := s.cron.AddFunc("0 * * * * *", func() {
		s.logger.Debug("scheduler: running CollectFromDueSources")
		s.service.CollectFromDueSources(ctx)
	}); err != nil {
		return err
	}

	if _, err := s.cron.AddFunc("0 */5 * * * *", func() {
		s.logger.Debug("scheduler: running RefreshSources")
		s.service.RefreshSources(ctx)
	}); err != nil {
		return err
	}

	s.cron.Start()
	s.logger.Info("scheduler started",
		"collect_schedule", "every minute",
		"refresh_schedule", "every 5 minutes",
	)
	return nil
}

// Stop останавливает планировщик и ждёт завершения текущих задач.
func (s *Scheduler) Stop() {
	s.logger.Info("scheduler stopping...")
	s.cron.Stop()
	s.logger.Info("scheduler stopped")
}
