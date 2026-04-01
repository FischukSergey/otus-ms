// Package producer содержит реализации публикаторов сообщений.
package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/models"
)

// KafkaProducer публикует RawNews в топик Kafka raw_news.
// Ключ сообщения — source_id: партиционирование по источнику обеспечивает
// упорядоченную обработку новостей одного источника внутри одной партиции.
type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// NewKafkaProducer создаёт продюсер с настройками из конфига.
func NewKafkaProducer(cfg config.KafkaConfig, logger *slog.Logger) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.TopicRawNews,
		Balancer:     &kafka.Hash{}, // партиционирование по ключу (source_id)
		BatchSize:    cfg.GetBatchSize(),
		BatchTimeout: cfg.GetBatchTimeout(),
		WriteTimeout: cfg.GetWriteTimeout(),
		// RequiredAcks=1: лидер подтверждает запись — баланс надёжности и производительности.
		RequiredAcks: kafka.RequireOne,
		// Автосоздание топика отключено — топики создаются через kafka-init контейнер.
		AllowAutoTopicCreation: false,
	}

	logger.Info("kafka producer initialized",
		"brokers", cfg.Brokers,
		"topic", cfg.TopicRawNews,
		"batch_size", cfg.GetBatchSize(),
		"batch_timeout", cfg.GetBatchTimeout(),
	)

	return &KafkaProducer{
		writer: writer,
		logger: logger,
	}
}

// Publish отправляет пачку новостей в Kafka топик raw_news.
// При ошибке логирует и возвращает err — вызывающий код продолжает работу
// (не блокируем сборщик из-за недоступности Kafka).
func (p *KafkaProducer) Publish(ctx context.Context, news []*models.RawNews) error {
	if len(news) == 0 {
		return nil
	}

	messages := make([]kafka.Message, 0, len(news))
	for _, item := range news {
		value, err := json.Marshal(item)
		if err != nil {
			p.logger.Error("failed to marshal raw news", "id", item.ID, "error", err)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(item.SourceID), // ключ для партиционирования по источнику
			Value: value,
		})
	}

	if len(messages) == 0 {
		return nil
	}

	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("kafka write messages: %w", err)
	}

	p.logger.Debug("published to kafka", "topic", p.writer.Topic, "count", len(messages))
	return nil
}

// Close закрывает соединение с Kafka. Вызывать при завершении работы сервиса.
func (p *KafkaProducer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("kafka writer close: %w", err)
	}
	return nil
}

// NoopPublisher — заглушка для случая, когда Kafka не сконфигурирована.
// Используется при локальной разработке без Kafka.
type NoopPublisher struct {
	logger *slog.Logger
}

// NewNoopPublisher создаёт заглушку публикатора.
func NewNoopPublisher(logger *slog.Logger) *NoopPublisher {
	logger.Info("kafka not configured, using noop publisher (news will not be sent to kafka)")
	return &NoopPublisher{logger: logger}
}

// Publish логирует новости вместо отправки в Kafka.
func (n *NoopPublisher) Publish(_ context.Context, news []*models.RawNews) error {
	if len(news) > 0 {
		n.logger.Debug("noop publisher: skipping kafka publish", "count", len(news))
	}
	return nil
}

// Close — ничего не делает, реализует интерфейс.
func (n *NoopPublisher) Close() error { return nil }
