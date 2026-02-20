# Auth-Proxy API Documentation

## Обзор

Auth-Proxy - это микросервис для централизованной аутентификации пользователей через Keycloak. Он предоставляет простой REST API для логина, обновления токенов и logout.

**Base URL:** `http://localhost:38081` (локально) или `https://fishouk-otus-ms.ru` (продакшен)

**Content-Type:** `application/json`

## Endpoints

### 1. Health Check

Проверка работоспособности сервиса.

**Endpoint:** `GET /health`

**Response:**

```json
{
  "status": "ok",
  "time": "2026-02-13T10:30:00Z"
}
```

**Status Codes:**
- `200 OK` - сервис работает

**Пример cURL:**

```bash
curl http://localhost:38081/health
```

---

### 2. Login

Аутентификация пользователя и получение токенов.

**Endpoint:** `POST /api/v1/auth/login`

**Request Body:**

```json
{
  "username": "test@example.com",
  "password": "test123"
}
```

**Fields:**
- `username` (string, required) - имя пользователя или email
- `password` (string, required) - пароль

**Success Response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "refresh_expires_in": 1800,
  "token_type": "Bearer",
  "scope": "openid profile email"
}
```

**Response Fields:**
- `access_token` (string) - JWT токен для доступа к защищённым ресурсам
- `refresh_token` (string) - токен для обновления access token
- `expires_in` (int) - время жизни access token в секундах (обычно 300 = 5 минут)
- `refresh_expires_in` (int) - время жизни refresh token в секундах (обычно 1800 = 30 минут)
- `token_type` (string) - тип токена (всегда "Bearer")
- `scope` (string) - области доступа

**Error Responses:**

**400 Bad Request** - невалидный запрос:
```json
{
  "error": "Username and password are required"
}
```

**401 Unauthorized** - неверные credentials:
```json
{
  "error": "Invalid credentials"
}
```

**Status Codes:**
- `200 OK` - успешная аутентификация
- `400 Bad Request` - невалидный запрос
- `401 Unauthorized` - неверные credentials
- `500 Internal Server Error` - ошибка сервера

**Пример cURL:**

```bash
curl -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "test@example.com",
    "password": "test123"
  }'
```

**Пример успешного ответа:**

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJGbDRUWTByVDdoX3ZNS2tfRnh6MWhRQ3ZKNk0ifQ.eyJleHAiOjE3MDc1Njc4OTAsImlhdCI6MTcwNzU2NzU5MCwianRpIjoiYWJjMTIzIiwiaXNzIjoiaHR0cHM6Ly9maXNob3VrLW90dXMtbXMucnUvYXV0aC9yZWFsbXMvb3R1cy1tcyIsImF1ZCI6ImFjY291bnQiLCJzdWIiOiJ1c2VyLXV1aWQtMTIzIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiYXV0aC1wcm94eSIsInByZWZlcnJlZF91c2VybmFtZSI6InRlc3RAZXhhbXBsZS5jb20iLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20ifQ.signature",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJiNGU4Nzk3OC0zZjcxLTQ5NTEtOWQ3Ny1hNjg3OTIzZWYyZDgifQ.eyJleHAiOjE3MDc1NjkzOTAsImlhdCI6MTcwNzU2NzU5MCwianRpIjoieHl6NDU2Iiwic3ViIjoidXNlci11dWlkLTEyMyIsInR5cCI6IlJlZnJlc2gifQ.signature",
  "expires_in": 300,
  "refresh_expires_in": 1800,
  "token_type": "Bearer",
  "scope": "openid profile email"
}
```

---

### 3. Refresh Token

Обновление access token используя refresh token.

**Endpoint:** `POST /api/v1/auth/refresh`

**Request Body:**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Fields:**
- `refresh_token` (string, required) - refresh token, полученный при логине

**Success Response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "refresh_expires_in": 1800,
  "token_type": "Bearer",
  "scope": "openid profile email"
}
```

**Error Responses:**

**400 Bad Request:**
```json
{
  "error": "Refresh token is required"
}
```

**401 Unauthorized:**
```json
{
  "error": "Invalid or expired refresh token"
}
```

**Status Codes:**
- `200 OK` - токен успешно обновлён
- `400 Bad Request` - невалидный запрос
- `401 Unauthorized` - невалидный или истёкший refresh token
- `500 Internal Server Error` - ошибка сервера

**Пример cURL:**

```bash
curl -X POST http://localhost:38081/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

---

### 4. Logout

Выход пользователя из системы (инвалидация refresh token).

**Endpoint:** `POST /api/v1/auth/logout`

**Request Body:**

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Fields:**
- `refresh_token` (string, required) - refresh token для инвалидации

**Success Response (204 No Content):**

Тело ответа пустое.

**Error Responses:**

**400 Bad Request:**
```json
{
  "error": "Refresh token is required"
}
```

**500 Internal Server Error:**
```json
{
  "error": "Logout failed"
}
```

**Status Codes:**
- `204 No Content` - logout успешен
- `400 Bad Request` - невалидный запрос
- `500 Internal Server Error` - ошибка при logout

**Пример cURL:**

```bash
curl -X POST http://localhost:38081/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

---

## Использование токенов

### Access Token

После успешного логина используйте access token для доступа к защищённым API эндпоинтам:

```bash
curl http://localhost:38080/api/v1/users/me \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Обновление токена

Access token имеет короткое время жизни (обычно 5 минут). Когда он истекает, используйте refresh token для получения нового:

1. Клиент делает запрос с access token
2. Сервер возвращает 401 Unauthorized
3. Клиент вызывает `/api/v1/auth/refresh` с refresh token
4. Получает новый access token и refresh token
5. Повторяет запрос с новым access token

### Хранение токенов

**Рекомендации:**

- **Access Token:** sessionStorage (автоматически очищается при закрытии вкладки)
- **Refresh Token:** httpOnly cookie (защита от XSS атак)
- **НЕ** используйте localStorage для хранения токенов

## Коды ошибок

| Код | Описание |
|-----|----------|
| 200 | Успешный запрос |
| 204 | Успешный запрос без тела ответа |
| 400 | Невалидный запрос (отсутствуют обязательные поля) |
| 401 | Неавторизован (неверные credentials или токен) |
| 500 | Внутренняя ошибка сервера |

## Примеры полного flow

### Пример 1: Логин и использование токена

```bash
# 1. Логин
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test@example.com","password":"test123"}')

# 2. Извлекаем access token
ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.access_token')

# 3. Используем access token для запроса
curl http://localhost:38080/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Пример 2: Обновление токена

```bash
# 1. Извлекаем refresh token
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.refresh_token')

# 2. Обновляем токен
REFRESH_RESPONSE=$(curl -s -X POST http://localhost:38081/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")

# 3. Получаем новый access token
NEW_ACCESS_TOKEN=$(echo $REFRESH_RESPONSE | jq -r '.access_token')
```

### Пример 3: Logout

```bash
# Logout
curl -X POST http://localhost:38081/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
```

## Логирование

Auth-Proxy логирует следующие события:

### Успешный логин
```json
{
  "level": "info",
  "msg": "User logged in successfully",
  "username": "test@example.com",
  "ip": "192.168.1.1",
  "timestamp": "2026-02-13T10:30:00Z"
}
```

### Неудачный логин
```json
{
  "level": "error",
  "msg": "Login failed",
  "error": "invalid credentials",
  "username": "test@example.com",
  "ip": "192.168.1.1",
  "timestamp": "2026-02-13T10:30:00Z"
}
```

### Logout
```json
{
  "level": "info",
  "msg": "User logged out successfully",
  "ip": "192.168.1.1",
  "timestamp": "2026-02-13T10:30:00Z"
}
```

## Security Best Practices

1. **Всегда используйте HTTPS** в продакшене
2. **Не храните токены в localStorage** - используйте sessionStorage или httpOnly cookies
3. **Обновляйте токены проактивно** - не ждите 401 ошибки
4. **Реализуйте logout** при закрытии приложения
5. **Используйте короткое время жизни** для access token (5 минут)
6. **Логируйте все попытки аутентификации** для аудита

## Troubleshooting

### Ошибка "Invalid credentials"

**Возможные причины:**
- Неверный username или password
- Пользователь не существует в Keycloak
- Email не подтверждён

**Решение:**
- Проверьте credentials
- Убедитесь, что пользователь существует
- Проверьте Email verified = ON в Keycloak

### Ошибка "Invalid or expired refresh token"

**Возможные причины:**
- Refresh token истёк (по умолчанию 30 дней)
- Пользователь выполнил logout
- Refresh token был использован после logout

**Решение:**
- Выполните новый login

### Ошибка "Connection refused"

**Возможные причины:**
- Auth-Proxy не запущен
- Неверный порт
- Проблемы с сетью

**Решение:**
- Проверьте, что Auth-Proxy запущен: `docker ps | grep auth-proxy`
- Проверьте health check: `curl http://localhost:38081/health`

## Дополнительная информация

- Keycloak Setup: [KEYCLOAK_AUTH_PROXY_SETUP.md](../deploy/prod/KEYCLOAK_AUTH_PROXY_SETUP.md)
- Architecture: [Feat_Authorization.md](../Feat_Authorization.md)
