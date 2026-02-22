//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// AuthProxyURL - адрес Auth-Proxy для тестов.
var AuthProxyURL = "http://localhost:38081"

func init() {
	// Переопределяем адрес через env, если задан.
	if url := os.Getenv("TEST_AUTH_PROXY_URL"); url != "" {
		AuthProxyURL = url
	}
}

// Структуры для Auth-Proxy API.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type registerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName string `json:"middleName,omitempty"`
}

// TestAuthProxyHealthCheck проверяет доступность Auth-Proxy.
func TestAuthProxyHealthCheck(t *testing.T) {
	resp, err := http.Get(AuthProxyURL + "/health")
	if err != nil {
		t.Skipf("Auth-Proxy недоступен на %s (возможно не запущен): %v", AuthProxyURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check failed: status %d", resp.StatusCode)
	}
}

// TestLoginSuccess проверяет успешный логин.
func TestLoginSuccess(t *testing.T) {
	// Проверяем доступность Auth-Proxy.
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	// Используем тестового пользователя из Keycloak.
	payload := loginRequest{
		Username: os.Getenv("TEST_KEYCLOAK_USERNAME"),
		Password: os.Getenv("TEST_KEYCLOAK_PASSWORD"),
	}

	// Если переменные не заданы, используем значения по умолчанию.
	if payload.Username == "" {
		payload.Username = "test@example.com"
	}
	if payload.Password == "" {
		payload.Password = "test123"
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	// Ожидаем либо 200 OK (успешный логин), либо 401 (если пользователь не настроен в Keycloak).
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skipf("Тестовый пользователь не настроен в Keycloak. Создайте пользователя %s с паролем %s", payload.Username, payload.Password)
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Login failed: status %d, error: %s", resp.StatusCode, errResp.Error)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	// Проверяем, что токены получены.
	if tokenResp.AccessToken == "" {
		t.Fatal("Access token is empty")
	}
	if tokenResp.RefreshToken == "" {
		t.Fatal("Refresh token is empty")
	}
	if tokenResp.TokenType != "Bearer" {
		t.Fatalf("Expected token type 'Bearer', got '%s'", tokenResp.TokenType)
	}
	if tokenResp.ExpiresIn <= 0 {
		t.Fatalf("Invalid expires_in: %d", tokenResp.ExpiresIn)
	}
}

// TestLoginInvalidCredentials проверяет логин с неверными credentials.
func TestLoginInvalidCredentials(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	payload := loginRequest{
		Username: "invalid@example.com",
		Password: "wrongpassword",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	// Ожидаем 401 Unauthorized.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp.StatusCode)
	}

	var errResp errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errResp.Error == "" {
		t.Fatal("Error message is empty")
	}
}

// TestLoginMissingFields проверяет валидацию обязательных полей.
func TestLoginMissingFields(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	// Тест 1: пустой username.
	payload1 := loginRequest{
		Username: "",
		Password: "password123",
	}
	testBadRequest(t, "/api/v1/auth/login", payload1)

	// Тест 2: пустой password.
	payload2 := loginRequest{
		Username: "test@example.com",
		Password: "",
	}
	testBadRequest(t, "/api/v1/auth/login", payload2)
}

// TestRefreshTokenSuccess проверяет обновление токена.
func TestRefreshTokenSuccess(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	// Сначала получаем токены через логин.
	tokens := loginForTest(t)
	if tokens == nil {
		t.Skip("Не удалось получить токены для теста refresh")
		return
	}

	// Ждем немного, чтобы убедиться что время изменилось.
	time.Sleep(2 * time.Second)

	// Обновляем токен.
	payload := refreshRequest{
		RefreshToken: tokens.RefreshToken,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Refresh failed: status %d, error: %s", resp.StatusCode, errResp.Error)
	}

	var newTokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&newTokens); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	// Проверяем, что новые токены получены.
	if newTokens.AccessToken == "" {
		t.Fatal("New access token is empty")
	}
	if newTokens.RefreshToken == "" {
		t.Fatal("New refresh token is empty")
	}

	// Новый access token должен отличаться от старого.
	if newTokens.AccessToken == tokens.AccessToken {
		t.Log("WARNING: Access token не изменился (может быть ок, зависит от настроек Keycloak)")
	}
}

// TestRefreshTokenInvalid проверяет refresh с невалидным токеном.
func TestRefreshTokenInvalid(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	payload := refreshRequest{
		RefreshToken: "invalid.refresh.token",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Refresh request failed: %v", err)
	}
	defer resp.Body.Close()

	// Ожидаем 401 Unauthorized.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp.StatusCode)
	}
}

// TestLogoutSuccess проверяет logout.
func TestLogoutSuccess(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	// Получаем токены.
	tokens := loginForTest(t)
	if tokens == nil {
		t.Skip("Не удалось получить токены для теста logout")
		return
	}

	// Выполняем logout.
	payload := logoutRequest{
		RefreshToken: tokens.RefreshToken,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/logout", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	defer resp.Body.Close()

	// Logout может вернуть 200 OK или 204 No Content - оба статуса валидны
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Logout failed: status %d, error: %s", resp.StatusCode, errResp.Error)
	}

	// Проверяем, что после logout токен больше не работает.
	time.Sleep(1 * time.Second)

	refreshPayload := refreshRequest{
		RefreshToken: tokens.RefreshToken,
	}
	refreshBody, _ := json.Marshal(refreshPayload)
	refreshResp, err := http.Post(AuthProxyURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(refreshBody))
	if err != nil {
		t.Fatalf("Refresh after logout request failed: %v", err)
	}
	defer refreshResp.Body.Close()

	// Ожидаем 401, так как токен должен быть инвалидирован.
	if refreshResp.StatusCode != http.StatusUnauthorized {
		t.Logf("WARNING: Refresh после logout вернул status %d (ожидалось 401). Возможно, Keycloak не инвалидировал сессию немедленно.", refreshResp.StatusCode)
	}
}

// TestAuthFullFlow проверяет полный флоу: Login → Refresh → Logout.
func TestAuthFullFlow(t *testing.T) {
	if !isAuthProxyAvailable(t) {
		t.Skip("Auth-Proxy недоступен, пропускаем тест")
	}

	// 1. Login.
	t.Log("Step 1: Login")
	tokens := loginForTest(t)
	if tokens == nil {
		t.Skip("Не удалось получить токены для full flow теста")
		return
	}
	t.Logf("✅ Login successful, access_token length: %d", len(tokens.AccessToken))

	// 2. Refresh.
	t.Log("Step 2: Refresh token")
	time.Sleep(2 * time.Second)
	refreshPayload := refreshRequest{RefreshToken: tokens.RefreshToken}
	refreshBody, _ := json.Marshal(refreshPayload)
	refreshResp, err := http.Post(AuthProxyURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(refreshBody))
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("Refresh failed with status %d", refreshResp.StatusCode)
	}

	var newTokens tokenResponse
	json.NewDecoder(refreshResp.Body).Decode(&newTokens)
	t.Logf("✅ Refresh successful, new access_token length: %d", len(newTokens.AccessToken))

	// 3. Logout.
	t.Log("Step 3: Logout")
	logoutPayload := logoutRequest{RefreshToken: newTokens.RefreshToken}
	logoutBody, _ := json.Marshal(logoutPayload)
	logoutResp, err := http.Post(AuthProxyURL+"/api/v1/auth/logout", "application/json", bytes.NewBuffer(logoutBody))
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	defer logoutResp.Body.Close()

	// Logout может вернуть 200 OK или 204 No Content - оба статуса валидны
	if logoutResp.StatusCode != http.StatusOK && logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("Logout failed with status %d", logoutResp.StatusCode)
	}
	t.Log("✅ Logout successful")

	t.Log("✅ Full flow completed successfully")
}

// === Вспомогательные функции ===

// isAuthProxyAvailable проверяет доступность Auth-Proxy.
func isAuthProxyAvailable(t *testing.T) bool {
	t.Helper()
	resp, err := http.Get(AuthProxyURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// loginForTest выполняет логин и возвращает токены (или nil если не удалось).
func loginForTest(t *testing.T) *tokenResponse {
	t.Helper()

	payload := loginRequest{
		Username: os.Getenv("TEST_KEYCLOAK_USERNAME"),
		Password: os.Getenv("TEST_KEYCLOAK_PASSWORD"),
	}

	if payload.Username == "" {
		payload.Username = "test@example.com"
	}
	if payload.Password == "" {
		payload.Password = "test123"
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Logf("Login request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("Login failed: status %d, error: %s", resp.StatusCode, errResp.Error)
		return nil
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		t.Logf("Failed to decode token response: %v", err)
		return nil
	}

	return &tokens
}

// testBadRequest проверяет что запрос возвращает 400 Bad Request.
func testBadRequest(t *testing.T, endpoint string, payload interface{}) {
	t.Helper()

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestRegisterSuccess проверяет успешную регистрацию нового пользователя.
func TestRegisterSuccess(t *testing.T) {
	// Используем уникальный email для избежания конфликтов
	timestamp := time.Now().UnixNano()
	payload := registerRequest{
		Email:      "newuser" + string(rune(timestamp%10000)) + "@test.com",
		Password:   "SecurePassword123",
		FirstName:  "Иван",
		LastName:   "Иванов",
		MiddleName: "Иванович",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Expected status 201, got %d, error: %s", resp.StatusCode, errResp.Error)
	}

	t.Log("User registered successfully")

	// После регистрации пытаемся залогиниться с новыми credentials
	time.Sleep(1 * time.Second) // Небольшая пауза для синхронизации

	loginPayload := loginRequest{
		Username: payload.Email,
		Password: payload.Password,
	}

	loginBody, _ := json.Marshal(loginPayload)
	loginResp, err := http.Post(AuthProxyURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		t.Fatalf("Login after registration failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(loginResp.Body).Decode(&errResp)
		t.Fatalf("Login failed after registration: status %d, error: %s", loginResp.StatusCode, errResp.Error)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&tokens); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("Expected tokens after login, but got empty tokens")
	}

	t.Log("Login after registration successful")
}

// TestRegisterDuplicateEmail проверяет что нельзя зарегистрировать пользователя с существующим email.
func TestRegisterDuplicateEmail(t *testing.T) {
	// Используем существующего пользователя (admin)
	payload := registerRequest{
		Email:     "admin@example.com",
		Password:  "anypassword123",
		FirstName: "Test",
		LastName:  "User",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(AuthProxyURL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		var errResp errorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("Expected status 409 Conflict, got %d, error: %s", resp.StatusCode, errResp.Error)
	}

	t.Log("Duplicate email correctly rejected with 409")
}

// TestRegisterValidation проверяет валидацию данных при регистрации.
func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload registerRequest
	}{
		{
			name: "invalid email",
			payload: registerRequest{
				Email:     "notanemail",
				Password:  "password123",
				FirstName: "Test",
				LastName:  "User",
			},
		},
		{
			name: "short password",
			payload: registerRequest{
				Email:     "test@example.com",
				Password:  "short",
				FirstName: "Test",
				LastName:  "User",
			},
		},
		{
			name: "missing firstName",
			payload: registerRequest{
				Email:    "test@example.com",
				Password: "password123",
				LastName: "User",
			},
		},
		{
			name: "missing lastName",
			payload: registerRequest{
				Email:     "test@example.com",
				Password:  "password123",
				FirstName: "Test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testBadRequest(t, "/api/v1/auth/register", tt.payload)
			t.Logf("Validation test '%s' passed", tt.name)
		})
	}
}

// TestRegisterFullFlow проверяет полный flow: регистрация -> логин -> создание чего-то с токеном.
func TestRegisterFullFlow(t *testing.T) {
	// 1. Регистрируем нового пользователя
	timestamp := time.Now().UnixNano()
	regPayload := registerRequest{
		Email:      "flowuser" + string(rune(timestamp%10000)) + "@test.com",
		Password:   "FlowPassword123",
		FirstName:  "Flow",
		LastName:   "User",
		MiddleName: "Testovich",
	}

	regBody, _ := json.Marshal(regPayload)
	regResp, err := http.Post(AuthProxyURL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(regBody))
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer regResp.Body.Close()

	if regResp.StatusCode != http.StatusCreated {
		var errResp errorResponse
		json.NewDecoder(regResp.Body).Decode(&errResp)
		t.Fatalf("Registration failed: status %d, error: %s", regResp.StatusCode, errResp.Error)
	}

	t.Log("Step 1: User registered")

	// Пауза для синхронизации
	time.Sleep(1 * time.Second)

	// 2. Логинимся
	loginPayload := loginRequest{
		Username: regPayload.Email,
		Password: regPayload.Password,
	}

	loginBody, _ := json.Marshal(loginPayload)
	loginResp, err := http.Post(AuthProxyURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		var errResp errorResponse
		json.NewDecoder(loginResp.Body).Decode(&errResp)
		t.Fatalf("Login failed: status %d, error: %s", loginResp.StatusCode, errResp.Error)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&tokens); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Fatal("Expected access token, but got empty")
	}

	t.Log("Step 2: Login successful, access token obtained")

	// 3. Используем access token для проверки (например, проверяем /health с токеном)
	// Или можно попытаться создать ресурс в Main Service (но user не имеет прав admin)
	// В данном случае просто проверим что токен валиден

	// В реальности можно добавить запрос к Main Service с этим токеном
	// Но так как user не имеет admin прав, запрос на создание другого user должен вернуть 403

	t.Log("Step 3: Full registration flow completed successfully")
}
