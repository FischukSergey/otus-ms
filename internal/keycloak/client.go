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
