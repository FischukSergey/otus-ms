package mainservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client HTTP клиент для взаимодействия с Main Service API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient создаёт новый клиент для Main Service.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
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
// Использует JWT токен для авторизации (должен иметь роль admin).
func (c *Client) CreateUser(ctx context.Context, jwtToken string, req CreateUserRequest) error {
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
	if jwtToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+jwtToken)
	}

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
