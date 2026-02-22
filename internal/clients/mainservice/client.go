package mainservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// KeycloakTokenProvider определяет интерфейс для получения service account токена от Keycloak.
type KeycloakTokenProvider interface {
	GetServiceAccountToken(ctx context.Context) (string, error)
}

// Client HTTP клиент для взаимодействия с Main Service API.
type Client struct {
	baseURL        string
	httpClient     *http.Client
	keycloakClient KeycloakTokenProvider
}

// NewClient создаёт новый клиент для Main Service.
func NewClient(baseURL string, keycloakClient KeycloakTokenProvider) *Client {
	return &Client{
		baseURL:        baseURL,
		keycloakClient: keycloakClient,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateUserRequest представляет запрос на создание пользователя в Main Service.
type CreateUserRequest struct {
	UUID       string  `json:"uuid"`
	Email      string  `json:"email"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	MiddleName *string `json:"middleName,omitempty"`
}

// CreateUser создаёт пользователя в Main Service.
// Получает JWT токен service account от Keycloak и использует его для авторизации.
// Токен должен иметь роль service-account.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) error {
	// Получаем service account токен от Keycloak
	jwtToken, err := c.keycloakClient.GetServiceAccountToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get service account token: %w", err)
	}

	// Сериализуем request body
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаём HTTP запрос
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/users",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Устанавливаем заголовки
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+jwtToken)

	// Отправляем запрос
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код
	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)

		if errResp.Error != "" {
			return fmt.Errorf("main service error (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
