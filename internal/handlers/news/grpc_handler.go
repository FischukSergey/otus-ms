// Package news реализует gRPC хендлер для сервиса обработанных новостей.
package news

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/FischukSergey/otus-ms/internal/models"
	pb "github.com/FischukSergey/otus-ms/pkg/news/v1"
)

// Repository определяет интерфейс доступа к таблице news.
type Repository interface {
	UpsertBatch(ctx context.Context, news []models.ProcessedNews) (int, error)
}

// AlertRulesRepository определяет интерфейс доступа к активным правилам алертинга.
type AlertRulesRepository interface {
	ListActiveRules(ctx context.Context) ([]models.AlertRule, error)
	CreatePendingEvent(ctx context.Context, event models.NewsAlertEvent) (bool, error)
	GetRuleCooldownSeconds(ctx context.Context, ruleID string) (int, error)
	GetLastSentAt(ctx context.Context, ruleID string) (*time.Time, error)
	MarkDropped(ctx context.Context, eventID, reason string) error
	MarkSent(ctx context.Context, eventID string, sentAt time.Time) error
	MarkFailed(ctx context.Context, eventID, errMsg string) error
}

// GRPCHandler реализует NewsServiceServer.
type GRPCHandler struct {
	pb.UnimplementedNewsServiceServer
	repo      Repository
	alertRepo AlertRulesRepository
	logger    *slog.Logger
}

// NewGRPCHandler создаёт новый gRPC хендлер новостей.
func NewGRPCHandler(repo Repository, alertRepo AlertRulesRepository, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		repo:      repo,
		alertRepo: alertRepo,
		logger:    logger,
	}
}

// SaveProcessedNews сохраняет пачку обработанных новостей в PostgreSQL.
// Дубликаты по URL игнорируются — операция идемпотентна.
//
// @Router /news.v1.NewsService/SaveProcessedNews [post].
func (h *GRPCHandler) SaveProcessedNews(
	ctx context.Context,
	req *pb.SaveProcessedNewsRequest,
) (*pb.SaveProcessedNewsResponse, error) {
	if len(req.News) == 0 {
		return &pb.SaveProcessedNewsResponse{SavedCount: 0}, nil
	}

	news := make([]models.ProcessedNews, 0, len(req.News))
	for _, item := range req.News {
		if item.Id == "" || item.Url == "" {
			h.logger.Warn("grpc SaveProcessedNews: skipping item with empty id or url",
				"id", item.Id, "url", item.Url)
			continue
		}
		news = append(news, protoToModel(item))
	}

	savedCount, err := h.repo.UpsertBatch(ctx, news)
	if err != nil {
		h.logger.Error("grpc SaveProcessedNews: upsert failed",
			"count", len(news), "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("upsert news: %v", err))
	}

	h.logger.Debug("grpc SaveProcessedNews: saved",
		"requested", len(req.News), "saved", savedCount)

	return &pb.SaveProcessedNewsResponse{SavedCount: int32(savedCount)}, nil //nolint:gosec // saved_count is always small
}

// protoToModel конвертирует proto-сообщение в доменную модель ProcessedNews.
func protoToModel(item *pb.ProcessedNewsItem) models.ProcessedNews {
	// В protobuf пустой repeated field десериализуется как nil, а не []string{}.
	// pgx передаёт nil-слайс как SQL NULL, что нарушает NOT NULL на колонке tags.
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}

	return models.ProcessedNews{
		ID:          item.Id,
		SourceID:    item.SourceId,
		Title:       item.Title,
		Summary:     item.Summary,
		URL:         item.Url,
		S3Key:       item.S3Key,
		Category:    item.Category,
		Tags:        tags,
		PublishedAt: time.Unix(item.PublishedAt, 0).UTC(),
		ProcessedAt: time.Unix(item.ProcessedAt, 0).UTC(),
	}
}

// GetActiveAlertRules возвращает активные правила алертинга для news-processor.
//
// @Router /news.v1.NewsService/GetActiveAlertRules [post].
func (h *GRPCHandler) GetActiveAlertRules(
	ctx context.Context,
	_ *pb.GetActiveAlertRulesRequest,
) (*pb.GetActiveAlertRulesResponse, error) {
	rules, err := h.alertRepo.ListActiveRules(ctx)
	if err != nil {
		h.logger.Error("grpc GetActiveAlertRules: query failed", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("list active alert rules: %v", err))
	}

	resp := make([]*pb.AlertRule, 0, len(rules))
	for i := range rules {
		if rules[i].ID == "" || rules[i].UserUUID == "" || rules[i].Keyword == "" {
			continue
		}
		resp = append(resp, &pb.AlertRule{
			Id:              rules[i].ID,
			UserUuid:        rules[i].UserUUID,
			Keyword:         rules[i].Keyword,
			CooldownSeconds: int32(rules[i].CooldownSeconds),
		})
	}

	return &pb.GetActiveAlertRulesResponse{Rules: resp}, nil
}

// ReserveAlertDelivery резервирует доставку: дедуп + cooldown + pending.
//
// @Router /news.v1.NewsService/ReserveAlertDelivery [post].
func (h *GRPCHandler) ReserveAlertDelivery(
	ctx context.Context,
	req *pb.ReserveAlertDeliveryRequest,
) (*pb.ReserveAlertDeliveryResponse, error) {
	event := models.NewsAlertEvent{
		EventID:  req.EventId,
		RuleID:   req.RuleId,
		UserUUID: req.UserUuid,
		NewsID:   req.NewsId,
		Keyword:  req.Keyword,
	}

	inserted, err := h.alertRepo.CreatePendingEvent(ctx, event)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("create pending event: %v", err))
	}
	if !inserted {
		return &pb.ReserveAlertDeliveryResponse{
			ShouldSend: false,
			Reason:     "duplicate",
		}, nil
	}

	cooldownSeconds, err := h.alertRepo.GetRuleCooldownSeconds(ctx, req.RuleId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("get cooldown: %v", err))
	}
	lastSentAt, err := h.alertRepo.GetLastSentAt(ctx, req.RuleId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("get last sent_at: %v", err))
	}

	if lastSentAt != nil && cooldownSeconds > 0 &&
		time.Since(*lastSentAt) < time.Duration(cooldownSeconds)*time.Second {
		reason := fmt.Sprintf("dropped by cooldown: %ds", cooldownSeconds)
		if err := h.alertRepo.MarkDropped(ctx, req.EventId, reason); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("mark dropped: %v", err))
		}

		return &pb.ReserveAlertDeliveryResponse{
			ShouldSend: false,
			Reason:     "cooldown",
		}, nil
	}

	return &pb.ReserveAlertDeliveryResponse{
		ShouldSend: true,
		Reason:     "ready",
	}, nil
}

// FinalizeAlertDelivery фиксирует финальный статус доставки.
//
// @Router /news.v1.NewsService/FinalizeAlertDelivery [post].
func (h *GRPCHandler) FinalizeAlertDelivery(
	ctx context.Context,
	req *pb.FinalizeAlertDeliveryRequest,
) (*pb.FinalizeAlertDeliveryResponse, error) {
	switch req.Status {
	case "sent":
		sentAt := time.Now().UTC()
		if req.SentAtUnix > 0 {
			sentAt = time.Unix(req.SentAtUnix, 0).UTC()
		}
		if err := h.alertRepo.MarkSent(ctx, req.EventId, sentAt); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("mark sent: %v", err))
		}
	case "failed":
		if err := h.alertRepo.MarkFailed(ctx, req.EventId, req.ErrorMessage); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("mark failed: %v", err))
		}
	case "dropped":
		if err := h.alertRepo.MarkDropped(ctx, req.EventId, req.ErrorMessage); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("mark dropped: %v", err))
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be sent, failed or dropped")
	}

	return &pb.FinalizeAlertDeliveryResponse{}, nil
}
