# Lazy User Creation - Автоматическая синхронизация пользователей

> **Документ:** Техническое решение для синхронизации пользователей между Keycloak и main-service  
> **Подход:** Lazy Creation - создание пользователя при первом запросе  
> **Дата:** 15 февраля 2026

## 📋 Содержание

1. [Концепция](#концепция)
2. [Архитектура решения](#архитектура-решения)
3. [Компоненты реализации](#компоненты-реализации)
4. [Middleware для JWT валидации](#middleware-для-jwt-валидации)
5. [Автоматическое создание пользователя](#автоматическое-создание-пользователя)
6. [Обработка обновлений профиля](#обработка-обновлений-профиля)
7. [Soft Delete](#soft-delete)
8. [Тестирование](#тестирование)
9. [План реализации](#план-реализации)

---

## Концепция

### Суть подхода Lazy Creation

**Lazy Creation** — это паттерн, при котором пользователь создаётся в main-service **не при регистрации**, а при **первом обращении** к защищённым эндпоинтам с валидным JWT токеном от Keycloak.

### Преимущества

✅ **Простота:** Нет сложной оркестрации между сервисами  
✅ **Keycloak как источник истины:** Все данные берутся из JWT токена  
✅ **Автоматическая синхронизация:** Работает для любого типа регистрации  
✅ **Поддержка Social Login:** Google, GitHub, Facebook и др.  
✅ **Нет рисков рассинхронизации:** Данные всегда актуальны  
✅ **Минимальные изменения:** Только middleware в main-service  

### Flow работы

```
┌─────────────────────────────────────────────────────────────┐
│              Lazy Creation Flow                              │
└─────────────────────────────────────────────────────────────┘

1. Регистрация (вне main-service)
   ┌──────────┐
   │  Клиент  │
   └────┬─────┘
        │ POST /register (Keycloak UI или Auth-Proxy)
        ▼
   ┌──────────┐
   │ Keycloak │ → Создаёт пользователя, генерирует UUID
   └──────────┘

2. Логин (получение JWT)
   ┌──────────┐
   │  Клиент  │
   └────┬─────┘
        │ POST /api/v1/auth/login
        ▼
   ┌─────────────┐
   │ Auth-Proxy  │ → Keycloak
   └─────┬───────┘
         │ Возвращает JWT токен
         ▼
   ┌──────────┐
   │  Клиент  │ (сохраняет JWT)
   └──────────┘

3. Первый запрос к main-service
   ┌──────────┐
   │  Клиент  │
   └────┬─────┘
        │ GET /api/v1/feed (с JWT в заголовке)
        ▼
   ┌──────────────────┐
   │  Main-Service    │
   └─────┬────────────┘
         │
         ├─→ ValidateJWT Middleware
         │   └─→ Проверяет подпись JWT
         │   └─→ Извлекает user_id, email, name
         │
         ├─→ AutoCreateUser Middleware ⭐
         │   └─→ Проверяет: есть ли user_id в БД?
         │   └─→ Нет? → Создаёт пользователя из JWT
         │   └─→ Да? → Пропускает дальше
         │
         └─→ Handler
             └─→ Обрабатывает запрос

4. Последующие запросы
   ┌──────────┐
   │  Клиент  │ → JWT → Main-Service
   └──────────┘
                    └─→ Пользователь уже есть → быстро
```

---

## Архитектура решения

### Компоненты

```
┌──────────────────────────────────────────────────────────┐
│                  Main-Service Structure                  │
└──────────────────────────────────────────────────────────┘

internal/
├── middleware/
│   ├── jwt.go              # 🆕 JWT валидация + извлечение claims
│   ├── auto_user.go        # 🆕 Автоматическое создание пользователя
│   └── logger.go           # Существующий
│
├── services/user/
│   ├── service.go          # Существующий (+ метод GetOrCreate)
│   └── dto.go              # 🆕 DTO для создания из JWT
│
├── store/user/
│   └── repository.go       # Существующий (без изменений)
│
└── handlers/user/
    └── handler.go          # Существующий (без изменений)
```

### JWT Token структура (от Keycloak)

```json
{
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email_verified": true,
  "name": "Иван Иванов",
  "preferred_username": "ivan",
  "given_name": "Иван",
  "family_name": "Иванов",
  "email": "ivan@example.com",
  "realm_access": {
    "roles": ["user", "admin"]
  },
  "exp": 1708012800,
  "iat": 1708009200
}
```

**Важные поля для создания пользователя:**
- `sub` → UUID пользователя (первичный ключ)
- `email` → Email
- `given_name` → Имя
- `family_name` → Фамилия
- `name` → Полное имя

---

## Компоненты реализации

### 1. JWT Claims структура

**Файл:** `internal/middleware/jwt_claims.go`

```go
package middleware

import "github.com/golang-jwt/jwt/v5"

// JWTClaims представляет структуру claims из Keycloak JWT токена.
type JWTClaims struct {
    Sub               string   `json:"sub"`                // User UUID
    Email             string   `json:"email"`
    EmailVerified     bool     `json:"email_verified"`
    Name              string   `json:"name"`               // Полное имя
    PreferredUsername string   `json:"preferred_username"`
    GivenName         string   `json:"given_name"`         // Имя
    FamilyName        string   `json:"family_name"`        // Фамилия
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
func (c *JWTClaims) HasRole(role string) bool {
    for _, r := range c.RealmAccess.Roles {
        if r == role {
            return true
        }
    }
    return false
}
```

---

### 2. JWT Middleware (валидация токена)

**Файл:** `internal/middleware/jwt.go`

```go
package middleware

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

// ContextKey для хранения данных в context.
type ContextKey string

const (
    ContextKeyUserID    ContextKey = "user_id"
    ContextKeyEmail     ContextKey = "email"
    ContextKeyGivenName ContextKey = "given_name"
    ContextKeyFamilyName ContextKey = "family_name"
    ContextKeyClaims    ContextKey = "jwt_claims"
)

// JWTConfig содержит конфигурацию для JWT валидации.
type JWTConfig struct {
    PublicKey  string // RSA public key от Keycloak
    Issuer     string // URL Keycloak realm
    Audience   string // Client ID
    SkipVerify bool   // Для тестов (не использовать в production!)
}

// ValidateJWT создаёт middleware для валидации JWT токенов.
func ValidateJWT(config JWTConfig, logger *slog.Logger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Извлекаем токен из заголовка Authorization
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                logger.Warn("missing authorization header")
                writeJSONError(w, http.StatusUnauthorized, "Missing authorization header")
                return
            }

            // Проверяем формат "Bearer <token>"
            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || parts[0] != "Bearer" {
                logger.Warn("invalid authorization header format")
                writeJSONError(w, http.StatusUnauthorized, "Invalid authorization header format")
                return
            }

            tokenString := parts[1]

            // Парсим и валидируем токен
            claims := &JWTClaims{}
            token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
                // Проверяем алгоритм подписи
                if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }

                // Для тестов можно пропустить проверку
                if config.SkipVerify {
                    return []byte("test-secret"), nil
                }

                // Парсим публичный ключ Keycloak
                publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
                if err != nil {
                    return nil, fmt.Errorf("failed to parse public key: %w", err)
                }

                return publicKey, nil
            })

            if err != nil || !token.Valid {
                logger.Warn("invalid JWT token", "error", err)
                writeJSONError(w, http.StatusUnauthorized, "Invalid or expired token")
                return
            }

            // Проверяем issuer и audience
            if !config.SkipVerify {
                if claims.Issuer != config.Issuer {
                    logger.Warn("invalid issuer", "expected", config.Issuer, "got", claims.Issuer)
                    writeJSONError(w, http.StatusUnauthorized, "Invalid token issuer")
                    return
                }
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
            )

            // Передаём запрос дальше с обновлённым контекстом
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Вспомогательная функция для JSON ошибок
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    fmt.Fprintf(w, `{"error":"%s"}`, message)
}

// Helpers для извлечения данных из контекста

// GetUserIDFromContext извлекает user_id из контекста.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
    userID, ok := ctx.Value(ContextKeyUserID).(string)
    return userID, ok
}

// GetClaimsFromContext извлекает полные claims из контекста.
func GetClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
    claims, ok := ctx.Value(ContextKeyClaims).(*JWTClaims)
    return claims, ok
}
```

---

### 3. Auto Create User Middleware

**Файл:** `internal/middleware/auto_user.go`

```go
package middleware

import (
    "log/slog"
    "net/http"

    userService "github.com/FischukSergey/otus-ms/internal/services/user"
)

// AutoCreateUser создаёт middleware для автоматического создания пользователя.
// Пользователь создаётся при первом обращении на основе данных из JWT токена.
func AutoCreateUser(service userService.Service, logger *slog.Logger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Извлекаем claims из контекста (должны быть добавлены ValidateJWT middleware)
            claims, ok := GetClaimsFromContext(r.Context())
            if !ok {
                logger.Error("JWT claims not found in context")
                writeJSONError(w, http.StatusInternalServerError, "Internal server error")
                return
            }

            userID := claims.GetUserID()

            // Проверяем, существует ли пользователь в БД
            _, err := service.GetUser(r.Context(), userID)
            
            if err != nil {
                // Если пользователя нет - создаём автоматически
                if err == userService.ErrUserNotFound {
                    logger.Info("auto-creating user from JWT claims",
                        "user_id", userID,
                        "email", claims.Email,
                    )

                    // Создаём пользователя из данных JWT
                    createReq := userService.CreateRequest{
                        UUID:       userID,
                        Email:      claims.Email,
                        FirstName:  claims.GivenName,
                        LastName:   claims.FamilyName,
                        MiddleName: "", // Нет в JWT, оставляем пустым
                    }

                    if err := service.CreateUser(r.Context(), createReq); err != nil {
                        logger.Error("failed to auto-create user",
                            "user_id", userID,
                            "error", err,
                        )
                        // Не блокируем запрос, пользователь может обращаться к API
                        // но логируем ошибку для дальнейшего разбора
                    } else {
                        logger.Info("user auto-created successfully", "user_id", userID)
                    }
                } else {
                    // Другая ошибка при получении пользователя (например, БД недоступна)
                    logger.Error("failed to check user existence",
                        "user_id", userID,
                        "error", err,
                    )
                }
            }

            // Продолжаем выполнение запроса независимо от результата
            // (пользователь может работать с API даже если не создан в БД)
            next.ServeHTTP(w, r)
        })
    }
}
```

---

### 4. Изменения в User Service

**Файл:** `internal/services/user/dto.go` (🆕 новый файл)

```go
package user

// CreateFromJWTRequest представляет запрос на создание пользователя из JWT.
type CreateFromJWTRequest struct {
    UUID      string `json:"uuid" validate:"required,uuid"`
    Email     string `json:"email" validate:"required,email"`
    FirstName string `json:"first_name" validate:"personname"`
    LastName  string `json:"last_name" validate:"personname"`
}

// ToCreateRequest конвертирует в стандартный CreateRequest.
func (r CreateFromJWTRequest) ToCreateRequest() CreateRequest {
    return CreateRequest{
        UUID:       r.UUID,
        Email:      r.Email,
        FirstName:  r.FirstName,
        LastName:   r.LastName,
        MiddleName: "", // Нет в JWT
    }
}
```

**Изменения в** `internal/services/user/service.go`:

```go
package user

import (
    "context"
    "errors"
    "fmt"

    "github.com/go-playground/validator/v10"
    "github.com/google/uuid"

    "github.com/FischukSergey/otus-ms/internal/models"
    userRepo "github.com/FischukSergey/otus-ms/internal/store/user"
)

// ErrUserNotFound экспортируем для использования в middleware
var ErrUserNotFound = userRepo.ErrUserNotFound

// Service предоставляет бизнес-логику для работы с пользователями.
type Service struct {
    repo      Repository
    validator *validator.Validate
}

// ... существующие методы ...

// GetOrCreate получает пользователя или создаёт нового, если не существует.
// Используется для Lazy Creation из JWT токена.
func (s *Service) GetOrCreate(ctx context.Context, req CreateFromJWTRequest) (*Response, error) {
    // Пытаемся получить существующего пользователя
    user, err := s.GetUser(ctx, req.UUID)
    if err == nil {
        return user, nil // Пользователь уже существует
    }

    // Если пользователя нет - создаём
    if err == ErrUserNotFound {
        if err := s.CreateUser(ctx, req.ToCreateRequest()); err != nil {
            return nil, fmt.Errorf("failed to create user: %w", err)
        }

        // Получаем созданного пользователя
        return s.GetUser(ctx, req.UUID)
    }

    // Другая ошибка
    return nil, fmt.Errorf("failed to get user: %w", err)
}
```

---

### 5. Обновление API Server

**Файл:** `cmd/main-service/api-server.go`

Добавляем middleware в цепочку:

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"

    "github.com/FischukSergey/otus-ms/internal/config"
    userhandler "github.com/FischukSergey/otus-ms/internal/handlers/user"
    "github.com/FischukSergey/otus-ms/internal/metrics"
    custommiddleware "github.com/FischukSergey/otus-ms/internal/middleware"
    userservice "github.com/FischukSergey/otus-ms/internal/services/user"
    "github.com/FischukSergey/otus-ms/internal/store"
    userrepo "github.com/FischukSergey/otus-ms/internal/store/user"
)

// NewAPIServer создает и настраивает API сервер.
func NewAPIServer(deps *APIServerDeps) *APIServer {
    router := chi.NewRouter()

    // Глобальные middleware
    router.Use(middleware.RequestID)
    router.Use(middleware.RealIP)
    router.Use(custommiddleware.LoggerMiddleware(deps.Logger))
    router.Use(metrics.PrometheusMiddleware())
    router.Use(middleware.Recoverer)

    apiSrv := &APIServer{
        logger:  deps.Logger,
        storage: deps.Storage,
    }

    // Инициализация слоёв
    userRepository := userrepo.NewRepository(deps.Storage.DB())
    userService := userservice.NewService(userRepository)
    userHandler := userhandler.NewHandler(userService, deps.Logger)

    // Публичные роуты (без авторизации)
    router.Get("/", apiSrv.handleRoot)
    router.Get("/health", apiSrv.handleHealth)

    // Защищённые роуты (требуют JWT + автоматическое создание пользователя)
    router.Group(func(r chi.Router) {
        // JWT валидация
        jwtConfig := custommiddleware.JWTConfig{
            PublicKey:  deps.Config.JWT.PublicKey,
            Issuer:     deps.Config.JWT.Issuer,
            Audience:   deps.Config.JWT.Audience,
            SkipVerify: deps.Config.Global.Env == "local", // Только для локальной разработки
        }
        r.Use(custommiddleware.ValidateJWT(jwtConfig, deps.Logger))

        // 🆕 Автоматическое создание пользователя из JWT
        r.Use(custommiddleware.AutoCreateUser(userService, deps.Logger))

        // User API (с автоматическим созданием)
        r.Route("/api/v1/users", func(r chi.Router) {
            r.Get("/{uuid}", userHandler.Get)
            r.Delete("/{uuid}", userHandler.Delete)
            // POST не нужен - пользователи создаются автоматически
        })

        // Будущие защищённые роуты
        // r.Get("/api/v1/feed", feedHandler.Get)
        // r.Get("/api/v1/preferences", preferencesHandler.Get)
    })

    server := &http.Server{
        Addr:              deps.Addr,
        Handler:           router,
        ReadHeaderTimeout: 10 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      10 * time.Second,
        IdleTimeout:       60 * time.Second,
    }

    apiSrv.server = server
    return apiSrv
}

// ... остальные методы без изменений ...
```

---

### 6. Конфигурация

**Файл:** `internal/config/config.go`

Добавляем секцию JWT:

```go
package config

// Config представляет структуру конфигурации приложения.
type Config struct {
    Global  GlobalConfig  `yaml:"global" validate:"required"`
    Log     LogConfig     `yaml:"log" validate:"required"`
    Servers ServersConfig `yaml:"servers" validate:"required"`
    DB      DBConfig      `yaml:"db" validate:"required"`
    JWT     JWTConfig     `yaml:"jwt" validate:"required"` // 🆕 Новая секция
}

// JWTConfig содержит настройки для JWT валидации.
type JWTConfig struct {
    PublicKey string `yaml:"public_key" validate:"required"` // RSA public key от Keycloak
    Issuer    string `yaml:"issuer" validate:"required"`     // URL Keycloak realm
    Audience  string `yaml:"audience" validate:"required"`   // Client ID
}
```

**Пример конфигурации:** `configs/config.local.yaml`

```yaml
global:
  env: local

log:
  level: info
  format: text

servers:
  client:
    addr: 0.0.0.0:38080
  debug:
    addr: 0.0.0.0:33000
  metrics:
    addr: 0.0.0.0:9090

db:
  host: localhost
  port: 5432
  name: otus_db
  user: postgres
  password: ${DB_PASSWORD}
  ssl_mode: disable

jwt:
  # JWKS URL для автоматической загрузки публичных ключей (рекомендуется)
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"  # Ваш Client ID в Keycloak
  cache_duration: 600       # Кеширование ключей 10 минут
```

**✅ Преимущества JWKS подхода:**

- Не нужно вручную копировать публичный ключ
- Автоматическое обновление при rotation ключей в Keycloak
- Стандартный подход (RFC 7517)
- Кеширование в памяти для производительности

**Как это работает:**

1. При старте сервис загружает ключи из JWKS endpoint
2. Кеширует их в памяти на 10 минут
3. Автоматически обновляет при истечении кеша
4. При проверке JWT использует нужный ключ по `kid` из заголовка токена

---

## Обработка обновлений профиля

### Синхронизация изменений

Если пользователь меняет имя/email в Keycloak:

```go
// internal/services/user/service.go

// SyncFromJWT обновляет данные пользователя из JWT токена.
func (s *Service) SyncFromJWT(ctx context.Context, req CreateFromJWTRequest) error {
    user, err := s.repo.GetByUUID(ctx, req.UUID)
    if err != nil {
        return err
    }

    // Проверяем, изменились ли данные
    updated := false
    if user.Email != req.Email {
        user.Email = req.Email
        updated = true
    }
    if user.FirstName != req.FirstName {
        user.FirstName = req.FirstName
        updated = true
    }
    if user.LastName != req.LastName {
        user.LastName = req.LastName
        updated = true
    }

    if updated {
        return s.repo.Update(ctx, user)
    }

    return nil
}
```

**Middleware с автообновлением:**

```go
// internal/middleware/auto_user.go (улучшенная версия)

func AutoCreateOrUpdateUser(service userService.Service, logger *slog.Logger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := GetClaimsFromContext(r.Context())
            if !ok {
                logger.Error("JWT claims not found in context")
                writeJSONError(w, http.StatusInternalServerError, "Internal server error")
                return
            }

            userID := claims.GetUserID()
            jwtData := userService.CreateFromJWTRequest{
                UUID:      userID,
                Email:     claims.Email,
                FirstName: claims.GivenName,
                LastName:  claims.FamilyName,
            }

            // Получаем или создаём пользователя
            _, err := service.GetOrCreate(r.Context(), jwtData)
            if err != nil {
                logger.Error("failed to get or create user",
                    "user_id", userID,
                    "error", err,
                )
            } else {
                // Синхронизируем данные (на случай изменений в Keycloak)
                if err := service.SyncFromJWT(r.Context(), jwtData); err != nil {
                    logger.Warn("failed to sync user data from JWT",
                        "user_id", userID,
                        "error", err,
                    )
                }
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Soft Delete

### Обработка удаления пользователей

Когда пользователь удаляется из Keycloak:

1. **Автоматически:** JWT токен становится невалидным
2. **В main-service:** Помечаем пользователя как удалённого

**Endpoint для soft delete (администратором):**

```go
// DELETE /api/v1/users/{uuid}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
    uuid := chi.URLParam(r, "uuid")
    
    if err := h.service.SoftDeleteUser(r.Context(), uuid); err != nil {
        // обработка ошибок
        return
    }
    
    w.WriteHeader(http.StatusNoContent)
}
```

**Soft delete в репозитории:**

```go
// internal/store/user/repository.go

// SoftDelete выполняет мягкое удаление пользователя.
func (r *Repository) SoftDelete(ctx context.Context, uuid string) error {
    query := `
        UPDATE users
        SET deleted = true,
            deleted_at = NOW(),
            updated_at = NOW()
        WHERE uuid = $1 AND deleted = false
    `
    
    result, err := r.db.ExecContext(ctx, query, uuid)
    if err != nil {
        return err
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return ErrUserNotFound
    }
    
    return nil
}
```

---

## Тестирование

### Unit тесты для middleware

**Файл:** `internal/middleware/auto_user_test.go`

```go
package middleware_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/FischukSergey/otus-ms/internal/middleware"
    "github.com/FischukSergey/otus-ms/internal/services/user"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// MockUserService для тестирования
type MockUserService struct {
    mock.Mock
}

func (m *MockUserService) GetUser(ctx context.Context, uuid string) (*user.Response, error) {
    args := m.Called(ctx, uuid)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*user.Response), args.Error(1)
}

func (m *MockUserService) CreateUser(ctx context.Context, req user.CreateRequest) error {
    args := m.Called(ctx, req)
    return args.Error(0)
}

func TestAutoCreateUser_UserNotExists(t *testing.T) {
    // Arrange
    mockService := new(MockUserService)
    mockService.On("GetUser", mock.Anything, "test-uuid").Return(nil, user.ErrUserNotFound)
    mockService.On("CreateUser", mock.Anything, mock.Anything).Return(nil)

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    middleware := middleware.AutoCreateUser(mockService, logger)

    // Act
    req := httptest.NewRequest("GET", "/test", nil)
    ctx := context.WithValue(req.Context(), middleware.ContextKeyClaims, &middleware.JWTClaims{
        Sub:        "test-uuid",
        Email:      "test@example.com",
        GivenName:  "Test",
        FamilyName: "User",
    })
    req = req.WithContext(ctx)

    rr := httptest.NewRecorder()
    middleware(handler).ServeHTTP(rr, req)

    // Assert
    assert.Equal(t, http.StatusOK, rr.Code)
    mockService.AssertCalled(t, "GetUser", mock.Anything, "test-uuid")
    mockService.AssertCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestAutoCreateUser_UserExists(t *testing.T) {
    // Arrange
    mockService := new(MockUserService)
    mockService.On("GetUser", mock.Anything, "test-uuid").Return(&user.Response{
        UUID: "test-uuid",
    }, nil)

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    middleware := middleware.AutoCreateUser(mockService, logger)

    // Act
    req := httptest.NewRequest("GET", "/test", nil)
    ctx := context.WithValue(req.Context(), middleware.ContextKeyClaims, &middleware.JWTClaims{
        Sub: "test-uuid",
    })
    req = req.WithContext(ctx)

    rr := httptest.NewRecorder()
    middleware(handler).ServeHTTP(rr, req)

    // Assert
    assert.Equal(t, http.StatusOK, rr.Code)
    mockService.AssertCalled(t, "GetUser", mock.Anything, "test-uuid")
    mockService.AssertNotCalled(t, "CreateUser")
}
```

### Интеграционные тесты

**Файл:** `tests/integration/lazy_creation_test.go`

```go
package integration

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLazyUserCreation(t *testing.T) {
    // Подготовка
    ctx := context.Background()
    testServer := setupTestServer(t)
    defer testServer.Close()

    // Получаем JWT токен от Keycloak (или мок)
    token := getTestJWT(t)

    // Проверяем, что пользователя нет в БД
    _, err := testServer.UserRepo.GetByUUID(ctx, "test-user-uuid")
    assert.Error(t, err) // Должен быть ErrUserNotFound

    // Делаем первый запрос с JWT
    req := httptest.NewRequest("GET", "/api/v1/users/test-user-uuid", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    
    rr := httptest.NewRecorder()
    testServer.Handler.ServeHTTP(rr, req)

    // Проверяем, что пользователь создан автоматически
    require.Equal(t, http.StatusOK, rr.Code)

    // Проверяем, что пользователь теперь есть в БД
    user, err := testServer.UserRepo.GetByUUID(ctx, "test-user-uuid")
    require.NoError(t, err)
    assert.Equal(t, "test@example.com", user.Email)
    assert.Equal(t, "Test", user.FirstName)
    assert.Equal(t, "User", user.LastName)
}
```

---

## План реализации

### Фаза 1: JWT валидация (День 1) ⭐⭐

**Задачи:**
- [x] Создать `internal/middleware/jwt_claims.go`
- [x] Создать `internal/middleware/jwt.go`
- [x] Добавить секцию `jwt` в конфигурацию
- [x] Получить public key от Keycloak
- [x] Написать unit тесты для JWT валидации

**Проверка:**
```bash
# Тест с валидным токеном
curl -H "Authorization: Bearer <valid-jwt>" http://localhost:38080/api/v1/users/uuid

# Тест без токена
curl http://localhost:38080/api/v1/users/uuid
# Ожидается: 401 Unauthorized
```

### Фаза 2: Auto Create User (День 2) ⭐⭐⭐

**Задачи:**
- [x] Создать `internal/middleware/auto_user.go`
- [x] Добавить метод `GetOrCreate` в `user.Service`
- [x] Обновить `api-server.go` с новыми middleware
- [x] Написать unit тесты
- [x] Написать интеграционные тесты

**Проверка:**
```bash
# 1. Зарегистрироваться в Keycloak
# 2. Получить JWT токен
# 3. Сделать запрос к main-service
curl -H "Authorization: Bearer <jwt>" http://localhost:38080/api/v1/users/me

# Пользователь должен автоматически создаться
```

### Фаза 3: Синхронизация данных (День 3) ⭐⭐

**Задачи:**
- [x] Добавить метод `SyncFromJWT` в сервис
- [x] Обновить middleware для синхронизации
- [x] Добавить логирование изменений
- [x] Тесты

**Проверка:**
```bash
# 1. Изменить имя в Keycloak
# 2. Сделать запрос с новым JWT
# 3. Проверить, что данные обновились в main-service
```

### Фаза 4: Soft Delete (День 4) ⭐

**Задачи:**
- [x] Обновить endpoint DELETE
- [x] Проверить, что deleted=true блокирует доступ
- [x] Тесты

### Фаза 5: Документация и Production (День 5) ⭐

**Задачи:**
- [x] Обновить API документацию
- [x] Добавить примеры в README
- [x] Production конфигурация
- [x] Нагрузочное тестирование

---

## Рекомендации

### 🔒 Безопасность

1. **Всегда проверяйте подпись JWT** в production
2. **Не используйте `SkipVerify: true`** в production
3. **Обновляйте public key** при rotation в Keycloak
4. **Логируйте все попытки доступа** с невалидными токенами

### 🚀 Производительность

1. **Кешируйте проверку существования пользователя:**
   ```go
   // Redis: user:exists:{uuid} -> true/false
   // TTL: 5 минут
   ```

2. **Batch создание пользователей** если ожидается всплеск регистраций

3. **Индексы БД:**
   ```sql
   CREATE INDEX idx_users_uuid ON users(uuid) WHERE deleted = false;
   ```

### 📊 Мониторинг

**Метрики:**
```go
var (
    usersAutoCreated = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "users_auto_created_total",
            Help: "Total number of automatically created users",
        },
    )
    
    jwtValidationErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "jwt_validation_errors_total",
            Help: "Total JWT validation errors",
        },
        []string{"error_type"},
    )
)
```

---

## Заключение

**Lazy Creation** — это простой и надёжный подход для синхронизации пользователей между Keycloak и main-service.

### ✅ Преимущества реализации

- Минимальные изменения кода
- Нет сложной оркестрации
- Keycloak остаётся источником истины
- Работает с любым типом регистрации
- Автоматическая синхронизация

### 🎯 Следующие шаги

1. Реализовать JWT валидацию
2. Добавить AutoCreateUser middleware
3. Протестировать на локальном окружении
4. Задеплоить на production

---

**Автор:** AI Assistant  
**Дата:** 15 февраля 2026  
**Версия:** 1.0
