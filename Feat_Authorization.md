# Архитектура авторизации с Keycloak

## Обзор

Документ описывает архитектуру авторизации микросервисной системы на основе Keycloak с использованием Auth-Proxy для централизованного управления токенами.

**Основные принципы:**
- ✅ Централизованная авторизация через Keycloak
- ✅ Auth-Proxy для user authentication
- ✅ Локальная валидация JWT в каждом микросервисе
- ✅ Service-to-Service аутентификация через Client Credentials
- ✅ Публичные ключи для валидации всех типов токенов

---

## 🏗️ Компоненты системы

### 1. Keycloak
**Роль:** Identity Provider (IdP)
- Хранит пользователей и клиентов (service accounts)
- Выдает JWT токены (user tokens, service tokens)
- Публикует публичные ключи через JWKS endpoint
- Управляет ролями и разрешениями

**Endpoints:**
```
POST /realms/{realm}/protocol/openid-connect/token    # Получение токенов
GET  /realms/{realm}/protocol/openid-connect/certs    # JWKS (публичные ключи)
POST /realms/{realm}/protocol/openid-connect/logout   # Logout
```

### 2. Auth-Proxy
**Роль:** Шлюз для user аутентификации

**Функции:**
- Проксирует login/refresh запросы к Keycloak
- Скрывает внутреннюю структуру Keycloak от клиентов
- Может добавлять дополнительную логику (rate limiting, audit)

**API:**
```
POST /api/v1/auth/login        # Логин пользователя
POST /api/v1/auth/refresh      # Обновление access token
POST /api/v1/auth/logout       # Logout (опционально)
GET  /api/v1/auth/.well-known  # Метаданные (опционально)
```

**Важно:** Auth-Proxy НЕ валидирует токены - это делают микросервисы!

### 3. Микросервисы
**Роль:** Бизнес-логика с авторизацией

**Функции:**
- ✅ Валидируют JWT токены **локально** (без запросов к Keycloak)
- ✅ Кэшируют публичные ключи из JWKS endpoint
- ✅ Получают service tokens для межсервисных вызовов
- ✅ Проверяют роли и permissions из токенов

**Каждый микросервис имеет:**
- JWT middleware для валидации входящих токенов
- Service account credentials для исходящих запросов
- Кэш публичных ключей (обновляется раз в 1-4 часа)
- Кэш service tokens (обновляется раз в 5 минут)

---

## 📊 Схема взаимодействия

```
┌────────────────────────────────────────────────────────────────┐
│                         KEYCLOAK                               │
│  • Хранит пользователей и клиентов                             │
│  • Выдает токены                                               │
│  • Публикует публичные ключи (JWKS endpoint)                   │
└────────────────────────────────────────────────────────────────┘
         ↑                    ↑                    ↑
         │                    │                    │
         │ Password Grant     │ Client Credentials │ JWKS
         │ (через proxy)      │ (напрямую)         │ (напрямую)
         │                    │                    │
┌────────┴──────┐      ┌──────┴───────┐      ┌────┴──────────┐
│  AUTH-PROXY   │      │ Микросервисы │      │ Микросервисы  │
│               │      │ (для service │      │ (для валидации│
│ /auth/login   │      │  tokens)     │      │  токенов)     │
│ /auth/refresh │      │              │      │               │
└───────┬───────┘      └──────┬───────┘      └───────────────┘
        │                     │
        │ User tokens         │ Service tokens
        │                     │
┌───────┴─────────────────────┴───────────────────────────────┐
│                      МИКРОСЕРВИСЫ                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │User Service  │  │Order Service │  │Payment Svc   │      │
│  │              │  │              │  │              │      │
│  │• Валидация   │  │• Валидация   │  │• Валидация   │      │
│  │  токенов     │  │  токенов     │  │  токенов     │      │
│  │• Service     │  │• Service     │  │• Service     │      │
│  │  account     │  │  account     │  │  account     │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

---

## 🎬 Сценарии использования

### Сценарий 1: Пользователь логинится

```
┌─────────┐      ┌──────────┐      ┌────────────┐      ┌──────────┐
│Frontend │      │Auth-Proxy│      │  Keycloak  │      │   DB     │
└────┬────┘      └─────┬────┘      └──────┬─────┘      └────┬─────┘
     │                 │                   │                 │
     │ 1. POST /auth/login                │                 │
     │ {username, password}                │                 │
     ├────────────────>│                   │                 │
     │                 │                   │                 │
     │                 │ 2. Password Grant  │                 │
     │                 │ (username, password)                │
     │                 ├──────────────────>│                 │
     │                 │                   │                 │
     │                 │                   │ 3. Verify user  │
     │                 │                   ├───────────────>│
     │                 │                   │<───────────────┤
     │                 │                   │                 │
     │                 │ 4. Access Token   │                 │
     │                 │    Refresh Token  │                 │
     │                 │<──────────────────┤                 │
     │                 │                   │                 │
     │ 5. Tokens       │                   │                 │
     │<────────────────┤                   │                 │
     │                 │                   │                 │
```

**Шаги:**
1. Пользователь вводит логин/пароль во frontend
2. Frontend отправляет POST запрос в Auth-Proxy
3. Auth-Proxy проксирует запрос в Keycloak (Password Grant Flow)
4. Keycloak проверяет credentials и выдает токены
5. Auth-Proxy возвращает токены frontend

**Токен пользователя:**
```json
{
  "sub": "user-uuid-123",
  "preferred_username": "john.doe",
  "email": "john@example.com",
  "realm_access": {
    "roles": ["user"]
  },
  "exp": 1707567890,
  "iss": "https://keycloak.example.com/realms/otus-ms",
  "aud": "account"
}
```

**Frontend сохраняет:**
- Access token → localStorage/sessionStorage
- Refresh token → httpOnly cookie (безопаснее)

---

### Сценарий 2: Пользователь работает с API

```
┌─────────┐      ┌──────────────┐      ┌──────────────┐
│Frontend │      │ User Service │      │Order Service │
└────┬────┘      └──────┬───────┘      └──────┬───────┘
     │                  │                     │
     │ 1. GET /api/v1/users/me                │
     │    Authorization: Bearer <user_token>  │
     ├─────────────────>│                     │
     │                  │                     │
     │                  │ 2. Validate token   │
     │                  │    • Parse JWT      │
     │                  │    • Check signature│
     │                  │    • Check exp/iss  │
     │                  │ ✓ Valid             │
     │                  │                     │
     │ 3. User data     │                     │
     │<─────────────────┤                     │
     │                  │                     │
     │ 4. GET /api/v1/orders                  │
     │    Authorization: Bearer <user_token>  │
     ├────────────────────────────────────────>│
     │                  │                     │
     │                  │                     │ 5. Validate token
     │                  │                     │    (локально)
     │                  │                     │ ✓ Valid
     │                  │                     │
     │ 6. Orders list   │                     │
     │<────────────────────────────────────────┤
     │                  │                     │
```

**Важно:**
- ✅ Keycloak НЕ участвует в этих запросах
- ✅ Валидация токена происходит **локально** в каждом микросервисе
- ✅ Используется публичный ключ из кэша
- ✅ Скорость валидации: < 1ms

**Что проверяется при валидации:**
1. **Подпись токена** - криптографическая проверка с публичным ключом
2. **Срок действия** (`exp`) - токен не истек
3. **Not Before** (`nbf`) - токен уже действителен
4. **Issuer** (`iss`) - токен выдан нашим Keycloak
5. **Audience** (`aud`) - токен предназначен для нас
6. **Роли** (`realm_access.roles`) - есть ли нужные права

---

### Сценарий 3: Микросервис вызывает другой микросервис

```
┌──────────────┐      ┌────────────┐      ┌──────────────┐
│Order Service │      │ Keycloak   │      │ User Service │
└──────┬───────┘      └─────┬──────┘      └──────┬───────┘
       │                    │                    │
       │ 1. Need user info  │                    │
       │                    │                    │
       │ 2. Check token cache                   │
       │ ❌ Not found       │                    │
       │                    │                    │
       │ 3. POST /token     │                    │
       │    grant_type=client_credentials        │
       │    client_id=order-service              │
       │    client_secret=xxx                    │
       ├───────────────────>│                    │
       │                    │                    │
       │ 4. Service Token   │                    │
       │<───────────────────┤                    │
       │                    │                    │
       │ 5. Cache token (5 min)                 │
       │                    │                    │
       │ 6. GET /api/internal/users/123          │
       │    Authorization: Bearer <service_token>│
       ├────────────────────────────────────────>│
       │                    │                    │
       │                    │                    │ 7. Validate token
       │                    │                    │    • Parse JWT
       │                    │                    │    • Check signature
       │                    │                    │    ✓ Valid
       │                    │                    │    ✓ Role: "service"
       │                    │                    │
       │ 8. User data       │                    │
       │<────────────────────────────────────────┤
       │                    │                    │
```

**Client Credentials Flow:**
```bash
curl -X POST https://keycloak.example.com/realms/otus-ms/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=order-service" \
  -d "client_secret=your-secret-here"
```

**Service токен:**
```json
{
  "sub": "service-account-order-service",
  "clientId": "order-service",
  "azp": "order-service",
  "realm_access": {
    "roles": ["service"]
  },
  "exp": 1707567890,
  "iss": "https://keycloak.example.com/realms/otus-ms"
}
```

**Особенности:**
- ✅ Токен кэшируется на 5 минут (до истечения)
- ✅ Автоматическое обновление при истечении
- ✅ Одного токена хватает для множества запросов
- ✅ Нет username - есть clientId

---

### Сценарий 4: Пользовательский запрос с межсервисным вызовом

```
┌─────────┐   ┌──────────────┐   ┌────────────┐   ┌──────────────┐
│Frontend │   │Order Service │   │  Keycloak  │   │ User Service │
└────┬────┘   └──────┬───────┘   └─────┬──────┘   └──────┬───────┘
     │               │                  │                 │
     │ 1. GET /orders                   │                 │
     │    Bearer <user_token>           │                 │
     ├──────────────>│                  │                 │
     │               │                  │                 │
     │               │ 2. Validate user_token             │
     │               │    ✓ Valid       │                 │
     │               │    ✓ user_id="123"                 │
     │               │                  │                 │
     │               │ 3. Get service token               │
     │               │    (from cache)  │                 │
     │               │ ✓ Found          │                 │
     │               │                  │                 │
     │               │ 4. GET /internal/users/123         │
     │               │    Bearer <service_token>          │
     │               ├────────────────────────────────────>│
     │               │                  │                 │
     │               │                  │                 │ 5. Validate
     │               │                  │                 │    service_token
     │               │                  │                 │ ✓ Valid
     │               │                  │                 │
     │               │ 6. User data     │                 │
     │               │<────────────────────────────────────┤
     │               │                  │                 │
     │               │ 7. Build response│                 │
     │               │    (orders + user info)            │
     │               │                  │                 │
     │ 8. Orders     │                  │                 │
     │<──────────────┤                  │                 │
     │               │                  │                 │
```

**Два токена в одном запросе:**
1. **User token** - для аутентификации пользователя
2. **Service token** - для межсервисного вызова

**Почему два токена?**
- User token может не иметь прав для internal API
- Service token гарантирует, что запрос от доверенного сервиса
- Разделение ответственности

---

## 🔑 Локальная валидация JWT

### Как работает

**1. Структура JWT токена:**
```
eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IkZsNFRZMH...  ← Header
.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0...  ← Payload
.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c           ← Signature
```

**2. Процесс валидации:**

```go
func ValidateToken(tokenString string) (*Claims, error) {
    // 1. Parse токен
    parts := strings.Split(tokenString, ".")
    header := base64Decode(parts[0])
    payload := base64Decode(parts[1])
    signature := base64Decode(parts[2])
    
    // 2. Извлекаем kid из header
    kid := header["kid"]
    
    // 3. Получаем публичный ключ из кэша
    publicKey := keyCache.Get(kid)
    
    // 4. Проверяем подпись (БЕЗ сетевых запросов!)
    expectedSignature := rsa.Sign(header + "." + payload, publicKey)
    if signature != expectedSignature {
        return nil, errors.New("invalid signature")
    }
    
    // 5. Проверяем claims
    claims := json.Unmarshal(payload)
    if claims.Exp < time.Now() {
        return nil, errors.New("token expired")
    }
    
    if claims.Iss != "https://keycloak.example.com/realms/otus-ms" {
        return nil, errors.New("invalid issuer")
    }
    
    return claims, nil
}
```

### Публичные ключи (JWKS)

**Keycloak JWKS endpoint:**
```
GET https://keycloak.example.com/realms/otus-ms/protocol/openid-connect/certs
```

**Ответ:**
```json
{
  "keys": [
    {
      "kid": "Fl4TY0rT7h_vMKk_Fxz1hQCvJ5M",
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "n": "xGOr-H7z...",  // Модуль RSA ключа
      "e": "AQAB"         // Экспонента
    }
  ]
}
```

**Кэширование:**
```go
type KeyCache struct {
    keys      map[string]*rsa.PublicKey  // kid -> public key
    keycloakURL string
    realm     string
    cacheTTL  time.Duration              // 1-4 часа
    lastFetch time.Time
}
```

**Стратегия обновления:**
- ✅ Обновляем **все ключи** при истечении TTL
- ✅ Обновляем **немедленно** при неизвестном kid
- ✅ При ошибке сети - используем устаревший кэш (до 24 часов)
- ✅ Фоновое обновление каждый час

### Производительность

| Операция | Время | Сеть |
|----------|-------|------|
| Первый запрос (загрузка ключей) | ~50ms | Да |
| Последующие запросы | <1ms | Нет |
| Обновление кэша (раз в час) | ~50ms | Да |
| Request rate | 10,000+ RPS | - |

---

## ⚙️ Конфигурация компонентов

### Auth-Proxy

```yaml
# configs/config.auth-proxy.yaml
global:
  env: prod

log:
  level: info

servers:
  api:
    addr: 0.0.0.0:38081
    read_timeout: 10s
    write_timeout: 10s

keycloak:
  url: "https://keycloak.example.com"
  realm: "otus-ms"
  # Для проксирования user auth

rate_limit:
  login_attempts: 5
  window_seconds: 60
```

### Микросервисы (User Service, Order Service и т.д.)

```yaml
# configs/config.user-service.yaml
global:
  env: prod

log:
  level: info

servers:
  api:
    addr: 0.0.0.0:38080

keycloak:
  url: "https://keycloak.example.com"
  realm: "otus-ms"
  
  # Для валидации входящих токенов
  jwks_url: "https://keycloak.example.com/realms/otus-ms/protocol/openid-connect/certs"
  jwks_cache_ttl: 3600  # 1 час
  
  # Для исходящих запросов (service account)
  service_account:
    client_id: "user-service"
    client_secret: "${KEYCLOAK_CLIENT_SECRET}"
    token_cache_ttl: 300  # 5 минут
```

### Docker Compose (локальная разработка)

```yaml
# deploy/local/docker-compose.local.yml
services:
  keycloak:
    image: quay.io/keycloak/keycloak:23.0
    environment:
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
    ports:
      - "8080:8080"
    command: start-dev
    networks:
      - otus-network
  
  auth-proxy:
    build:
      context: ../..
      dockerfile: auth-proxy.Dockerfile
    ports:
      - "38081:8080"
    depends_on:
      - keycloak
    environment:
      KEYCLOAK_URL: http://keycloak:8080
      KEYCLOAK_REALM: otus-ms
    networks:
      - otus-network
  
  user-service:
    build:
      context: ../..
      dockerfile: user-service.Dockerfile
    ports:
      - "38082:8080"
    depends_on:
      - keycloak
      - postgres
    environment:
      KEYCLOAK_URL: http://keycloak:8080
      KEYCLOAK_REALM: otus-ms
      KEYCLOAK_CLIENT_SECRET: ${KEYCLOAK_CLIENT_SECRET}
    networks:
      - otus-network
  
  order-service:
    build:
      context: ../..
      dockerfile: order-service.Dockerfile
    ports:
      - "38083:8080"
    depends_on:
      - keycloak
      - user-service
    environment:
      KEYCLOAK_URL: http://keycloak:8080
      KEYCLOAK_REALM: otus-ms
      KEYCLOAK_CLIENT_SECRET: ${KEYCLOAK_CLIENT_SECRET}
      USER_SERVICE_URL: http://user-service:8080
    networks:
      - otus-network

networks:
  otus-network:
    driver: bridge
```

---

## 🔒 Безопасность

### Разделение API

**Публичные эндпоинты** (для пользователей):
```go
r.Group(func(r chi.Router) {
    r.Use(authMiddleware.Authenticate)
    r.Use(authMiddleware.RequireUserToken())
    
    r.Get("/api/v1/orders", orderHandler.List)           // Мои заказы
    r.Get("/api/v1/orders/{id}", orderHandler.Get)       // Мой заказ
    r.Post("/api/v1/orders", orderHandler.Create)        // Создать
})
```

**Internal эндпоинты** (только для сервисов):
```go
r.Group(func(r chi.Router) {
    r.Use(authMiddleware.Authenticate)
    r.Use(authMiddleware.RequireServiceAccount())
    
    r.Get("/api/internal/orders", orderHandler.ListAll)           // Все
    r.Get("/api/internal/orders/user/{id}", orderHandler.ByUser)  // По user
    r.Patch("/api/internal/orders/{id}/status", orderHandler.UpdateStatus)
})
```

### Проверка ролей

```go
func (m *JWTMiddleware) RequireRoles(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := r.Context().Value("claims").(jwt.MapClaims)
            
            realmAccess := claims["realm_access"].(map[string]interface{})
            userRoles := realmAccess["roles"].([]interface{})
            
            hasRole := false
            for _, requiredRole := range roles {
                for _, userRole := range userRoles {
                    if userRole.(string) == requiredRole {
                        hasRole = true
                        break
                    }
                }
            }
            
            if !hasRole {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### Хранение токенов

**Frontend:**
- ✅ Access token → sessionStorage (автоматически очищается при закрытии вкладки)
- ✅ Refresh token → httpOnly cookie (защита от XSS)
- ❌ НЕ хранить токены в localStorage (уязвимо к XSS)

**Backend (service accounts):**
- ✅ Client secret → переменные окружения
- ✅ Токены → in-memory cache
- ❌ НЕ хранить credentials в коде/конфигах

### Rate Limiting

```go
// В Auth-Proxy
func (s *AuthService) Login(ctx context.Context, req LoginRequest) error {
    // Rate limiting по IP
    if s.rateLimiter.IsBlocked(req.IP) {
        return ErrTooManyAttempts
    }
    
    // Попытка логина
    result, err := s.keycloak.Login(ctx, req)
    if err != nil {
        s.rateLimiter.RecordFailure(req.IP)
        return err
    }
    
    s.rateLimiter.RecordSuccess(req.IP)
    return result
}
```

---

## 📊 Таблица взаимодействий

| От кого | Куда | Для чего | Токен | Как часто |
|---------|------|----------|-------|-----------|
| Frontend | Auth-Proxy | Логин | - | По требованию |
| Auth-Proxy | Keycloak | Получить user token | - | По требованию |
| Frontend | Микросервисы | API запросы | User token | Постоянно |
| Микросервисы | Keycloak JWKS | Публичные ключи | - | Раз в 1-4 часа |
| Микросервисы | Keycloak | Service token | Client credentials | Раз в 5 минут |
| Микросервис A | Микросервис B | Межсервисные вызовы | Service token | Постоянно |

---

## 🎯 Резюме

### Ключевые принципы

1. **Auth-Proxy** - только для получения user токенов (login/refresh)
2. **Локальная валидация** - каждый микросервис валидирует токены сам
3. **Публичные ключи** - один набор для всех типов токенов
4. **Service tokens** - получаются напрямую из Keycloak (Client Credentials)
5. **Два типа API** - public (для users) и internal (для services)

### Производительность

- ✅ Валидация токена: < 1ms (без сетевых запросов)
- ✅ Обновление ключей: раз в 1-4 часа
- ✅ Service token: кэшируется на 5 минут
- ✅ Масштабируемость: 10,000+ RPS на инстанс

### Безопасность

- ✅ JWT подпись проверяется криптографически
- ✅ Токены передаются только по HTTPS
- ✅ Refresh token rotation
- ✅ Rate limiting на login
- ✅ Разделение public/internal API
- ✅ Проверка ролей и permissions

### Библиотеки

```go
require (
    github.com/Nerzal/gocloak/v13 v13.9.0  // Keycloak client
    github.com/golang-jwt/jwt/v5 v5.2.0    // JWT валидация
)
```

---

## 📚 Дополнительные материалы

### Keycloak Realm настройка

1. Создать Realm: `otus-ms`
2. Создать Clients для каждого сервиса:
   - `user-service` (Access Type: confidential, Service Accounts: ON)
   - `order-service` (Access Type: confidential, Service Accounts: ON)
   - `payment-service` (Access Type: confidential, Service Accounts: ON)
3. Настроить Roles:
   - `user` - для обычных пользователей
   - `admin` - для администраторов
   - `service` - для service accounts
4. Настроить Token Settings:
   - Access Token Lifespan: 5 минут
   - Refresh Token Lifespan: 30 дней
   - Client Session Idle: 30 минут

### Мониторинг

**Метрики для Prometheus:**
```go
// Количество валидаций токенов
jwt_validations_total{service="user-service",result="success"} 1234
jwt_validations_total{service="user-service",result="failed"} 12

// Обновления кэша ключей
jwks_refresh_total{service="user-service",result="success"} 24

// Service token запросы
service_token_requests_total{service="order-service"} 48
```

**Логи для аудита:**
```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "info",
  "msg": "user authenticated",
  "user_id": "user-uuid-123",
  "username": "john.doe",
  "ip": "192.168.1.1",
  "service": "auth-proxy"
}
```

---

**Статус:** Готово к реализации
**Дата:** 2026-02-10
