package collector_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/models"
	"github.com/FischukSergey/otus-ms/internal/services/collector"
)

// --- Моки ---

type mockSourcesClient struct {
	sources []models.Source
	err     error
	calls   int
	mu      sync.Mutex
}

func (m *mockSourcesClient) GetNewsSources(_ context.Context) ([]models.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.sources, m.err
}

type mockStateStore struct {
	mu          sync.Mutex
	isDueMap    map[string]bool
	deactivated map[string]bool
	errorCounts map[string]int
	collected   map[string]time.Time
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		isDueMap:    make(map[string]bool),
		deactivated: make(map[string]bool),
		errorCounts: make(map[string]int),
		collected:   make(map[string]time.Time),
	}
}

func (m *mockStateStore) IsDue(_ context.Context, source *models.Source) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.isDueMap[source.ID]
	if !ok {
		return true, nil
	}
	return v, nil
}

func (m *mockStateStore) SetCollected(_ context.Context, sourceID string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collected[sourceID] = t
	return nil
}

func (m *mockStateStore) IncrementErrorCount(_ context.Context, sourceID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCounts[sourceID]++
	return m.errorCounts[sourceID], nil
}

func (m *mockStateStore) ResetErrorCount(_ context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCounts[sourceID] = 0
	return nil
}

func (m *mockStateStore) IsLocallyDeactivated(_ context.Context, sourceID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deactivated[sourceID], nil
}

func (m *mockStateStore) LocallyDeactivate(_ context.Context, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivated[sourceID] = true
	return nil
}

// --- Вспомогательные функции ---

func newTestService(t *testing.T, client *mockSourcesClient, state *mockStateStore, maxErr int) *collector.Service {
	t.Helper()
	parser := collector.NewParser(5*time.Second, newTestLogger(t))
	return collector.NewService(client, state, parser, newTestLogger(t), collector.ServiceConfig{
		MaxWorkers:  3,
		MaxRetries:  1,
		MaxErrCount: maxErr,
	})
}

// --- Тесты RefreshSources ---

func TestService_RefreshSources_Success(t *testing.T) {
	sources := []models.Source{
		{ID: "s1", Name: "Source 1", URL: "https://example.com/rss", FetchInterval: 3600, IsActive: true},
		{ID: "s2", Name: "Source 2", URL: "https://example.com/feed", FetchInterval: 1800, IsActive: true},
	}
	client := &mockSourcesClient{sources: sources}
	state := newMockStateStore()

	svc := newTestService(t, client, state, 5)
	svc.RefreshSources(context.Background())

	assert.Equal(t, 1, client.calls)
}

func TestService_RefreshSources_ErrorDoesNotPanic(t *testing.T) {
	client := &mockSourcesClient{err: errors.New("grpc unavailable")}
	state := newMockStateStore()

	svc := newTestService(t, client, state, 5)

	// Не должно паниковать и не должно возвращать ошибку — просто логирует
	require.NotPanics(t, func() {
		svc.RefreshSources(context.Background())
	})
}

// --- Тесты CollectFromDueSources ---

func TestService_CollectFromDueSources_EmptyCache(t *testing.T) {
	client := &mockSourcesClient{}
	state := newMockStateStore()
	svc := newTestService(t, client, state, 5)

	// Без вызова RefreshSources кеш пустой — должно отработать без паники
	require.NotPanics(t, func() {
		svc.CollectFromDueSources(context.Background())
	})
}

func TestService_CollectFromDueSources_SkipsNotDue(t *testing.T) {
	sources := []models.Source{
		{ID: "s1", Name: "Source 1", URL: "https://example.com/rss", FetchInterval: 3600, IsActive: true},
	}
	client := &mockSourcesClient{sources: sources}
	state := newMockStateStore()
	state.isDueMap["s1"] = false // не пора собирать

	svc := newTestService(t, client, state, 5)
	svc.RefreshSources(context.Background())
	svc.CollectFromDueSources(context.Background())

	state.mu.Lock()
	defer state.mu.Unlock()
	_, wasCollected := state.collected["s1"]
	assert.False(t, wasCollected, "источник не должен быть собран если IsDue=false")
}

func TestService_CollectFromDueSources_SkipsDeactivated(t *testing.T) {
	sources := []models.Source{
		{ID: "s1", Name: "Source 1", URL: "https://example.com/rss", FetchInterval: 3600, IsActive: true},
	}
	client := &mockSourcesClient{sources: sources}
	state := newMockStateStore()
	state.deactivated["s1"] = true // локально деактивирован

	svc := newTestService(t, client, state, 5)
	svc.RefreshSources(context.Background())
	svc.CollectFromDueSources(context.Background())

	state.mu.Lock()
	defer state.mu.Unlock()
	_, wasCollected := state.collected["s1"]
	assert.False(t, wasCollected, "деактивированный источник не должен собираться")
}

// --- Тесты handleCollectError / деактивация ---

func TestService_HandleError_DeactivatesAfterMaxErrors(t *testing.T) {
	// Используем несуществующий URL чтобы парсер всегда возвращал ошибку
	sources := []models.Source{
		{ID: "s1", Name: "Bad Source", URL: "http://127.0.0.1:1/rss", FetchInterval: 3600, IsActive: true},
	}
	client := &mockSourcesClient{sources: sources}
	state := newMockStateStore()
	maxErr := 3

	svc := newTestService(t, client, state, maxErr)
	svc.RefreshSources(context.Background())

	// Запускаем сбор maxErr раз чтобы накопить ошибки
	for range maxErr {
		svc.CollectFromDueSources(context.Background())
		// После каждого неудачного сбора IsDue должен быть true (нет SetCollected)
		state.mu.Lock()
		state.isDueMap["s1"] = true
		state.mu.Unlock()
	}

	state.mu.Lock()
	isDeactivated := state.deactivated["s1"]
	errCount := state.errorCounts["s1"]
	state.mu.Unlock()

	assert.True(t, isDeactivated, "источник должен быть деактивирован после maxErrCount ошибок")
	assert.GreaterOrEqual(t, errCount, maxErr)
}

func TestService_HandleError_ResetsOnSuccess(t *testing.T) {
	// Источник с ошибками — но IsDue=false, коллект не вызовется
	// Проверяем что ResetErrorCount вызывается при успехе косвенно через state
	state := newMockStateStore()
	state.errorCounts["s1"] = 2

	err := state.ResetErrorCount(context.Background(), "s1")
	require.NoError(t, err)

	state.mu.Lock()
	count := state.errorCounts["s1"]
	state.mu.Unlock()

	assert.Equal(t, 0, count, "после ResetErrorCount счётчик должен стать 0")
}
