package middleware

import "github.com/golang-jwt/jwt/v5"

// JWTClaims представляет структуру claims из Keycloak JWT токена.
// Keycloak использует snake_case для имён полей в JWT, поэтому используем соответствующие теги.
//
//nolint:tagliatelle // Keycloak JWT uses snake_case field names
type JWTClaims struct {
	Sub               string `json:"sub"` // User UUID
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"` // Полное имя
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`  // Имя
	FamilyName        string `json:"family_name"` // Фамилия
	Azp               string `json:"azp"`         // Authorized party (для service account)
	ClientID          string `json:"clientId"`    // Client ID (для service account)
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	jwt.RegisteredClaims
}

// GetUserID возвращает UUID пользователя из claims.
func (c *JWTClaims) GetUserID() string {
	return c.Sub
}

// GetFullName возвращает полное имя или комбинацию имени и фамилии.
func (c *JWTClaims) GetFullName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.GivenName + " " + c.FamilyName
}

// HasRole проверяет наличие роли у пользователя.
// Для service account токенов: роль "service-account" определяется по наличию Azp или ClientID.
func (c *JWTClaims) HasRole(role string) bool {
	// Специальная обработка для service account
	if role == "service-account" {
		return c.IsServiceAccount()
	}

	// Обычная проверка realm roles
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsServiceAccount проверяет, является ли токен service account токеном.
// Service account токены имеют claim azp или clientId, но не имеют email.
func (c *JWTClaims) IsServiceAccount() bool {
	// Для service account токенов Keycloak заполняет azp (authorized party)
	// и обычно нет email
	return c.Azp != "" || c.ClientID != ""
}
