// Package keycloak реализует клиент для взаимодействия с Keycloak через gocloak.
package keycloak

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
)

// Client представляет клиент для работы с Keycloak.
type Client struct {
	gocloak      *gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string
}

// NewClient создаёт новый клиент для работы с Keycloak.
func NewClient(keycloakURL, realm, clientID, clientSecret string) *Client {
	return &Client{
		gocloak:      gocloak.NewClient(keycloakURL),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Login выполняет аутентификацию пользователя через Password Grant Flow.
// Возвращает access token и refresh token.
func (c *Client) Login(ctx context.Context, username, password string) (*TokenResponse, error) {
	token, err := c.gocloak.Login(
		ctx,
		c.clientID,
		c.clientSecret,
		c.realm,
		username,
		password,
	)
	if err != nil {
		return nil, fmt.Errorf("keycloak login failed: %w", err)
	}

	return &TokenResponse{
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		ExpiresIn:        token.ExpiresIn,
		RefreshExpiresIn: token.RefreshExpiresIn,
		TokenType:        token.TokenType,
		Scope:            token.Scope,
	}, nil
}

// RefreshToken обновляет access token используя refresh token.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	token, err := c.gocloak.RefreshToken(
		ctx,
		refreshToken,
		c.clientID,
		c.clientSecret,
		c.realm,
	)
	if err != nil {
		return nil, fmt.Errorf("keycloak refresh token failed: %w", err)
	}

	return &TokenResponse{
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		ExpiresIn:        token.ExpiresIn,
		RefreshExpiresIn: token.RefreshExpiresIn,
		TokenType:        token.TokenType,
		Scope:            token.Scope,
	}, nil
}

// Logout выполняет logout пользователя, инвалидируя refresh token.
func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	err := c.gocloak.Logout(
		ctx,
		c.clientID,
		c.clientSecret,
		c.realm,
		refreshToken,
	)
	if err != nil {
		return fmt.Errorf("keycloak logout failed: %w", err)
	}

	return nil
}

// CreateUser создаёт нового пользователя в Keycloak с ролью user.
// Требует service account с ролями manage-users и view-users.
// Возвращает UUID созданного пользователя.
func (c *Client) CreateUser(ctx context.Context, user User) (string, error) {
	// 1. Получаем admin токен через service account (client credentials flow)
	token, err := c.gocloak.LoginClient(ctx, c.clientID, c.clientSecret, c.realm)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}

	// 2. Подготавливаем данные пользователя для Keycloak
	enabled := true
	emailVerified := true
	kcUser := gocloak.User{
		Username:      gocloak.StringP(user.Email), // Username = Email
		Email:         gocloak.StringP(user.Email),
		EmailVerified: &emailVerified,
		FirstName:     gocloak.StringP(user.FirstName),
		LastName:      gocloak.StringP(user.LastName),
		Enabled:       &enabled,
	}

	// 3. Создаём пользователя в Keycloak
	userID, err := c.gocloak.CreateUser(ctx, token.AccessToken, c.realm, kcUser)
	if err != nil {
		return "", fmt.Errorf("failed to create user in keycloak: %w", err)
	}

	// 4. Устанавливаем пароль (permanent, не temporary)
	err = c.gocloak.SetPassword(ctx, token.AccessToken, userID, c.realm, user.Password, false)
	if err != nil {
		// Rollback: удаляем пользователя если не удалось установить пароль
		_ = c.gocloak.DeleteUser(ctx, token.AccessToken, c.realm, userID)
		return "", fmt.Errorf("failed to set password: %w", err)
	}

	// 5. Назначаем роль "user"
	// Получаем realm role "user"
	role, err := c.gocloak.GetRealmRole(ctx, token.AccessToken, c.realm, "user")
	if err == nil && role != nil && role.ID != nil {
		// Назначаем роль пользователю
		roles := []gocloak.Role{*role}
		err = c.gocloak.AddRealmRoleToUser(ctx, token.AccessToken, c.realm, userID, roles)
		if err != nil {
			// Логируем ошибку, но НЕ откатываем создание пользователя
			// Администратор может назначить роль вручную
			// В production окружении лучше использовать default roles в Keycloak
			return userID, fmt.Errorf("user created but role assignment failed: %w", err)
		}
	}

	return userID, nil
}

// DeleteUser удаляет пользователя из Keycloak.
// Используется для rollback при ошибках регистрации.
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	// Получаем admin токен
	token, err := c.gocloak.LoginClient(ctx, c.clientID, c.clientSecret, c.realm)
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	// Удаляем пользователя
	err = c.gocloak.DeleteUser(ctx, token.AccessToken, c.realm, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user from keycloak: %w", err)
	}

	return nil
}

// GetServiceAccountToken получает JWT токен для service account через Client Credentials Flow.
// Используется для service-to-service аутентификации (например, Auth-Proxy -> Main Service).
// Токен содержит роль service-account и проверяется через JWKS в Main Service.
func (c *Client) GetServiceAccountToken(ctx context.Context) (string, error) {
	token, err := c.gocloak.LoginClient(ctx, c.clientID, c.clientSecret, c.realm)
	if err != nil {
		return "", fmt.Errorf("failed to get service account token: %w", err)
	}

	return token.AccessToken, nil
}
