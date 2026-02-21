package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireRole(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name           string
		userRoles      []string
		requiredRoles  []string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "admin has access to admin route",
			userRoles:      []string{"admin"},
			requiredRoles:  []string{"admin"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "user denied from admin route",
			userRoles:      []string{"user"},
			requiredRoles:  []string{"admin"},
			expectedStatus: http.StatusForbidden,
			expectedBody:   `{"error":"Access denied - insufficient permissions"}`,
		},
		{
			name:           "user has access to user route",
			userRoles:      []string{"user"},
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "admin has access to user route",
			userRoles:      []string{"admin", "user"},
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "user or admin - user passes",
			userRoles:      []string{"user"},
			requiredRoles:  []string{"user", "admin"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "user or admin - admin passes",
			userRoles:      []string{"admin"},
			requiredRoles:  []string{"user", "admin"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "no matching roles",
			userRoles:      []string{"guest"},
			requiredRoles:  []string{"user", "admin"},
			expectedStatus: http.StatusForbidden,
			expectedBody:   `{"error":"Access denied - insufficient permissions"}`,
		},
		{
			name:           "empty user roles",
			userRoles:      []string{},
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusForbidden,
			expectedBody:   `{"error":"Access denied - insufficient permissions"}`,
		},
		{
			name:           "multiple user roles, one matches",
			userRoles:      []string{"guest", "user", "viewer"},
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаём claims с нужными ролями
			claims := &JWTClaims{
				Sub:   "test-user-id",
				Email: "test@example.com",
			}
			claims.RealmAccess.Roles = tt.userRoles

			// Создаём контекст с claims
			ctx := context.WithValue(context.Background(), ContextKeyClaims, claims)
			req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
			rec := httptest.NewRecorder()

			// Создаём test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("success"))
			})

			// Применяем RBAC middleware
			handler := RequireRole(tt.requiredRoles, logger)(testHandler)

			// Выполняем запрос
			handler.ServeHTTP(rec, req)

			// Проверяем статус
			assert.Equal(t, tt.expectedStatus, rec.Code, "unexpected status code")

			// Проверяем тело ответа
			assert.Equal(t, tt.expectedBody, rec.Body.String(), "unexpected response body")
		})
	}
}

func TestRequireRole_MissingClaims(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Создаём запрос БЕЗ claims в контексте
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Создаём test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Применяем RBAC middleware
	handler := RequireRole([]string{"user"}, logger)(testHandler)

	// Выполняем запрос
	handler.ServeHTTP(rec, req)

	// Проверяем что получили 401 (claims отсутствуют)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "should return 401 when claims missing")
	assert.Contains(t, rec.Body.String(), "Authentication required", "should indicate missing authentication")
}

func TestRequireAdmin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name           string
		userRoles      []string
		expectedStatus int
	}{
		{
			name:           "admin has access",
			userRoles:      []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user denied",
			userRoles:      []string{"user"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "admin with multiple roles has access",
			userRoles:      []string{"user", "admin", "viewer"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &JWTClaims{
				Sub:   "test-user-id",
				Email: "test@example.com",
			}
			claims.RealmAccess.Roles = tt.userRoles

			ctx := context.WithValue(context.Background(), ContextKeyClaims, claims)
			req := httptest.NewRequest("GET", "/admin/test", nil).WithContext(ctx)
			rec := httptest.NewRecorder()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := RequireAdmin(logger)(testHandler)
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestRequireUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name           string
		userRoles      []string
		expectedStatus int
	}{
		{
			name:           "user has access",
			userRoles:      []string{"user"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "admin has access",
			userRoles:      []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "both user and admin have access",
			userRoles:      []string{"user", "admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "guest denied",
			userRoles:      []string{"guest"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "no roles denied",
			userRoles:      []string{},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &JWTClaims{
				Sub:   "test-user-id",
				Email: "test@example.com",
			}
			claims.RealmAccess.Roles = tt.userRoles

			ctx := context.WithValue(context.Background(), ContextKeyClaims, claims)
			req := httptest.NewRequest("GET", "/user/test", nil).WithContext(ctx)
			rec := httptest.NewRecorder()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := RequireUser(logger)(testHandler)
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
