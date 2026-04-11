package processor

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/models"
	"github.com/FischukSergey/otus-ms/internal/objectstore"
)

const (
	commitRetryAttempts       = 3
	commitInitialRetryBackoff = 100 * time.Millisecond
	commitMaxRetryBackoff     = 800 * time.Millisecond
)

type reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
	Stats() kafka.ReaderStats
}

type rawTask struct {
	msg kafka.Message
	raw *models.RawNews
}

// startOffsetLabel возвращает строковое представление StartOffset для логов.
func startOffsetLabel(o int64) string {
	switch o {
	case kafka.FirstOffset:
		return "earliest"
	case kafka.LastOffset:
		return "latest"
	default:
		return "earliest"
	}
}

// NewsClient определяет интерфейс для сохранения обработанных новостей через gRPC.
// Реализуется clients/mainservice.GRPCClient.
type NewsClient interface {
	SaveProcessedNews(ctx context.Context, news []models.ProcessedNews) (int, error)
	GetActiveAlertRules(ctx context.Context) ([]models.AlertRule, error)
}

// ArtifactStore определяет интерфейс сохранения текстового артефакта новости.
type ArtifactStore interface {
	PutText(ctx context.Context, key string, content string) (string, error)
}

// Service читает сырые новости из Kafka, обрабатывает через конвейер
// и батчами сохраняет через gRPC в main-service.
type Service struct {
	reader         reader
	alertWriter    *kafka.Writer
	alertTopic     string
	newsClient     NewsClient
	artifactStore  ArtifactStore
	objectStoreCfg config.ObjectStorageConfig
	processorCfg   config.ProcessorConfig
	rulesCache     *cachedAlertRules
	logger         *slog.Logger
}

// NewService создаёт сервис обработки новостей.
func NewService(
	kafkaCfg config.KafkaConfig,
	objectStoreCfg config.ObjectStorageConfig,
	processorCfg config.ProcessorConfig,
	newsClient NewsClient,
	artifactStore ArtifactStore,
	logger *slog.Logger,
) *Service {
	// FirstOffset: для нового consumer group читаем с начала топика,
	// чтобы не пропустить сообщения, накопившиеся пока сервис не был запущен.
	// После первого коммита Kafka запомнит позицию и перезапуск продолжит с неё.
	startOffset := kafka.FirstOffset

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     kafkaCfg.Brokers,
		Topic:       kafkaCfg.TopicRawNews,
		GroupID:     kafkaCfg.ConsumerGroup,
		MinBytes:    1,    // 1 B — не ждём накопления батча при чтении
		MaxBytes:    10e6, // 10 MB
		MaxWait:     500 * time.Millisecond,
		StartOffset: startOffset,
	})

	var alertWriter *kafka.Writer
	if kafkaCfg.TopicNewsAlerts != "" {
		alertWriter = &kafka.Writer{
			Addr:                   kafka.TCP(kafkaCfg.Brokers...),
			Topic:                  kafkaCfg.TopicNewsAlerts,
			Balancer:               &kafka.Hash{},
			BatchSize:              kafkaCfg.GetBatchSize(),
			BatchTimeout:           kafkaCfg.GetBatchTimeout(),
			WriteTimeout:           kafkaCfg.GetWriteTimeout(),
			RequiredAcks:           kafka.RequireOne,
			AllowAutoTopicCreation: false,
		}
	}

	logger.Info("processor service created",
		"brokers", kafkaCfg.Brokers,
		"topic", kafkaCfg.TopicRawNews,
		"consumer_group", kafkaCfg.ConsumerGroup,
		"start_offset", startOffsetLabel(startOffset),
		"workers", processorCfg.GetWorkers(),
		"alerts_topic", kafkaCfg.TopicNewsAlerts,
		"artifact_bucket", objectStoreCfg.Bucket,
		"artifact_endpoint", objectStoreCfg.Endpoint,
		"artifact_prefix", objectStoreCfg.Prefix,
	)

	return &Service{
		reader:         reader,
		alertWriter:    alertWriter,
		alertTopic:     kafkaCfg.TopicNewsAlerts,
		newsClient:     newsClient,
		artifactStore:  artifactStore,
		objectStoreCfg: objectStoreCfg,
		processorCfg:   processorCfg,
		rulesCache:     &cachedAlertRules{},
		logger:         logger,
	}
}

// Run запускает конвейер обработки: читает из Kafka, параллельно обрабатывает,
// батчами сохраняет в main-service. Блокируется до отмены ctx.
func (s *Service) Run(ctx context.Context) error {
	workers := s.processorCfg.GetWorkers()
	saveBatchSize := s.processorCfg.GetSaveBatchSize()

	s.logger.Info("processor starting",
		"workers", workers,
		"save_batch_size", saveBatchSize,
		"fetch_content", s.processorCfg.FetchContent,
		"alerts_enabled", s.alertWriter != nil,
	)

	if s.alertWriter != nil {
		go s.alertRulesRefresher(ctx, 45*time.Second)
	}

	// Буферизованные каналы: задачи и результаты.
	tasks := make(chan rawTask, workers*2)
	results := make(chan *models.ProcessedNews, workers*2)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.workerLoop(ctx, tasks, results)
		}()
	}

	// Горутина-батчер сохраняет результаты через gRPC.
	saveDone := make(chan struct{})
	go func() {
		defer close(saveDone)
		s.saveBatcher(ctx, results, saveBatchSize)
	}()

	// Основной цикл чтения из Kafka (блокирует до ctx.Done).
	s.readLoop(ctx, tasks)

	close(tasks)
	wg.Wait()
	close(results)
	<-saveDone

	if err := s.reader.Close(); err != nil {
		s.logger.Error("kafka reader close error", "error", err)
		return err
	}
	if s.alertWriter != nil {
		if err := s.alertWriter.Close(); err != nil {
			s.logger.Error("kafka alert writer close error", "error", err)
			return err
		}
	}
	return nil
}

// readLoop читает сообщения из Kafka и отправляет десериализованные RawNews в tasks.
func (s *Service) readLoop(ctx context.Context, tasks chan<- rawTask) {
	s.logger.Debug("kafka readLoop started, waiting for messages...")

	var received int64
	logTicker := time.NewTicker(10 * time.Second)
	defer logTicker.Stop()

	for {
		// Показываем статус каждые 10 сек если сообщений нет.
		select {
		case <-logTicker.C:
			stats := s.reader.Stats()
			s.logger.Debug("kafka reader stats",
				"messages_received_total", received,
				"lag", stats.Lag,
				"offset", stats.Offset,
				"errors", stats.Errors,
				"fetches", stats.Fetches,
			)
		default:
		}

		msg, err := s.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				s.logger.Info("kafka readLoop stopped by context", "total_received", received)
				return
			}
			s.logger.Error("kafka read error", "error", err)
			continue
		}

		received++
		s.logger.Debug("kafka message received",
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"value_bytes", len(msg.Value),
			"total_received", received,
		)

		var raw models.RawNews
		if err := json.Unmarshal(msg.Value, &raw); err != nil {
			s.logger.Warn("failed to unmarshal raw news, skipping",
				"partition", msg.Partition, "offset", msg.Offset, "error", err)
			if commitErr := s.commitWithRetry(ctx, msg); commitErr != nil {
				s.logger.Error("failed to commit invalid raw news message",
					"partition", msg.Partition,
					"offset", msg.Offset,
					"error", commitErr,
				)
			}
			continue
		}

		select {
		case tasks <- rawTask{
			msg: msg,
			raw: &raw,
		}:
		case <-ctx.Done():
			return
		}
	}
}

// workerLoop получает задачи из tasks, запускает конвейер обработки и отправляет результаты.
func (s *Service) workerLoop(ctx context.Context, tasks <-chan rawTask, results chan<- *models.ProcessedNews) {
	for task := range tasks {
		raw := task.raw
		s.logger.Debug("processing news item",
			"id", raw.ID,
			"source_id", raw.SourceID,
			"url", raw.URL,
			"has_content", raw.Content != "",
		)

		processed := Process(ctx, raw,
			s.processorCfg.FetchContent,
			s.processorCfg.GetFetchTimeout(),
		)

		if processed.Content != "" {
			key := objectstore.BuildNewsTextKey(
				s.objectStoreCfg.Prefix,
				processed.ID,
				processed.ProcessedAt,
			)
			s3Key, err := s.artifactStore.PutText(ctx, key, processed.Content)
			if err != nil {
				s.logger.Error("failed to upload news artifact to object storage",
					"news_id", processed.ID,
					"bucket", s.objectStoreCfg.Bucket,
					"endpoint", s.objectStoreCfg.Endpoint,
					"error", err,
				)
				continue
			}
			processed.S3Key = s3Key
		} else {
			s.logger.Debug("skip artifact upload: processed content is empty",
				"news_id", processed.ID,
			)
		}

		s.logger.Debug("news item processed",
			"id", processed.ID,
			"category", processed.Category,
			"summary_len", len(processed.Summary),
			"s3_key", processed.S3Key,
		)

		if s.alertWriter != nil {
			events := matchAlertEvents(processed, s.rulesCache.get())
			if len(events) > 0 {
				if err := s.publishAlertEvents(ctx, events); err != nil {
					s.logger.Error("failed to publish alert events",
						"news_id", processed.ID,
						"events_count", len(events),
						"error", err,
					)
				} else {
					s.logger.Debug("alert events published",
						"news_id", processed.ID,
						"events_count", len(events),
						"topic", s.alertTopic,
					)
				}
			}
		}

		select {
		case results <- processed:
			if err := s.commitWithRetry(ctx, task.msg); err != nil {
				s.logger.Error("failed to commit processed kafka message",
					"id", processed.ID,
					"source_id", processed.SourceID,
					"partition", task.msg.Partition,
					"offset", task.msg.Offset,
					"error", err,
				)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) commitWithRetry(ctx context.Context, msg kafka.Message) error {
	backoff := commitInitialRetryBackoff
	var lastErr error

	for attempt := 1; attempt <= commitRetryAttempts; attempt++ {
		err := s.reader.CommitMessages(ctx, msg)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == commitRetryAttempts {
			break
		}

		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, commitMaxRetryBackoff)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

func (s *Service) alertRulesRefresher(ctx context.Context, interval time.Duration) {
	refresh := func() {
		rules, err := s.newsClient.GetActiveAlertRules(ctx)
		if err != nil {
			s.logger.Warn("failed to refresh alert rules", "error", err)
			return
		}
		s.rulesCache.set(rules)
		s.logger.Debug("alert rules cache refreshed", "rules_count", len(rules))
	}

	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			refresh()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) publishAlertEvents(ctx context.Context, events []models.NewsAlertEvent) error {
	messages := make([]kafka.Message, 0, len(events))
	for i := range events {
		value, err := json.Marshal(events[i])
		if err != nil {
			s.logger.Warn("failed to marshal alert event",
				"event_id", events[i].EventID,
				"rule_id", events[i].RuleID,
				"error", err,
			)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(events[i].RuleID),
			Value: value,
		})
	}

	if len(messages) == 0 {
		return nil
	}

	return s.alertWriter.WriteMessages(ctx, messages...)
}

// saveBatcher собирает ProcessedNews в батчи и отправляет через gRPC.
// Принудительный сброс батча происходит по таймеру (каждые 5 с) или при закрытии канала.
func (s *Service) saveBatcher(ctx context.Context, results <-chan *models.ProcessedNews, batchSize int) {
	batch := make([]models.ProcessedNews, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		saved, err := s.newsClient.SaveProcessedNews(ctx, batch)
		if err != nil {
			s.logger.Error("failed to save processed news batch",
				"batch_size", len(batch), "error", err)
		} else {
			s.logger.Info("processed news batch saved",
				"sent", len(batch), "saved", saved)
		}
		batch = batch[:0]
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case item, ok := <-results:
			if !ok {
				flush()
				return
			}
			batch = append(batch, *item)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}
