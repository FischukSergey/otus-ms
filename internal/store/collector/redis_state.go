// Package collector реализует хранилище операционного состояния сбора новостей в Redis.
//
// Операционное состояние каждого источника хранится в Hash nc:source:{id}:
//   - last_collected_at      — unix timestamp последнего успешного сбора
//   - error_count            — счётчик последовательных ошибок
//   - deactivation_count     — сколько раз источник был деактивирован (не сбрасывается)
//
// Временная деактивация (circuit-breaker) хранится как отдельный TTL-ключ nc:deact:{id}.
// Когда ключ существует — источник пропускается при сборе.
// Когда TTL истекает — Redis автоматически удаляет ключ и источник снова становится активным.
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
	keyPrefix              = "nc:source:"
	deactKeyPrefix         = "nc:deact:"
	fieldLastAt            = "last_collected_at"
	fieldErrorCount        = "error_count"
	fieldDeactivationCount = "deactivation_count"
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

// IsLocallyDeactivated проверяет, активен ли circuit-breaker для источника.
// Возвращает true пока существует TTL-ключ nc:deact:{id}.
// Когда TTL истекает — Redis удаляет ключ автоматически, источник снова активен.
func (s *RedisStateStore) IsLocallyDeactivated(ctx context.Context, sourceID string) (bool, error) {
	exists, err := s.client.Exists(ctx, deactKeyPrefix+sourceID).Result()
	if err != nil {
		return false, fmt.Errorf("redis EXISTS deact key for %s: %w", sourceID, err)
	}
	return exists > 0, nil
}

// LocallyDeactivate временно деактивирует источник на заданный backoff-период,
// инкрементирует счётчик деактиваций (используется для расчёта следующего backoff).
// По истечении TTL Redis удаляет ключ и источник автоматически возобновляет сбор.
func (s *RedisStateStore) LocallyDeactivate(ctx context.Context, sourceID string, backoff time.Duration) error {
	pipe := s.client.Pipeline()
	pipe.Set(ctx, deactKeyPrefix+sourceID, 1, backoff)
	pipe.HIncrBy(ctx, key(sourceID), fieldDeactivationCount, 1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline LocallyDeactivate for %s: %w", sourceID, err)
	}
	return nil
}

// GetDeactivationCount возвращает количество раз, которое источник был деактивирован.
// Используется для расчёта следующего exponential backoff.
func (s *RedisStateStore) GetDeactivationCount(ctx context.Context, sourceID string) (int, error) {
	val, err := s.client.HGet(ctx, key(sourceID), fieldDeactivationCount).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("redis HGet deactivation_count for %s: %w", sourceID, err)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse deactivation_count for %s: %w", sourceID, err)
	}
	return n, nil
}
