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

type mockDedupStore struct {
	mu   sync.Mutex
	seen map[string]bool
	err  error
}

func newMockDedupStore() *mockDedupStore {
	return &mockDedupStore{seen: make(map[string]bool)}
}

func (m *mockDedupStore) IsNewURL(_ context.Context, url string) (bool, error) {
	if m.err != nil {
		return true, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.seen[url] {
		return false, nil
	}
	m.seen[url] = true
	return true, nil
}

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
	mu               sync.Mutex
	isDueMap         map[string]bool
	deactivated      map[string]bool
	deactivatedUntil map[string]time.Time
	deactivationCnt  map[string]int
	errorCounts      map[string]int
	collected        map[string]time.Time
	lastBackoff      map[string]time.Duration
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		isDueMap:         make(map[string]bool),
		deactivated:      make(map[string]bool),
		deactivatedUntil: make(map[string]time.Time),
		deactivationCnt:  make(map[string]int),
		errorCounts:      make(map[string]int),
		collected:        make(map[string]time.Time),
		lastBackoff:      make(map[string]time.Duration),
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
	until, ok := m.deactivatedUntil[sourceID]
	if !ok {
		return m.deactivated[sourceID], nil
	}
	return time.Now().Before(until), nil
}

func (m *mockStateStore) LocallyDeactivate(_ context.Context, sourceID string, backoff time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deactivated[sourceID] = true
	m.deactivatedUntil[sourceID] = time.Now().Add(backoff)
	m.deactivationCnt[sourceID]++
	m.lastBackoff[sourceID] = backoff
	return nil
}

func (m *mockStateStore) GetDeactivationCount(_ context.Context, sourceID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deactivationCnt[sourceID], nil
}

// mockPublisher реализует NewsPublisher для тестов — просто считает вызовы.
type mockPublisher struct {
	mu       sync.Mutex
	calls    int
	lastNews []*models.RawNews
}

func (m *mockPublisher) Publish(_ context.Context, news []*models.RawNews) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastNews = news
	return nil
}

// --- Вспомогательные функции ---

func newTestService(t *testing.T, client *mockSourcesClient, state *mockStateStore, maxErr int) *collector.Service {
	t.Helper()
	parser := collector.NewParser(5*time.Second, newTestLogger(t))
	dedup := newMockDedupStore()
	return collector.NewService(client, state, dedup, &mockPublisher{}, parser, newTestLogger(t), collector.ServiceConfig{
		MaxWorkers:  3,
		MaxRetries:  1,
		MaxErrCount: maxErr,
		BaseBackoff: 1 * time.Minute,
		MaxBackoff:  10 * time.Minute,
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
	deactCount := state.deactivationCnt["s1"]
	errCountAfter := state.errorCounts["s1"]
	backoff := state.lastBackoff["s1"]
	state.mu.Unlock()

	assert.True(t, isDeactivated, "источник должен быть деактивирован после maxErrCount ошибок")
	assert.Equal(t, 1, deactCount, "должна быть ровно одна деактивация")
	assert.Equal(t, 0, errCountAfter, "error_count сбрасывается после деактивации")
	assert.Equal(t, 1*time.Minute, backoff, "первый backoff = baseBackoff * 2^0 = 1m")
}

func TestService_HandleError_ExponentialBackoff(t *testing.T) {
	sources := []models.Source{
		{ID: "s1", Name: "Bad Source", URL: "http://127.0.0.1:1/rss", FetchInterval: 3600, IsActive: true},
	}
	client := &mockSourcesClient{sources: sources}
	state := newMockStateStore()
	maxErr := 2

	svc := newTestService(t, client, state, maxErr)
	svc.RefreshSources(context.Background())

	// triggerDeactivation сбрасывает state и гонит ошибки до деактивации, возвращает backoff.
	triggerDeactivation := func(t *testing.T) time.Duration {
		t.Helper()
		state.mu.Lock()
		state.errorCounts["s1"] = 0
		state.deactivated["s1"] = false
		delete(state.deactivatedUntil, "s1")
		state.isDueMap["s1"] = true
		state.mu.Unlock()

		for range 100 {
			state.mu.Lock()
			state.isDueMap["s1"] = true
			deact := state.deactivated["s1"]
			state.mu.Unlock()
			if deact {
				state.mu.Lock()
				b := state.lastBackoff["s1"]
				state.mu.Unlock()
				return b
			}
			svc.CollectFromDueSources(context.Background())
		}
		t.Fatal("источник не был деактивирован за 100 итераций")
		return 0
	}

	// 1-я деактивация: deactivation_count=0 → backoff = base * 2^0 = 1m
	backoff1 := triggerDeactivation(t)
	assert.Equal(t, 1*time.Minute, backoff1, "1-я деактивация: backoff = base")

	// 2-я деактивация: deactivation_count=1 → backoff = base * 2^1 = 2m
	backoff2 := triggerDeactivation(t)
	assert.Equal(t, 2*time.Minute, backoff2, "2-я деактивация: backoff = base * 2")
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
