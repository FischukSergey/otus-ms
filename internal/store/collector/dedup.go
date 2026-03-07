package collector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const dedupKeyPrefix = "nc:seen:"

// RedisDedupStore реализует атомарную дедупликацию новостей по URL через Redis SETNX.
// Ключ: nc:seen:{sha256(normalizedURL)}, значение: 1, TTL — настраиваемый.
// SETNX атомарен: безопасен при параллельных worker'ах (см. Service.CollectFromDueSources).
type RedisDedupStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisDedupStore создаёт RedisDedupStore с заданным TTL хранения seen-URL.
func NewRedisDedupStore(client *redis.Client, ttl time.Duration) *RedisDedupStore {
	return &RedisDedupStore{client: client, ttl: ttl}
}

// IsNewURL атомарно проверяет, встречался ли URL ранее, и помечает его как виденный.
// Возвращает true если URL новый (SETNX вернул 1), false если дубль.
// При ошибке Redis возвращает (true, err) — fail open, чтобы не потерять новости.
func (d *RedisDedupStore) IsNewURL(ctx context.Context, rawURL string) (bool, error) {
	h := sha256.Sum256([]byte(rawURL))
	k := dedupKeyPrefix + hex.EncodeToString(h[:])

	_, err := d.client.SetArgs(ctx, k, 1, redis.SetArgs{
		Mode: "NX",
		TTL:  d.ttl,
	}).Result()
	if err == redis.Nil {
		// Ключ уже существует — URL виден ранее
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("dedup Set NX: %w", err)
	}
	return true, nil
}
