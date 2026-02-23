package mainservice

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/FischukSergey/otus-ms/internal/models"
	pb "github.com/FischukSergey/otus-ms/pkg/news_sources/v1"
)

// TokenProvider определяет интерфейс для получения JWT-токена сервисного аккаунта.
// Реализуется keycloak.Client через метод GetServiceAccountToken.
type TokenProvider interface {
	GetServiceAccountToken(ctx context.Context) (string, error)
}

// GRPCClient — клиент для обращения к main-service по gRPC.
type GRPCClient struct {
	conn          *grpc.ClientConn
	sourcesClient pb.NewsSourcesServiceClient
	tokenProvider TokenProvider
	logger        *slog.Logger
}

// NewGRPCClient создаёт новый gRPC клиент main-service.
// addr — адрес gRPC сервера main-service (например "localhost:50051").
func NewGRPCClient(addr string, tokenProvider TokenProvider, logger *slog.Logger) (*GRPCClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial main-service %s: %w", addr, err)
	}

	return &GRPCClient{
		conn:          conn,
		sourcesClient: pb.NewNewsSourcesServiceClient(conn),
		tokenProvider: tokenProvider,
		logger:        logger,
	}, nil
}

// Close закрывает gRPC соединение.
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

// GetNewsSources запрашивает список источников новостей у main-service.
// Автоматически получает JWT-токен от Keycloak и добавляет его в metadata.
func (c *GRPCClient) GetNewsSources(ctx context.Context) ([]models.Source, error) {
	// Получаем service account JWT от Keycloak
	token, err := c.tokenProvider.GetServiceAccountToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get service account token: %w", err)
	}

	// Добавляем токен в gRPC metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	resp, err := c.sourcesClient.GetNewsSources(ctx, &pb.GetNewsSourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc GetNewsSources: %w", err)
	}

	sources := make([]models.Source, 0, len(resp.Sources))
	for _, s := range resp.Sources {
		sources = append(sources, protoToModel(s))
	}

	c.logger.Debug("grpc GetNewsSources: received sources", "count", len(sources))
	return sources, nil
}

// protoToModel конвертирует proto-сообщение NewsSource в модель Source.
func protoToModel(s *pb.NewsSource) models.Source {
	return models.Source{
		ID:            s.Id,
		Name:          s.Name,
		URL:           s.Url,
		Language:      s.Language,
		Category:      s.Category,
		FetchInterval: int(s.FetchInterval),
		IsActive:      s.IsActive,
	}
}
