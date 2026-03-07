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
	// LocallyDeactivate временно деактивирует источник на период backoff (circuit-breaker).
	// Автоматически реактивируется по истечении TTL.
	LocallyDeactivate(ctx context.Context, sourceID string, backoff time.Duration) error
	// GetDeactivationCount возвращает число деактиваций для расчёта exponential backoff.
	GetDeactivationCount(ctx context.Context, sourceID string) (int, error)
}

// DedupStore определяет интерфейс дедупликации новостей по нормализованному URL.
// Реализуется store/collector.RedisDedupStore.
type DedupStore interface {
	IsNewURL(ctx context.Context, url string) (bool, error)
}

// ServiceConfig содержит настройки сервиса сбора.
type ServiceConfig struct {
	MaxWorkers  int
	MaxRetries  int
	MaxErrCount int
	// BaseBackoff — начальный период деактивации источника (circuit-breaker).
	// Удваивается с каждой новой деактивацией. По умолчанию 15 минут.
	BaseBackoff time.Duration
	// MaxBackoff — максимальный период деактивации. По умолчанию 24 часа.
	MaxBackoff time.Duration
}

// Service координирует сбор новостей: планирование, парсинг, дедупликацию, обновление состояния.
type Service struct {
	client      SourcesClient
	state       StateStore
	dedup       DedupStore
	parser      *Parser
	logger      *slog.Logger
	maxWorkers  int
	maxRetries  int
	maxErrCount int
	baseBackoff time.Duration
	maxBackoff  time.Duration

	sourcesMu sync.RWMutex
	sources   []models.Source
}

// NewService создаёт новый CollectorService.
func NewService(
	client SourcesClient,
	state StateStore,
	dedup DedupStore,
	parser *Parser,
	logger *slog.Logger,
	cfg ServiceConfig,
) *Service {
	base := cfg.BaseBackoff
	if base <= 0 {
		base = 15 * time.Minute
	}
	maxB := cfg.MaxBackoff
	if maxB <= 0 {
		maxB = 24 * time.Hour
	}
	return &Service{
		client:      client,
		state:       state,
		dedup:       dedup,
		parser:      parser,
		logger:      logger,
		maxWorkers:  cfg.MaxWorkers,
		maxRetries:  cfg.MaxRetries,
		maxErrCount: cfg.MaxErrCount,
		baseBackoff: base,
		maxBackoff:  maxB,
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

// collectFromSource парсит RSS-фид одного источника, дедуплицирует и обновляет состояние в Redis.
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

	fresh := s.filterDuplicates(ctx, news)

	s.logger.Info("collection success",
		"source_id", source.ID,
		"total", len(news),
		"fresh", len(fresh),
		"duplicates", len(news)-len(fresh),
		"duration", time.Since(start),
	)

	for _, item := range fresh {
		s.logger.Debug("collected item",
			"source_id", source.ID,
			"title", item.Title,
			"url", item.URL,
			"published_at", item.PublishedAt,
			"author", item.Author,
		)
	}
}

// filterDuplicates отсеивает новости с уже виденными URL.
// URL нормализуется перед проверкой. При ошибке дедупа конкретная новость включается (fail open).
func (s *Service) filterDuplicates(ctx context.Context, news []*models.RawNews) []*models.RawNews {
	fresh := make([]*models.RawNews, 0, len(news))
	for _, item := range news {
		if item.URL == "" {
			fresh = append(fresh, item)
			continue
		}
		normalized := NormalizeURL(item.URL)
		isNew, err := s.dedup.IsNewURL(ctx, normalized)
		if err != nil {
			s.logger.Warn("dedup check failed, treating as new", "url", item.URL, "error", err)
			fresh = append(fresh, item)
			continue
		}
		if isNew {
			fresh = append(fresh, item)
		}
	}
	return fresh
}

// handleCollectError обрабатывает ошибку сбора: инкрементирует счётчик,
// при достижении лимита временно деактивирует источник с exponential backoff (circuit-breaker).
// После deactivation_count деактиваций backoff = baseBackoff * 2^count, но не более maxBackoff.
func (s *Service) handleCollectError(ctx context.Context, source models.Source, err error) {
	s.logger.Error("collection failed", "source_id", source.ID, "name", source.Name, "error", err)

	count, incrErr := s.state.IncrementErrorCount(ctx, source.ID)
	if incrErr != nil {
		s.logger.Error("increment error count failed", "source_id", source.ID, "error", incrErr)
		return
	}

	if count < s.maxErrCount {
		return
	}

	deactCount, dcErr := s.state.GetDeactivationCount(ctx, source.ID)
	if dcErr != nil {
		s.logger.Error("get deactivation count failed", "source_id", source.ID, "error", dcErr)
		deactCount = 0
	}

	backoff := calcBackoff(deactCount, s.baseBackoff, s.maxBackoff)

	s.logger.Warn("circuit-breaker: temporarily deactivating source",
		"source_id", source.ID,
		"error_count", count,
		"deactivation_count", deactCount,
		"backoff", backoff,
	)

	if deactErr := s.state.LocallyDeactivate(ctx, source.ID, backoff); deactErr != nil {
		s.logger.Error("local deactivation failed", "source_id", source.ID, "error", deactErr)
		return
	}
	if resetErr := s.state.ResetErrorCount(ctx, source.ID); resetErr != nil {
		s.logger.Error("reset error count after deactivation failed", "source_id", source.ID, "error", resetErr)
	}
}

// calcBackoff вычисляет период деактивации: baseBackoff * 2^count, но не более maxBackoff.
// count — текущее число деактиваций до новой (начиная с 0).
func calcBackoff(count int, base, maxBackoff time.Duration) time.Duration {
	shift := min(count, 10) // предотвращаем overflow при больших значениях
	backoff := base * (1 << shift)
	return min(backoff, maxBackoff)
}
