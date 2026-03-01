package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/FischukSergey/otus-ms/internal/jwks"
)

// GRPCAuthConfig содержит конфигурацию для gRPC JWT-валидации.
type GRPCAuthConfig struct {
	Issuer     string // URL Keycloak realm
	SkipVerify bool   // Только для тестов!
}

// UnaryAuthInterceptor возвращает gRPC unary interceptor, который
// проверяет JWT-токен из metadata "authorization: Bearer <token>"
// с использованием JWKS Manager от Keycloak.
func UnaryAuthInterceptor(
	cfg GRPCAuthConfig,
	jwksManager *jwks.Manager,
	logger *slog.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		tokenString, err := extractBearerToken(ctx)
		if err != nil {
			logger.Warn("grpc auth: missing or malformed authorization",
				"method", info.FullMethod,
				"error", err,
			)
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		claims, err := parseToken(ctx, cfg, jwksManager, tokenString)
		if err != nil {
			logger.Warn("grpc auth: token validation failed",
				"method", info.FullMethod, "error", err,
			)
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		if cfg.Issuer != "" && claims.Issuer != cfg.Issuer {
			logger.Warn("grpc auth: invalid issuer",
				"method", info.FullMethod,
				"expected", cfg.Issuer,
				"got", claims.Issuer,
			)
			return nil, status.Error(codes.Unauthenticated, "invalid token issuer")
		}

		logger.Debug("grpc auth: token validated",
			"method", info.FullMethod,
			"subject", claims.Sub,
			"is_service_account", claims.IsServiceAccount(),
		)

		ctx = context.WithValue(ctx, ContextKeyClaims, claims)
		return handler(ctx, req)
	}
}

// parseToken выбирает стратегию валидации токена в зависимости от конфигурации.
func parseToken(ctx context.Context, cfg GRPCAuthConfig, jwksManager *jwks.Manager, tokenString string) (*JWTClaims, error) {
	if cfg.SkipVerify {
		return parseTokenSkipVerify(tokenString)
	}
	return parseTokenWithJWKS(ctx, jwksManager, tokenString)
}

// parseTokenSkipVerify парсит токен без проверки подписи (только для тестов).
func parseTokenSkipVerify(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, fmt.Errorf("JWT parse error: %w", err)
	}
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}
	return claims, nil
}

// parseTokenWithJWKS валидирует токен с проверкой подписи через JWKS.
func parseTokenWithJWKS(ctx context.Context, jwksManager *jwks.Manager, tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		if jwksManager == nil {
			return nil, errors.New("JWKS Manager is not configured")
		}
		keySet, err := jwksManager.GetKeySet(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("token header missing kid")
		}
		key, found := keySet.LookupKeyID(kid)
		if !found {
			return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
		}
		var rawKey any
		if err := key.Raw(&rawKey); err != nil {
			return nil, fmt.Errorf("failed to get raw key: %w", err)
		}
		return rawKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired token: %w", err)
	}
	return claims, nil
}

// extractBearerToken извлекает JWT из gRPC metadata (ключ "authorization").
func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return "", errors.New("missing authorization header")
	}

	parts := strings.SplitN(values[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization format, expected 'Bearer <token>'")
	}

	return parts[1], nil
}
