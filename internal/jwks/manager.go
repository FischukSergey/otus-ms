package jwks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// Manager управляет кешем JWKS ключей от Keycloak.
// Предназначен для использования во всех микросервисах проекта.
type Manager struct {
	cache         *jwk.Cache
	jwksURL       string
	cacheDuration time.Duration
	logger        *slog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewManager создает новый JWKS Manager с автоматическим обновлением ключей.
//
// Параметры:
//   - jwksURL: URL к JWKS endpoint Keycloak
//   - cacheDuration: время кеширования ключей в секундах (600 = 10 минут)
//   - logger: логгер для записи событий
//
// Возвращает:
//   - *Manager: инициализированный менеджер
//   - error: ошибка инициализации
func NewManager(
	jwksURL string,
	cacheDuration int,
	logger *slog.Logger,
) (*Manager, error) {
	if jwksURL == "" {
		return nil, errors.New("jwks_url is required")
	}

	duration := time.Duration(cacheDuration) * time.Second
	if duration == 0 {
		duration = 10 * time.Minute // По умолчанию 10 минут
	}

	ctx, cancel := context.WithCancel(context.Background())
	cache := jwk.NewCache(ctx)

	// Регистрируем JWKS URL с автоматическим обновлением
	err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(duration))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to register JWKS cache: %w", err)
	}

	manager := &Manager{
		cache:         cache,
		jwksURL:       jwksURL,
		cacheDuration: duration,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}

	logger.Info("JWKS Manager initialized",
		"jwks_url", jwksURL,
		"cache_duration", duration,
	)

	return manager, nil
}

// GetKeySet возвращает актуальный набор ключей из кеша.
// Если кеш устарел - автоматически обновляет ключи из JWKS endpoint.
func (m *Manager) GetKeySet(ctx context.Context) (jwk.Set, error) {
	keySet, err := m.cache.Get(ctx, m.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}
	return keySet, nil
}

// Close останавливает автоматическое обновление кеша и освобождает ресурсы.
// Должен вызываться при остановке сервиса (например, в defer).
func (m *Manager) Close() error {
	m.logger.Info("Closing JWKS Manager", "jwks_url", m.jwksURL)
	m.cancel()
	return nil
}

// GetJWKSURL возвращает URL к JWKS endpoint.
func (m *Manager) GetJWKSURL() string {
	return m.jwksURL
}

// GetCacheDuration возвращает время кеширования ключей.
func (m *Manager) GetCacheDuration() time.Duration {
	return m.cacheDuration
}
