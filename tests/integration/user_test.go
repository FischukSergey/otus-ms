//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// Адрес API сервера (можно переопределить через TEST_SERVER_URL)
	// По умолчанию: docker-compose на порту 8081
	// В CI: localhost:8080
	testServerAddr = getServerAddr()

	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func getServerAddr() string {
	if addr := os.Getenv("TEST_SERVER_URL"); addr != "" {
		return addr
	}
	return "http://localhost:8081"
}

func TestUserBasicFlow(t *testing.T) {
	// Генерируем тестовые данные
	testUUID := uuid.New().String()
	testEmail := fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])

	// Получаем admin токен для тестов
	adminToken := GenerateAdminToken()

	t.Run("Create User", func(t *testing.T) {
		// Подготавливаем запрос на создание пользователя
		createReq := map[string]interface{}{
			"uuid":      testUUID,
			"email":     testEmail,
			"firstName": "Иван",
			"lastName":  "Иванов",
		}

		body, err := json.Marshal(createReq)
		require.NoError(t, err)

		// Создаем запрос с JWT токеном
		url := fmt.Sprintf("%s/api/v1/users", testServerAddr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		// Отправляем запрос
		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Проверяем статус код
		assert.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")
	})

	t.Run("Get User", func(t *testing.T) {
		// Получаем созданного пользователя
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Проверяем статус код
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

		// Парсим ответ
		var user map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&user)
		require.NoError(t, err)

		// Проверяем данные
		assert.Equal(t, testUUID, user["uuid"])
		assert.Equal(t, testEmail, user["email"])
		assert.Equal(t, "Иван", user["firstName"])
		assert.Equal(t, "Иванов", user["lastName"])
		assert.False(t, user["deleted"].(bool), "User should not be deleted")
	})

	t.Run("Delete User", func(t *testing.T) {
		// Удаляем пользователя
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Проверяем статус код
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, "Expected 204 No Content")
	})

	t.Run("Get Deleted User Shows Deleted Flag", func(t *testing.T) {
		// Получаем удаленного пользователя
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Пользователь должен возвращаться с кодом 200
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Deleted user should still be accessible")

		// Парсим ответ
		var user map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&user)
		require.NoError(t, err)

		// Проверяем что флаг deleted установлен
		assert.True(t, user["deleted"].(bool), "User should be marked as deleted")
		assert.NotNil(t, user["deletedAt"], "DeletedAt timestamp should be set")
	})
}

func TestUserValidation(t *testing.T) {
	adminToken := GenerateAdminToken()

	t.Run("Create User With Invalid UUID", func(t *testing.T) {
		createReq := map[string]interface{}{
			"uuid":  "invalid-uuid",
			"email": "test@example.com",
		}

		body, err := json.Marshal(createReq)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/users", testServerAddr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Должны получить ошибку валидации
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for invalid UUID")
	})

	t.Run("Create User With Invalid Email", func(t *testing.T) {
		createReq := map[string]interface{}{
			"uuid":  uuid.New().String(),
			"email": "not-an-email",
		}

		body, err := json.Marshal(createReq)
		require.NoError(t, err)

		url := fmt.Sprintf("%s/api/v1/users", testServerAddr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Должны получить ошибку валидации
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for invalid email")
	})

	t.Run("Get User With Invalid UUID", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/users/invalid-uuid", testServerAddr)
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Должны получить ошибку валидации
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for invalid UUID")
	})
}

func TestHealthCheck(t *testing.T) {
	url := fmt.Sprintf("%s/health", testServerAddr)
	resp, err := httpClient.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200 OK")

	var healthResp map[string]string
	err = json.NewDecoder(resp.Body).Decode(&healthResp)
	require.NoError(t, err)

	assert.Equal(t, "ok", healthResp["status"])
	assert.NotEmpty(t, healthResp["time"])
}

// TestRBAC проверяет систему контроля доступа на основе ролей.
func TestRBAC(t *testing.T) {
	adminToken := GenerateAdminToken()
	userToken := GenerateUserToken()
	testUUID := uuid.New().String()

	t.Run("Admin Can Create User", func(t *testing.T) {
		createReq := map[string]interface{}{
			"uuid":      testUUID,
			"email":     fmt.Sprintf("rbac-test-%s@example.com", uuid.New().String()[:8]),
			"firstName": "RBAC",
			"lastName":  "Test",
		}

		body, _ := json.Marshal(createReq)
		url := fmt.Sprintf("%s/api/v1/users", testServerAddr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "Admin should be able to create user")
	})

	t.Run("User Cannot Create User", func(t *testing.T) {
		createReq := map[string]interface{}{
			"uuid":      uuid.New().String(),
			"email":     "another@example.com",
			"firstName": "Test",
			"lastName":  "User",
		}

		body, _ := json.Marshal(createReq)
		url := fmt.Sprintf("%s/api/v1/users", testServerAddr)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Пользователь без роли admin должен получить 403 Forbidden
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "User without admin role should get 403")

		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "Access denied", "Error should mention access denied")
	})

	t.Run("User Cannot Get Other Users", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+userToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "User should not be able to get other users")
	})

	t.Run("User Cannot Delete Users", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest("DELETE", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+userToken)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "User should not be able to delete users")
	})

	t.Run("No Token Returns 401", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		resp, err := httpClient.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Request without token should get 401")
	})

	t.Run("Invalid Token Returns 401", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/users/%s", testServerAddr, testUUID)
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Invalid token should get 401")
	})
}
