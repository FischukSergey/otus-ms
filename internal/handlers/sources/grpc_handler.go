package sources

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/FischukSergey/otus-ms/internal/models"
	pb "github.com/FischukSergey/otus-ms/pkg/news_sources/v1"
)

// SourceRepository определяет интерфейс доступа к источникам новостей.
type SourceRepository interface {
	GetAll(ctx context.Context) ([]models.Source, error)
}

// GRPCHandler реализует NewsSourcesServiceServer.
type GRPCHandler struct {
	pb.UnimplementedNewsSourcesServiceServer
	repo   SourceRepository
	logger *slog.Logger
}

// NewGRPCHandler создаёт новый gRPC хендлер.
func NewGRPCHandler(repo SourceRepository, logger *slog.Logger) *GRPCHandler {
	return &GRPCHandler{
		repo:   repo,
		logger: logger,
	}
}

// GetNewsSources возвращает список всех источников новостей.
//
// @Router /news_sources.v1.NewsSourcesService/GetNewsSources [post].
func (h *GRPCHandler) GetNewsSources(
	ctx context.Context,
	_ *pb.GetNewsSourcesRequest,
) (*pb.GetNewsSourcesResponse, error) {
	sources, err := h.repo.GetAll(ctx)
	if err != nil {
		h.logger.Error("grpc GetNewsSources: failed to get sources from DB", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get sources: %v", err))
	}

	pbSources := make([]*pb.NewsSource, 0, len(sources))
	for _, s := range sources {
		pbSources = append(pbSources, modelToProto(s))
	}

	h.logger.Debug("grpc GetNewsSources: sources returned", "count", len(pbSources))

	return &pb.GetNewsSourcesResponse{Sources: pbSources}, nil
}

// modelToProto конвертирует модель Source в proto-сообщение NewsSource.
func modelToProto(s models.Source) *pb.NewsSource {
	return &pb.NewsSource{
		Id:            s.ID,
		Name:          s.Name,
		Url:           s.URL,
		Language:      s.Language,
		Category:      s.Category,
		FetchInterval: int32(s.FetchInterval),
		IsActive:      s.IsActive,
	}
}
