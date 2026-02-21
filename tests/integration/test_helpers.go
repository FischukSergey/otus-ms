//go:build integration

package integration

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTClaims структура claims для тестовых JWT токенов.
// Должна совпадать с JWTClaims из internal/middleware/jwt_claims.go
//
//nolint:tagliatelle // Keycloak JWT uses snake_case field names
type TestJWTClaims struct {
	jwt.RegisteredClaims
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// generateTestToken генерирует тестовый JWT токен с указанными ролями.
// Использует HMAC подпись с секретом "test-secret" (как в middleware для тестов).
func generateTestToken(roles []string, userID string, email string) (string, error) {
	claims := TestJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   userID,
			Audience:  jwt.ClaimStrings{"test-client"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email:             email,
		EmailVerified:     true,
		Name:              "Test User",
		PreferredUsername: email,
		GivenName:         "Test",
		FamilyName:        "User",
	}
	claims.RealmAccess.Roles = roles

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte("test-secret"))
}

// GenerateAdminToken генерирует JWT токен с ролью admin.
// Используется для тестирования endpoints требующих admin права.
func GenerateAdminToken() string {
	token, err := generateTestToken(
		[]string{"admin", "user"},
		"admin-test-id",
		"admin@test.example.com",
	)
	if err != nil {
		panic("failed to generate admin token: " + err.Error())
	}
	return token
}

// GenerateUserToken генерирует JWT токен с ролью user (без admin).
// Используется для тестирования отказа в доступе к admin endpoints.
func GenerateUserToken() string {
	token, err := generateTestToken(
		[]string{"user"},
		"user-test-id",
		"user@test.example.com",
	)
	if err != nil {
		panic("failed to generate user token: " + err.Error())
	}
	return token
}

// GenerateTokenWithRoles генерирует JWT токен с произвольным набором ролей.
// Полезно для тестирования различных комбинаций ролей.
func GenerateTokenWithRoles(roles []string) string {
	token, err := generateTestToken(
		roles,
		"test-user-id",
		"test@example.com",
	)
	if err != nil {
		panic("failed to generate token: " + err.Error())
	}
	return token
}
