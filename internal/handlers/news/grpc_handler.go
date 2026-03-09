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

// NewsRepository определяет интерфейс доступа к таблице news.
type NewsRepository interface {
	UpsertBatch(ctx context.Context, news []models.ProcessedNews) (int, error)
}

// GRPCHandler реализует NewsServiceServer.
type GRPCHandler struct {
	pb.UnimplementedNewsServiceServer
	repo   NewsRepository
	logger *slog.Logger
}

// NewGRPCHandler создаёт новый gRPC хендлер новостей.
func NewGRPCHandler(repo NewsRepository, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		repo:   repo,
		logger: logger,
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
	return models.ProcessedNews{
		ID:          item.Id,
		SourceID:    item.SourceId,
		Title:       item.Title,
		Summary:     item.Summary,
		URL:         item.Url,
		Category:    item.Category,
		Tags:        item.Tags,
		PublishedAt: time.Unix(item.PublishedAt, 0).UTC(),
		ProcessedAt: time.Unix(item.ProcessedAt, 0).UTC(),
	}
}
