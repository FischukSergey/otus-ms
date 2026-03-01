package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
)

// RequireRole создаёт middleware для проверки наличия требуемой роли в JWT токене.
//
// Параметры:
//   - roles: список допустимых ролей (достаточно любой одной)
//   - logger: логгер для записи событий
//
// ВАЖНО: Должен использоваться ПОСЛЕ ValidateJWT middleware!
// ValidateJWT добавляет claims в контекст, RequireRole проверяет роли из этих claims.
//
// Пример использования:
//
//	// Защитить роут для администраторов
//	r.With(middleware.RequireRole([]string{"admin"}, logger)).
//	  Delete("/api/v1/users/{id}", handler.Delete)
//
//	// Защитить роут для пользователей и админов
//	r.With(middleware.RequireRole([]string{"user", "admin"}, logger)).
//	  Get("/api/v1/users/me", handler.GetMe)
func RequireRole(roles []string, logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Извлекаем claims из контекста (должны быть добавлены ValidateJWT)
			claims, ok := GetClaimsFromContext(r.Context())
			if !ok {
				logger.Error("claims not found in context - ValidateJWT middleware missing?",
					"path", r.URL.Path,
					"method", r.Method,
				)
				writeRBACError(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Проверяем наличие хотя бы одной требуемой роли
			hasAccess := false
			matchedRole := ""
			for _, requiredRole := range roles {
				if claims.HasRole(requiredRole) {
					hasAccess = true
					matchedRole = requiredRole
					break
				}
			}

			if !hasAccess {
				logger.Warn("access denied - missing required role",
					"user_id", claims.GetUserID(),
					"email", claims.Email,
					"required_roles", roles,
					"user_roles", claims.RealmAccess.Roles,
					"path", r.URL.Path,
					"method", r.Method,
				)
				writeRBACError(w, "Access denied - insufficient permissions", http.StatusForbidden)
				return
			}

			logger.Debug("role check passed",
				"user_id", claims.GetUserID(),
				"email", claims.Email,
				"matched_role", matchedRole,
				"path", r.URL.Path,
				"method", r.Method,
			)

			// Пользователь имеет нужную роль - продолжаем
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin — удобная обёртка для проверки роли admin.
//
// Использование:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(middleware.RequireAdmin(logger))
//	    // Все роуты в этой группе требуют роль admin
//	    r.Delete("/users/{id}", handler.Delete)
//	    r.Get("/stats", handler.GetStats)
//	})
func RequireAdmin(logger *slog.Logger) func(next http.Handler) http.Handler {
	return RequireRole([]string{"admin"}, logger)
}

// RequireUser — проверка что пользователь имеет роль user или admin.
//
// Обычные пользователи получают роль "user", администраторы обычно имеют обе роли.
// Этот middleware позволяет доступ для обеих ролей.
//
// Использование:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(middleware.RequireUser(logger))
//	    // Доступно для пользователей с ролью user или admin
//	    r.Get("/me", handler.GetMe)
//	    r.Put("/me", handler.UpdateMe)
//	})
func RequireUser(logger *slog.Logger) func(next http.Handler) http.Handler {
	return RequireRole([]string{"user", "admin"}, logger)
}

// RequireServiceAccount — проверка что запрос выполняется от service account.
//
// Service account используется для service-to-service коммуникации.
// Например, Auth-Proxy вызывает Main Service API с токеном service account.
//
// Требования:
//   - Service account должен быть настроен в Keycloak с ролью service-account
//   - JWT токен получается через Client Credentials Flow
//
// Использование:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(middleware.RequireServiceAccount(logger))
//	    // Service-to-service endpoints
//	    r.Post("/users", handler.CreateFromService)
//	})
func RequireServiceAccount(logger *slog.Logger) func(next http.Handler) http.Handler {
	return RequireRole([]string{"service-account"}, logger)
}

// writeRBACError отправляет JSON ответ с ошибкой RBAC.
func writeRBACError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"error":"%s"}`, message)
}
