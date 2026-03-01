package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// RateLimiterInterface определяет интерфейс для Rate Limiter.
// Позволяет подменять реализацию в тестах.
type RateLimiterInterface interface {
	Allow(ctx context.Context, ip string) (bool, error)
}

// RateLimitMiddleware возвращает chi middleware, ограничивающий число запросов по IP.
// Если лимит превышен — возвращает 429 Too Many Requests.
// При ошибке Redis middleware пропускает запрос (fail open), чтобы не блокировать легитимных пользователей.
func RateLimitMiddleware(limiter RateLimiterInterface, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)

			allowed, err := limiter.Allow(r.Context(), ip)
			if err != nil {
				// При ошибке Redis — пропускаем запрос (fail open)
				logger.Warn("rate limiter error, skipping",
					"error", err,
					"ip", ip,
				)
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				logger.Warn("rate limit exceeded",
					"ip", ip,
					"path", r.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Too many login attempts. Please try again later.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractClientIP извлекает IP-адрес клиента из запроса.
// Проверяет заголовки X-Real-IP, X-Forwarded-For и RemoteAddr.
func extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	// Убираем порт из RemoteAddr (формат host:port)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
