// Package ratelimiter реализует Rate Limiter на основе Redis для ограничения
// числа запросов с одного IP-адреса в заданном временном окне.
package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter ограничивает количество запросов по ключу (IP) с помощью Redis.
// Алгоритм: INCR + EXPIRE (скользящее окно по первому запросу).
type Limiter struct {
	client        *redis.Client
	maxAttempts   int
	windowSeconds int
}

// New создаёт новый Limiter и проверяет доступность Redis.
func New(ctx context.Context, redisAddr, redisPassword string, maxAttempts, windowSeconds int) (*Limiter, error) {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if windowSeconds <= 0 {
		windowSeconds = 60
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ratelimiter: failed to connect to Redis at %s: %w", redisAddr, err)
	}

	return &Limiter{
		client:        client,
		maxAttempts:   maxAttempts,
		windowSeconds: windowSeconds,
	}, nil
}

// Allow проверяет, разрешён ли следующий запрос для данного IP.
// Возвращает true если запрос разрешён, false если лимит превышен.
// Инкрементирует счётчик атомарно через pipeline.
func (l *Limiter) Allow(ctx context.Context, ip string) (bool, error) {
	key := "ratelimit:login:" + ip

	pipe := l.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	// Всегда обновляем TTL — expire работает идемпотентно, если TTL не менялся извне
	pipe.Expire(ctx, key, time.Duration(l.windowSeconds)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("ratelimiter: pipeline exec: %w", err)
	}

	count := incr.Val()
	return count <= int64(l.maxAttempts), nil
}

// Close закрывает соединение с Redis.
func (l *Limiter) Close() error {
	return l.client.Close()
}
