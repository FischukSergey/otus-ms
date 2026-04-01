package news

import (
	"context"
	"fmt"

	"github.com/FischukSergey/otus-ms/internal/models"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// Repository определяет интерфейс чтения новостей из хранилища.
type Repository interface {
	ListLatest(ctx context.Context, limit int) ([]models.NewsBrief, error)
}

// Service предоставляет бизнес-логику чтения новостей.
type Service struct {
	repo Repository
}

// NewService создаёт новый сервис новостей.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListLatest возвращает список последних новостей.
func (s *Service) ListLatest(ctx context.Context, limit int) ([]Response, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	items, err := s.repo.ListLatest(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list latest news: %w", err)
	}

	resp := make([]Response, 0, len(items))
	for _, item := range items {
		resp = append(resp, Response{
			Topic:     item.Topic,
			Source:    item.Source,
			URL:       item.URL,
			CreatedAt: item.CreatedAt,
		})
	}

	return resp, nil
}
