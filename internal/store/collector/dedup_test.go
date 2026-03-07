package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisstore "github.com/FischukSergey/otus-ms/internal/store/collector"
)

func newTestDedupStore(t *testing.T, ttl time.Duration) *redisstore.RedisDedupStore {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return redisstore.NewRedisDedupStore(client, ttl)
}

func TestRedisDedupStore_IsNewURL_FirstTimeSeen(t *testing.T) {
	store := newTestDedupStore(t, time.Hour)
	ctx := context.Background()

	isNew, err := store.IsNewURL(ctx, "https://example.com/article/1")

	require.NoError(t, err)
	assert.True(t, isNew, "первое появление URL должно вернуть true")
}

func TestRedisDedupStore_IsNewURL_Duplicate(t *testing.T) {
	store := newTestDedupStore(t, time.Hour)
	ctx := context.Background()
	articleURL := "https://example.com/article/1"

	_, err := store.IsNewURL(ctx, articleURL)
	require.NoError(t, err)

	isNew, err := store.IsNewURL(ctx, articleURL)
	require.NoError(t, err)
	assert.False(t, isNew, "повторный URL должен вернуть false")
}

func TestRedisDedupStore_IsNewURL_DifferentURLsAreIndependent(t *testing.T) {
	store := newTestDedupStore(t, time.Hour)
	ctx := context.Background()

	isNew1, err := store.IsNewURL(ctx, "https://example.com/article/1")
	require.NoError(t, err)

	isNew2, err := store.IsNewURL(ctx, "https://example.com/article/2")
	require.NoError(t, err)

	assert.True(t, isNew1)
	assert.True(t, isNew2, "разные URL должны быть независимы")
}

func TestRedisDedupStore_IsNewURL_SameURLAfterTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := redisstore.NewRedisDedupStore(client, time.Second)

	ctx := context.Background()
	articleURL := "https://example.com/article/ttl-test"

	isNew, err := store.IsNewURL(ctx, articleURL)
	require.NoError(t, err)
	assert.True(t, isNew)

	mr.FastForward(2 * time.Second)

	isNew, err = store.IsNewURL(ctx, articleURL)
	require.NoError(t, err)
	assert.True(t, isNew, "после истечения TTL URL должен снова считаться новым")
}

func TestRedisDedupStore_IsNewURL_MultipleDuplicates(t *testing.T) {
	store := newTestDedupStore(t, time.Hour)
	ctx := context.Background()
	articleURL := "https://example.com/article/multi"

	for i := range 5 {
		isNew, err := store.IsNewURL(ctx, articleURL)
		require.NoError(t, err)
		if i == 0 {
			assert.True(t, isNew, "первый вызов должен вернуть true")
		} else {
			assert.False(t, isNew, "последующие вызовы должны вернуть false")
		}
	}
}
