package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/models"
	redisstate "github.com/FischukSergey/otus-ms/internal/store/collector"
)

func newTestStore(t *testing.T) *redisstate.RedisStateStore {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return redisstate.NewRedisStateStore(client)
}

func newSource(id string, fetchIntervalSec int) *models.Source {
	return &models.Source{
		ID:            id,
		Name:          "Test Source",
		FetchInterval: fetchIntervalSec,
		IsActive:      true,
	}
}

func TestRedisStateStore_IsDue_NoKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	due, err := store.IsDue(ctx, newSource("source_1", 3600))

	require.NoError(t, err)
	assert.True(t, due, "источник без записи в Redis должен считаться готовым к сбору")
}

func TestRedisStateStore_IsDue_FreshlyCollected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	source := newSource("source_1", 3600)

	err := store.SetCollected(ctx, source.ID, time.Now())
	require.NoError(t, err)

	due, err := store.IsDue(ctx, source)
	require.NoError(t, err)
	assert.False(t, due, "только что собранный источник не должен быть готов к сбору")
}

func TestRedisStateStore_IsDue_IntervalExpired(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	source := newSource("source_1", 60)

	// Записываем время сбора 2 минуты назад
	pastTime := time.Now().Add(-2 * time.Minute)
	err := store.SetCollected(ctx, source.ID, pastTime)
	require.NoError(t, err)

	due, err := store.IsDue(ctx, source)
	require.NoError(t, err)
	assert.True(t, due, "источник с истёкшим интервалом должен быть готов к сбору")
}

func TestRedisStateStore_SetCollected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	err := store.SetCollected(ctx, "source_1", now)
	require.NoError(t, err)

	source := newSource("source_1", 3600)
	due, err := store.IsDue(ctx, source)
	require.NoError(t, err)
	assert.False(t, due)
}

func TestRedisStateStore_IncrementErrorCount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	count1, err := store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)
	assert.Equal(t, 1, count1)

	count2, err := store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)
	assert.Equal(t, 2, count2)

	count3, err := store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)
	assert.Equal(t, 3, count3)
}

func TestRedisStateStore_ResetErrorCount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)
	_, err = store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)

	err = store.ResetErrorCount(ctx, "source_1")
	require.NoError(t, err)

	// После сброса следующий инкремент даст 1
	count, err := store.IncrementErrorCount(ctx, "source_1")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRedisStateStore_LocallyDeactivate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	deactivated, err := store.IsLocallyDeactivated(ctx, "source_1")
	require.NoError(t, err)
	assert.False(t, deactivated)

	err = store.LocallyDeactivate(ctx, "source_1")
	require.NoError(t, err)

	deactivated, err = store.IsLocallyDeactivated(ctx, "source_1")
	require.NoError(t, err)
	assert.True(t, deactivated)
}

func TestRedisStateStore_DifferentSources_Independent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.LocallyDeactivate(ctx, "source_1")
	require.NoError(t, err)

	deactivated, err := store.IsLocallyDeactivated(ctx, "source_2")
	require.NoError(t, err)
	assert.False(t, deactivated, "деактивация source_1 не должна влиять на source_2")

	_, _ = store.IncrementErrorCount(ctx, "source_1")
	_, _ = store.IncrementErrorCount(ctx, "source_1")

	count, err := store.IncrementErrorCount(ctx, "source_2")
	require.NoError(t, err)
	assert.Equal(t, 1, count, "счётчик source_2 должен начинаться с 0")
}

func TestRedisStateStore_IsDue_MultipleSourceIDs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Проверяем IsDue для разных ID источников
	for _, id := range []string{"source_1", "source_2", "source_abc"} {
		due, err := store.IsDue(ctx, newSource(id, 3600))
		require.NoError(t, err)
		assert.True(t, due, "новый источник %s должен быть готов к сбору", id)
	}
}
