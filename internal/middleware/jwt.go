package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/FischukSergey/otus-ms/internal/jwks"
)

// ContextKey для хранения данных в context.
type ContextKey string

// Ключи для хранения данных JWT-авторизации в контексте запроса.
const (
	ContextKeyUserID     ContextKey = "user_id"
	ContextKeyEmail      ContextKey = "email"
	ContextKeyGivenName  ContextKey = "given_name"
	ContextKeyFamilyName ContextKey = "family_name"
	ContextKeyClaims     ContextKey = "jwt_claims"
)

// JWTConfig содержит конфигурацию для JWT валидации.
type JWTConfig struct {
	Issuer     string // URL Keycloak realm
	Audience   string // Client ID
	SkipVerify bool   // Для тестов (не использовать в production!)
}

// ValidateJWT создаёт middleware для валидации JWT токенов через JWKS Manager.
//
// Параметры:
//   - config: конфигурация JWT (issuer, audience)
//   - jwksManager: менеджер JWKS ключей (обязателен для production, может быть nil только для тестов с SkipVerify)
//   - logger: логгер для записи событий
//
// Использует JWKS Manager для автоматической загрузки и обновления ключей от Keycloak.
func ValidateJWT(
	config JWTConfig,
	jwksManager *jwks.Manager,
	logger *slog.Logger,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекаем токен из заголовка Authorization
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn("missing authorization header",
					"path", r.URL.Path,
					"method", r.Method,
				)
				writeJSONError(w, "Missing authorization header")
				return
			}

			// Принимаем "Bearer <token>" или просто "<token>" (удобно для Swagger UI и др. клиентов)
			parts := strings.SplitN(strings.TrimSpace(authHeader), " ", 2)
			var tokenString string
			switch {
			case len(parts) == 2 && strings.EqualFold(parts[0], "Bearer"):
				tokenString = strings.TrimSpace(parts[1])
			case len(parts) == 1:
				tokenString = parts[0]
			default:
				logger.Warn("invalid authorization header format",
					"header", authHeader,
				)
				writeJSONError(w, "Invalid authorization header format")
				return
			}
			if tokenString == "" {
				writeJSONError(w, "Invalid authorization header format")
				return
			}

			// Парсим и валидируем токен
			claims := &JWTClaims{}

			// Для тестов парсим без проверки подписи, но С проверкой expiration
			//nolint:nestif // Сложность оправдана разными режимами валидации
			if config.SkipVerify {
				// Сначала парсим без проверки подписи
				parser := jwt.NewParser(jwt.WithoutClaimsValidation())
				_, _, err := parser.ParseUnverified(tokenString, claims)
				if err != nil {
					logger.Warn("JWT parsing error in skip_verify mode",
						"error", err,
						"path", r.URL.Path,
					)
					writeJSONError(w, "Invalid token")
					return
				}

				// Вручную проверяем expiration
				now := time.Now()
				if claims.ExpiresAt != nil && claims.ExpiresAt.Before(now) {
					logger.Warn("JWT token expired in skip_verify mode",
						"expires_at", claims.ExpiresAt.Time,
						"now", now,
						"path", r.URL.Path,
					)
					writeJSONError(w, "Invalid or expired token")
					return
				}
			} else {
				// Production: полная проверка через JWKS
				//nolint:contextcheck // jwt.ParseWithClaims callback has fixed signature without context
				token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
					// Production: проверяем RSA алгоритм
					if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}

					// Получаем ключи из JWKS Manager
					if jwksManager == nil {
						return nil, errors.New("JWKS Manager is not configured")
					}

					keySet, err := jwksManager.GetKeySet(r.Context())
					if err != nil {
						return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
					}

					// Извлекаем kid из заголовка токена
					kid, ok := token.Header["kid"].(string)
					if !ok {
						return nil, errors.New("token header missing kid")
					}

					// Находим нужный ключ по kid
					key, found := keySet.LookupKeyID(kid)
					if !found {
						return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
					}

					// Конвертируем в RSA public key
					var rawKey any
					if err := key.Raw(&rawKey); err != nil {
						return nil, fmt.Errorf("failed to get raw key: %w", err)
					}

					return rawKey, nil
				})
				if err != nil {
					logger.Warn("JWT parsing error",
						"error", err,
						"path", r.URL.Path,
					)
					writeJSONError(w, "Invalid token")
					return
				}

				if !token.Valid {
					logger.Warn("invalid JWT token",
						"path", r.URL.Path,
					)
					writeJSONError(w, "Invalid or expired token")
					return
				}
			}

			// Проверяем issuer (если указан в конфиге)
			if config.Issuer != "" && claims.Issuer != config.Issuer {
				logger.Warn("invalid issuer",
					"expected", config.Issuer,
					"got", claims.Issuer,
				)
				writeJSONError(w, "Invalid token issuer")
				return
			}

			// Добавляем claims в контекст
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyUserID, claims.GetUserID())
			ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
			ctx = context.WithValue(ctx, ContextKeyGivenName, claims.GivenName)
			ctx = context.WithValue(ctx, ContextKeyFamilyName, claims.FamilyName)
			ctx = context.WithValue(ctx, ContextKeyClaims, claims)

			logger.Debug("JWT validated successfully",
				"user_id", claims.GetUserID(),
				"email", claims.Email,
				"path", r.URL.Path,
			)

			// Передаём запрос дальше с обновлённым контекстом
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeJSONError отправляет JSON ответ с ошибкой (всегда 401 Unauthorized для JWT middleware).
func writeJSONError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprintf(w, `{"error":"%s"}`, message)
}

// GetUserIDFromContext извлекает user_id из контекста.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// GetEmailFromContext извлекает email из контекста.
func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(ContextKeyEmail).(string)
	return email, ok
}

// GetClaimsFromContext извлекает полные claims из контекста.
func GetClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*JWTClaims)
	return claims, ok
}
