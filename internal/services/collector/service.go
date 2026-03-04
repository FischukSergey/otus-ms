package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/FischukSergey/otus-ms/internal/models"
)

// SourcesClient определяет интерфейс для получения списка источников от main-service.
// Реализуется clients/mainservice.GRPCClient.
type SourcesClient interface {
	GetNewsSources(ctx context.Context) ([]models.Source, error)
}

// StateStore определяет интерфейс хранилища операционного состояния сбора.
// Реализуется store/collector.RedisStateStore.
type StateStore interface {
	IsDue(ctx context.Context, source *models.Source) (bool, error)
	SetCollected(ctx context.Context, sourceID string, t time.Time) error
	IncrementErrorCount(ctx context.Context, sourceID string) (int, error)
	ResetErrorCount(ctx context.Context, sourceID string) error
	IsLocallyDeactivated(ctx context.Context, sourceID string) (bool, error)
	LocallyDeactivate(ctx context.Context, sourceID string) error
}

// ServiceConfig содержит настройки сервиса сбора.
type ServiceConfig struct {
	MaxWorkers  int
	MaxRetries  int
	MaxErrCount int
}

// Service координирует сбор новостей: планирование, парсинг, обновление состояния.
type Service struct {
	client      SourcesClient
	state       StateStore
	parser      *Parser
	logger      *slog.Logger
	maxWorkers  int
	maxRetries  int
	maxErrCount int

	sourcesMu sync.RWMutex
	sources   []models.Source
}

// NewService создаёт новый CollectorService.
func NewService(
	client SourcesClient,
	state StateStore,
	parser *Parser,
	logger *slog.Logger,
	cfg ServiceConfig,
) *Service {
	return &Service{
		client:      client,
		state:       state,
		parser:      parser,
		logger:      logger,
		maxWorkers:  cfg.MaxWorkers,
		maxRetries:  cfg.MaxRetries,
		maxErrCount: cfg.MaxErrCount,
	}
}

// RefreshSources загружает актуальный список источников от main-service по gRPC
// и обновляет внутренний кеш. Вызывается при старте и периодически планировщиком.
func (s *Service) RefreshSources(ctx context.Context) {
	sources, err := s.client.GetNewsSources(ctx)
	if err != nil {
		s.logger.Error("failed to refresh sources from main-service", "error", err)
		return
	}

	s.sourcesMu.Lock()
	s.sources = sources
	s.sourcesMu.Unlock()

	s.logger.Info("sources refreshed", "count", len(sources))
}

// CollectFromDueSources проверяет все кешированные источники и собирает новости
// из тех, у которых подошёл FetchInterval. Запускает параллельный worker pool.
func (s *Service) CollectFromDueSources(ctx context.Context) {
	s.sourcesMu.RLock()
	sources := make([]models.Source, len(s.sources))
	copy(sources, s.sources)
	s.sourcesMu.RUnlock()

	if len(sources) == 0 {
		s.logger.Debug("no sources in cache, skipping collection")
		return
	}

	var due []models.Source
	for i := range sources {
		deactivated, err := s.state.IsLocallyDeactivated(ctx, sources[i].ID)
		if err != nil {
			s.logger.Error("check deactivated failed", "source_id", sources[i].ID, "error", err)
			continue
		}
		if deactivated {
			continue
		}

		isDue, err := s.state.IsDue(ctx, &sources[i])
		if err != nil {
			s.logger.Error("check isDue failed", "source_id", sources[i].ID, "error", err)
			continue
		}
		if isDue {
			due = append(due, sources[i])
		}
	}

	if len(due) == 0 {
		s.logger.Debug("no sources due for collection")
		return
	}

	s.logger.Info("starting collection", "due_count", len(due))

	semaphore := make(chan struct{}, s.maxWorkers)
	var wg sync.WaitGroup

	for i := range due {
		wg.Add(1)
		go func(src models.Source) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			s.collectFromSource(ctx, src)
		}(due[i])
	}

	wg.Wait()
	s.logger.Info("collection round complete", "processed", len(due))
}

// collectFromSource парсит RSS-фид одного источника и обновляет его состояние в Redis.
func (s *Service) collectFromSource(ctx context.Context, source models.Source) {
	start := time.Now()
	s.logger.Info("collecting source", "source_id", source.ID, "name", source.Name, "url", source.URL)

	news, err := s.parser.ParseFeedWithRetry(source.ID, source.URL, s.maxRetries)
	if err != nil {
		s.handleCollectError(ctx, source, err)
		return
	}

	if err := s.state.SetCollected(ctx, source.ID, time.Now()); err != nil {
		s.logger.Error("set collected failed", "source_id", source.ID, "error", err)
	}

	if err := s.state.ResetErrorCount(ctx, source.ID); err != nil {
		s.logger.Error("reset error count failed", "source_id", source.ID, "error", err)
	}

	s.logger.Info("collection success",
		"source_id", source.ID,
		"news_count", len(news),
		"duration", time.Since(start),
	)

	for _, item := range news {
		s.logger.Debug("collected item",
			"source_id", source.ID,
			"title", item.Title,
			"url", item.URL,
			"published_at", item.PublishedAt,
			"author", item.Author,
		)
	}
}

// handleCollectError обрабатывает ошибку сбора: инкрементирует счётчик,
// при достижении лимита локально деактивирует источник.
func (s *Service) handleCollectError(ctx context.Context, source models.Source, err error) {
	s.logger.Error("collection failed", "source_id", source.ID, "name", source.Name, "error", err)

	count, incrErr := s.state.IncrementErrorCount(ctx, source.ID)
	if incrErr != nil {
		s.logger.Error("increment error count failed", "source_id", source.ID, "error", incrErr)
		return
	}

	if count >= s.maxErrCount {
		s.logger.Warn("deactivating source locally after max errors",
			"source_id", source.ID,
			"error_count", count,
			"max_error_count", s.maxErrCount,
		)
		if deactErr := s.state.LocallyDeactivate(ctx, source.ID); deactErr != nil {
			s.logger.Error("local deactivation failed", "source_id", source.ID, "error", deactErr)
		}
	}
}
