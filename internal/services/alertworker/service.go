// Package alertworker содержит бизнес-логику доставки алертов.
package alertworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/FischukSergey/otus-ms/internal/config"
	"github.com/FischukSergey/otus-ms/internal/models"
)

const (
	retryAttempts = 3
)

// DeliveryClient определяет интерфейс оркестрации доставки через main-service.
type DeliveryClient interface {
	ReserveAlertDelivery(ctx context.Context, event models.NewsAlertEvent) (bool, string, error)
	FinalizeAlertDelivery(ctx context.Context, eventID, status, errorMessage string, sentAt *time.Time) error
}

// Sender определяет интерфейс отправки сообщения в канал доставки.
type Sender interface {
	Send(ctx context.Context, event models.NewsAlertEvent) error
}

// Service читает события из Kafka и доставляет их в Telegram.
type Service struct {
	reader    *kafka.Reader
	dltWriter *kafka.Writer
	delivery  DeliveryClient
	sender    Sender
	logger    *slog.Logger
}

// NewService создает alert-worker сервис.
func NewService(
	kafkaCfg config.KafkaConfig,
	delivery DeliveryClient,
	sender Sender,
	logger *slog.Logger,
) *Service {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     kafkaCfg.Brokers,
		Topic:       kafkaCfg.TopicNewsAlerts,
		GroupID:     kafkaCfg.ConsumerGroup + "-alert-worker",
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})

	dltTopic := kafkaCfg.TopicNewsAlertsDLT
	if dltTopic == "" {
		dltTopic = kafkaCfg.TopicNewsAlerts + ".DLT"
	}

	dltWriter := &kafka.Writer{
		Addr:                   kafka.TCP(kafkaCfg.Brokers...),
		Topic:                  dltTopic,
		Balancer:               &kafka.Hash{},
		BatchSize:              kafkaCfg.GetBatchSize(),
		BatchTimeout:           kafkaCfg.GetBatchTimeout(),
		WriteTimeout:           kafkaCfg.GetWriteTimeout(),
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: false,
	}

	logger.Info("alert-worker initialized",
		"alerts_topic", kafkaCfg.TopicNewsAlerts,
		"dlt_topic", dltTopic,
		"brokers", kafkaCfg.Brokers,
	)

	return &Service{
		reader:    reader,
		dltWriter: dltWriter,
		delivery:  delivery,
		sender:    sender,
		logger:    logger,
	}
}

// Run запускает цикл обработки alert-событий.
func (s *Service) Run(ctx context.Context) error {
	defer func() {
		_ = s.reader.Close()
		_ = s.dltWriter.Close()
	}()

	for {
		msg, err := s.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			s.logger.Error("alert-worker read kafka message failed", "error", err)
			continue
		}

		var event models.NewsAlertEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			s.logger.Warn("alert-worker skip invalid message", "error", err, "offset", msg.Offset)
			continue
		}

		if err := s.handleEvent(ctx, event); err != nil {
			s.logger.Error("alert-worker handle event failed",
				"event_id", event.EventID,
				"rule_id", event.RuleID,
				"news_id", event.NewsID,
				"error", err,
			)
		}
	}
}

func (s *Service) handleEvent(ctx context.Context, event models.NewsAlertEvent) error {
	shouldSend, reason, err := s.delivery.ReserveAlertDelivery(ctx, event)
	if err != nil {
		return fmt.Errorf("reserve alert delivery: %w", err)
	}
	if !shouldSend {
		s.logger.Debug("alert event skipped by reserve decision",
			"event_id", event.EventID,
			"reason", reason,
		)
		return nil
	}

	var sendErr error
	backoff := time.Second
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		sendErr = s.sender.Send(ctx, event)
		if sendErr == nil {
			sentAt := time.Now().UTC()
			if err := s.delivery.FinalizeAlertDelivery(ctx, event.EventID, "sent", "", &sentAt); err != nil {
				return fmt.Errorf("finalize sent: %w", err)
			}
			return nil
		}

		if attempt < retryAttempts {
			select {
			case <-time.After(backoff):
				backoff *= 2
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if err := s.delivery.FinalizeAlertDelivery(ctx, event.EventID, "failed", sendErr.Error(), nil); err != nil {
		return fmt.Errorf("finalize failed: %w", err)
	}

	if err := s.publishDLT(ctx, event); err != nil {
		return fmt.Errorf("publish to DLT: %w", err)
	}

	return nil
}

func (s *Service) publishDLT(ctx context.Context, event models.NewsAlertEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal DLT event: %w", err)
	}

	return s.dltWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.RuleID),
		Value: value,
	})
}
