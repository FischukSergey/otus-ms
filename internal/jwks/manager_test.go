package jwks_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/jwks"
)

func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Только ошибки в тестах
	}))
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name          string
		jwksURL       string
		cacheDuration int
		wantErr       bool
		errContains   string
	}{
		{
			name:          "Valid JWKS URL",
			jwksURL:       "https://keycloak.example.com/realms/test/protocol/openid-connect/certs",
			cacheDuration: 600,
			wantErr:       false,
		},
		{
			name:          "Empty JWKS URL",
			jwksURL:       "",
			cacheDuration: 600,
			wantErr:       true,
			errContains:   "jwks_url is required",
		},
		{
			name:          "Default cache duration",
			jwksURL:       "https://keycloak.example.com/realms/test/protocol/openid-connect/certs",
			cacheDuration: 0, // Должно установиться 10 минут
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := jwks.NewManager(tt.jwksURL, tt.cacheDuration, createTestLogger())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, manager)
			defer manager.Close()

			assert.Equal(t, tt.jwksURL, manager.GetJWKSURL())

			// Проверяем cache duration
			expectedDuration := time.Duration(tt.cacheDuration) * time.Second
			if tt.cacheDuration == 0 {
				expectedDuration = 10 * time.Minute
			}
			assert.Equal(t, expectedDuration, manager.GetCacheDuration())
		})
	}
}

func TestManager_GetKeySet_InvalidURL(t *testing.T) {
	// Arrange
	manager, err := jwks.NewManager(
		"https://invalid-keycloak-url.example.com/certs",
		600,
		createTestLogger(),
	)
	require.NoError(t, err)
	defer manager.Close()

	// Act
	ctx := context.Background()
	keySet, err := manager.GetKeySet(ctx)

	// Assert
	require.Error(t, err)
	assert.Nil(t, keySet)
	assert.Contains(t, err.Error(), "failed to get JWKS")
}

func TestManager_Close(t *testing.T) {
	// Arrange
	manager, err := jwks.NewManager(
		"https://keycloak.example.com/realms/test/protocol/openid-connect/certs",
		600,
		createTestLogger(),
	)
	require.NoError(t, err)

	// Act
	err = manager.Close()

	// Assert
	assert.NoError(t, err)
}

func TestManager_MultipleClose(t *testing.T) {
	// Arrange
	manager, err := jwks.NewManager(
		"https://keycloak.example.com/realms/test/protocol/openid-connect/certs",
		600,
		createTestLogger(),
	)
	require.NoError(t, err)

	// Act - вызываем Close несколько раз
	err1 := manager.Close()
	err2 := manager.Close()

	// Assert - должно быть безопасно
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	// Arrange
	manager, err := jwks.NewManager(
		"https://keycloak.example.com/realms/test/protocol/openid-connect/certs",
		600,
		createTestLogger(),
	)
	require.NoError(t, err)
	defer manager.Close()

	// Act - несколько горутин одновременно обращаются к менеджеру
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			ctx := context.Background()
			_, _ = manager.GetKeySet(ctx)
			done <- true
		}()
	}

	// Assert - все горутины должны завершиться
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for goroutines")
		}
	}
}
