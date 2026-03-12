// Package collector реализует хранилище операционного состояния сбора новостей в Redis.
// Каждый источник хранится как Hash: ключ nc:source:{id},
// поля: last_collected_at (unix timestamp int64), error_count (int), deactivated ("0"/"1").
package collector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/FischukSergey/otus-ms/internal/models"
)

const (
	keyPrefix        = "nc:source:"
	fieldLastAt      = "last_collected_at"
	fieldErrorCount  = "error_count"
	fieldDeactivated = "deactivated"
)

// RedisStateStore хранит операционное состояние сбора в Redis.
// Интерфейс StateStore объявлен в пакете-потребителе (services/collector).
type RedisStateStore struct {
	client *redis.Client
}

// NewRedisStateStore создаёт новый RedisStateStore.
func NewRedisStateStore(client *redis.Client) *RedisStateStore {
	return &RedisStateStore{client: client}
}

// key формирует Redis-ключ для источника.
func key(sourceID string) string {
	return keyPrefix + sourceID
}

// IsDue проверяет, пора ли собирать новости из источника.
// Возвращает true если:
//   - источник ещё никогда не собирался (ключа нет в Redis)
//   - прошло не менее FetchInterval секунд с последнего сбора
func (s *RedisStateStore) IsDue(ctx context.Context, source *models.Source) (bool, error) {
	val, err := s.client.HGet(ctx, key(source.ID), fieldLastAt).Result()
	if err == redis.Nil {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis HGet last_collected_at for %s: %w", source.ID, err)
	}

	ts, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse last_collected_at for %s: %w", source.ID, err)
	}

	nextFetch := time.Unix(ts, 0).Add(time.Duration(source.FetchInterval) * time.Second)
	return time.Now().After(nextFetch), nil
}

// SetCollected записывает время успешного сбора.
func (s *RedisStateStore) SetCollected(ctx context.Context, sourceID string, t time.Time) error {
	err := s.client.HSet(ctx, key(sourceID), fieldLastAt, t.Unix()).Err()
	if err != nil {
		return fmt.Errorf("redis HSet last_collected_at for %s: %w", sourceID, err)
	}
	return nil
}

// IncrementErrorCount атомарно увеличивает счётчик ошибок и возвращает новое значение.
func (s *RedisStateStore) IncrementErrorCount(ctx context.Context, sourceID string) (int, error) {
	val, err := s.client.HIncrBy(ctx, key(sourceID), fieldErrorCount, 1).Result()
	if err != nil {
		return 0, fmt.Errorf("redis HIncrBy error_count for %s: %w", sourceID, err)
	}
	return int(val), nil
}

// ResetErrorCount сбрасывает счётчик ошибок в 0.
func (s *RedisStateStore) ResetErrorCount(ctx context.Context, sourceID string) error {
	err := s.client.HSet(ctx, key(sourceID), fieldErrorCount, 0).Err()
	if err != nil {
		return fmt.Errorf("redis HSet error_count for %s: %w", sourceID, err)
	}
	return nil
}

// IsLocallyDeactivated проверяет, деактивирован ли источник локально в news-collector.
// Локальная деактивация не влияет на is_active в main-service.
func (s *RedisStateStore) IsLocallyDeactivated(ctx context.Context, sourceID string) (bool, error) {
	val, err := s.client.HGet(ctx, key(sourceID), fieldDeactivated).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis HGet deactivated for %s: %w", sourceID, err)
	}
	return val == "1", nil
}

// LocallyDeactivate помечает источник как локально деактивированный.
// Вызывается когда error_count достигает максимума.
func (s *RedisStateStore) LocallyDeactivate(ctx context.Context, sourceID string) error {
	err := s.client.HSet(ctx, key(sourceID), fieldDeactivated, "1").Err()
	if err != nil {
		return fmt.Errorf("redis HSet deactivated for %s: %w", sourceID, err)
	}
	return nil
}
