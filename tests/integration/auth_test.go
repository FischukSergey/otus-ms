//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// Адрес Auth-Proxy сервера (можно переопределить через TEST_AUTH_PROXY_URL)
	authProxyAddr = getAuthProxyAddr()

	// Тестовые credentials (должны быть созданы в Keycloak)
	testUsername = getEnvOrDefault("TEST_KEYCLOAK_USERNAME", "test@example.com")
	testPassword = getEnvOrDefault("TEST_KEYCLOAK_PASSWORD", "test123")
)

func getAuthProxyAddr() string {
	if addr := os.Getenv("TEST_AUTH_PROXY_URL"); addr != "" {
		return addr
	}
	return "http://localhost:38081"
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TokenResponse представляет ответ с токенами от Auth-Proxy.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// ErrorResponse представляет ответ с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}

func TestAuthProxyHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(authProxyAddr + "/health")
	require.NoError(t, err, "Failed to perform health check request")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200 OK")

	var health map[string]string
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err, "Failed to decode health response")

	assert.Equal(t, "ok", health["status"], "Health status should be 'ok'")
}

func TestLoginSuccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Подготавливаем запрос на логин
	loginReq := map[string]string{
		"username": testUsername,
		"password": testPassword,
	}
	body, err := json.Marshal(loginReq)
	require.NoError(t, err, "Failed to marshal login request")

	// Выполняем запрос
	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err, "Failed to perform login request")
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Login should return 200 OK")

	// Декодируем токены
	var tokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	require.NoError(t, err, "Failed to decode token response")

	// Проверяем наличие токенов
	assert.NotEmpty(t, tokens.AccessToken, "Access token should not be empty")
	assert.NotEmpty(t, tokens.RefreshToken, "Refresh token should not be empty")
	assert.Greater(t, tokens.ExpiresIn, 0, "ExpiresIn should be greater than 0")
	assert.Greater(t, tokens.RefreshExpiresIn, 0, "RefreshExpiresIn should be greater than 0")
	assert.Equal(t, "Bearer", tokens.TokenType, "TokenType should be 'Bearer'")
}

func TestLoginInvalidCredentials(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Подготавливаем запрос с неверными credentials
	loginReq := map[string]string{
		"username": testUsername,
		"password": "wrong-password",
	}
	body, err := json.Marshal(loginReq)
	require.NoError(t, err, "Failed to marshal login request")

	// Выполняем запрос
	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err, "Failed to perform login request")
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Login with invalid credentials should return 401")

	// Декодируем ошибку
	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	require.NoError(t, err, "Failed to decode error response")

	assert.NotEmpty(t, errResp.Error, "Error message should not be empty")
}

func TestLoginMissingFields(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	tests := []struct {
		name        string
		requestBody map[string]string
	}{
		{
			name:        "Missing username",
			requestBody: map[string]string{"password": "test123"},
		},
		{
			name:        "Missing password",
			requestBody: map[string]string{"username": testUsername},
		},
		{
			name:        "Empty request",
			requestBody: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err, "Failed to marshal request")

			resp, err := client.Post(
				authProxyAddr+"/api/v1/auth/login",
				"application/json",
				bytes.NewBuffer(body),
			)
			require.NoError(t, err, "Failed to perform login request")
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request")
		})
	}
}

func TestRefreshTokenSuccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Сначала логинимся, чтобы получить refresh token
	loginReq := map[string]string{
		"username": testUsername,
		"password": testPassword,
	}
	body, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Login should succeed")

	var loginTokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&loginTokens)
	require.NoError(t, err)
	require.NotEmpty(t, loginTokens.RefreshToken, "Should have refresh token")

	// Теперь используем refresh token для получения новых токенов
	refreshReq := map[string]string{
		"refresh_token": loginTokens.RefreshToken,
	}
	body, err = json.Marshal(refreshReq)
	require.NoError(t, err)

	resp, err = client.Post(
		authProxyAddr+"/api/v1/auth/refresh",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Refresh should return 200 OK")

	// Декодируем новые токены
	var newTokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&newTokens)
	require.NoError(t, err)

	// Проверяем наличие токенов
	assert.NotEmpty(t, newTokens.AccessToken, "New access token should not be empty")
	assert.NotEmpty(t, newTokens.RefreshToken, "New refresh token should not be empty")
}

func TestRefreshTokenInvalid(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Используем невалидный refresh token
	refreshReq := map[string]string{
		"refresh_token": "invalid-token",
	}
	body, err := json.Marshal(refreshReq)
	require.NoError(t, err)

	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/refresh",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Refresh with invalid token should return 401")
}

func TestLogoutSuccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Сначала логинимся
	loginReq := map[string]string{
		"username": testUsername,
		"password": testPassword,
	}
	body, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	require.NoError(t, err)

	// Теперь выполняем logout
	logoutReq := map[string]string{
		"refresh_token": tokens.RefreshToken,
	}
	body, err = json.Marshal(logoutReq)
	require.NoError(t, err)

	resp, err = client.Post(
		authProxyAddr+"/api/v1/auth/logout",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, "Logout should return 204 No Content")

	// Проверяем, что refresh token больше не работает
	refreshReq := map[string]string{
		"refresh_token": tokens.RefreshToken,
	}
	body, err = json.Marshal(refreshReq)
	require.NoError(t, err)

	resp, err = client.Post(
		authProxyAddr+"/api/v1/auth/refresh",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Refresh after logout should fail with 401")
}

func TestAuthFullFlow(t *testing.T) {
	// Комплексный тест: Login -> Refresh -> Logout
	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: Login
	t.Log("Step 1: Login")
	loginReq := map[string]string{
		"username": testUsername,
		"password": testPassword,
	}
	body, err := json.Marshal(loginReq)
	require.NoError(t, err)

	resp, err := client.Post(
		authProxyAddr+"/api/v1/auth/login",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	require.NoError(t, err)
	t.Logf("Login successful, got tokens")

	// Step 2: Wait a bit and refresh
	t.Log("Step 2: Refresh token")
	time.Sleep(2 * time.Second)

	refreshReq := map[string]string{
		"refresh_token": tokens.RefreshToken,
	}
	body, err = json.Marshal(refreshReq)
	require.NoError(t, err)

	resp, err = client.Post(
		authProxyAddr+"/api/v1/auth/refresh",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var newTokens TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&newTokens)
	require.NoError(t, err)
	t.Logf("Refresh successful, got new tokens")

	// Step 3: Logout
	t.Log("Step 3: Logout")
	logoutReq := map[string]string{
		"refresh_token": newTokens.RefreshToken,
	}
	body, err = json.Marshal(logoutReq)
	require.NoError(t, err)

	resp, err = client.Post(
		authProxyAddr+"/api/v1/auth/logout",
		"application/json",
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	t.Logf("Full auth flow completed successfully")
}
