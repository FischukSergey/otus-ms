package middleware_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/jwks"
	"github.com/FischukSergey/otus-ms/internal/middleware"
)

// Тестовый секретный ключ для подписи JWT.
const testSecret = "test-secret"

// createTestJWT создаёт тестовый JWT токен.
func createTestJWT(claims *middleware.JWTClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(testSecret))
}

// createTestLogger создаёт тестовый логгер.
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Скрываем debug логи в тестах
	}))
}

func TestValidateJWT_ValidToken(t *testing.T) {
	// Arrange
	claims := &middleware.JWTClaims{
		Sub:        "test-user-uuid",
		Email:      "test@example.com",
		GivenName:  "Test",
		FamilyName: "User",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "http://localhost:8080/realms/test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true, // Пропускаем проверку подписи для тестов
		Issuer:     "http://localhost:8080/realms/test",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем, что claims добавлены в контекст
		userID, ok := middleware.GetUserIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "test-user-uuid", userID)

		email, ok := middleware.GetEmailFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "test@example.com", email)

		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestValidateJWT_ValidToken_WithoutBearerPrefix(t *testing.T) {
	// Проверяем, что токен без префикса "Bearer " тоже принимается (удобно для Swagger UI и др.)
	claims := &middleware.JWTClaims{
		Sub:        "test-user-uuid",
		Email:      "test@example.com",
		GivenName:  "Test",
		FamilyName: "User",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "http://localhost:8080/realms/test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true,
		Issuer:     "http://localhost:8080/realms/test",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", token) // без "Bearer "

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestValidateJWT_MissingAuthorizationHeader(t *testing.T) {
	// Arrange
	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Missing authorization header")
}

func TestValidateJWT_InvalidAuthorizationFormat(t *testing.T) {
	// Arrange
	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	testCases := []struct {
		name   string
		header string
	}{
		{"No Bearer prefix", "some-token"},
		{"Wrong prefix", "Basic some-token"},
		{"Empty token", "Bearer "},
		{"Multiple spaces", "Bearer  token"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tc.header)

			rr := httptest.NewRecorder()
			mw(handler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	// Arrange
	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid token")
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	// Arrange
	claims := &middleware.JWTClaims{
		Sub:   "test-user-uuid",
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Истёкший токен
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestValidateJWT_InvalidIssuer(t *testing.T) {
	// Arrange
	claims := &middleware.JWTClaims{
		Sub:   "test-user-uuid",
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "http://wrong-issuer.com",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true, // Используем тестовый режим
		Issuer:     "http://localhost:8080/realms/test",
	}

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid token issuer")
}

func TestValidateJWT_ClaimsInContext(t *testing.T) {
	// Arrange
	claims := &middleware.JWTClaims{
		Sub:        "test-user-uuid",
		Email:      "test@example.com",
		GivenName:  "Test",
		FamilyName: "User",
		Name:       "Test User",
		RealmAccess: struct {
			Roles []string `json:"roles"`
		}{
			Roles: []string{"user", "admin"},
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем извлечение claims из контекста
		extractedClaims, ok := middleware.GetClaimsFromContext(r.Context())
		assert.True(t, ok, "claims should be in context")

		assert.Equal(t, "test-user-uuid", extractedClaims.GetUserID())
		assert.Equal(t, "test@example.com", extractedClaims.Email)
		assert.Equal(t, "Test", extractedClaims.GivenName)
		assert.Equal(t, "User", extractedClaims.FamilyName)
		assert.Equal(t, "Test User", extractedClaims.GetFullName())
		assert.True(t, extractedClaims.HasRole("admin"))
		assert.True(t, extractedClaims.HasRole("user"))
		assert.False(t, extractedClaims.HasRole("superadmin"))

		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTClaims_GetFullName(t *testing.T) {
	testCases := []struct {
		name     string
		claims   middleware.JWTClaims
		expected string
	}{
		{
			name: "Name field present",
			claims: middleware.JWTClaims{
				Name:       "John Doe",
				GivenName:  "John",
				FamilyName: "Doe",
			},
			expected: "John Doe",
		},
		{
			name: "Name field empty, use GivenName and FamilyName",
			claims: middleware.JWTClaims{
				Name:       "",
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
			expected: "Jane Smith",
		},
		{
			name: "Only GivenName",
			claims: middleware.JWTClaims{
				GivenName:  "Alice",
				FamilyName: "",
			},
			expected: "Alice ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.claims.GetFullName()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestJWTClaims_HasRole(t *testing.T) {
	claims := middleware.JWTClaims{
		RealmAccess: struct {
			Roles []string `json:"roles"`
		}{
			Roles: []string{"user", "admin", "moderator"},
		},
	}

	assert.True(t, claims.HasRole("user"))
	assert.True(t, claims.HasRole("admin"))
	assert.True(t, claims.HasRole("moderator"))
	assert.False(t, claims.HasRole("superadmin"))
	assert.False(t, claims.HasRole(""))
}

// Benchmark тесты.
func BenchmarkValidateJWT(b *testing.B) {
	claims := &middleware.JWTClaims{
		Sub:   "test-user-uuid",
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	token, _ := createTestJWT(claims)

	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		mw(handler).ServeHTTP(rr, req)
	}
}

// Example тесты для документации.
func ExampleValidateJWT() {
	// Создаём JWKS Manager
	jwksManager, _ := jwks.NewManager(
		"https://keycloak.example.com/realms/otus-realm/protocol/openid-connect/certs",
		600,
		createTestLogger(),
	)
	defer func() { _ = jwksManager.Close() }()

	// Создаём конфигурацию JWT
	config := middleware.JWTConfig{
		Issuer:   "http://localhost:8080/realms/otus-realm",
		Audience: "otus-client",
	}

	// Создаём middleware
	jwtMiddleware := middleware.ValidateJWT(config, jwksManager, createTestLogger())

	// Применяем к handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := middleware.GetUserIDFromContext(r.Context())
		_, _ = fmt.Fprintf(w, "User ID: %s", userID)
	})

	// Используем middleware
	http.Handle("/api/protected", jwtMiddleware(handler))
	// Output:
}

// Интеграционный тест с реальным контекстом.
func TestValidateJWT_Integration(t *testing.T) {
	// Arrange
	claims := &middleware.JWTClaims{
		Sub:        "550e8400-e29b-41d4-a716-446655440000",
		Email:      "integration@example.com",
		GivenName:  "Integration",
		FamilyName: "Test",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "http://localhost:8080/realms/otus-realm",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true,
		Issuer:     "http://localhost:8080/realms/otus-realm",
	}

	// Цепочка middleware
	var capturedUserID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserIDFromContext(r.Context())
		assert.True(t, ok, "user_id should be in context")
		capturedUserID = userID
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Act
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mw(handler).ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", capturedUserID)
}

// Тест конкурентного доступа.
func TestValidateJWT_Concurrent(t *testing.T) {
	claims := &middleware.JWTClaims{
		Sub:   "test-user-uuid",
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	token, err := createTestJWT(claims)
	require.NoError(t, err)

	config := middleware.JWTConfig{
		SkipVerify: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.ValidateJWT(config, nil, createTestLogger())

	// Запускаем 100 конкурентных запросов
	const numRequests = 100
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()
			mw(handler).ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			done <- true
		}()
	}

	// Ждём завершения всех запросов
	for i := 0; i < numRequests; i++ {
		<-done
	}
}
