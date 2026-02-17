# Настройка JWT валидации

> Инструкция по настройке JWT валидации с автоматической загрузкой ключей через JWKS

## 📋 Содержание

1. [JWKS Endpoint - автоматическая загрузка ключей](#jwks-endpoint---автоматическая-загрузка-ключей)
2. [Конфигурация main-service](#конфигурация-main-service)
3. [Тестирование JWT](#тестирование-jwt)
4. [Troubleshooting](#troubleshooting)

---

## JWKS Endpoint - автоматическая загрузка ключей

**Единственный способ валидации JWT в проекте!** Keycloak автоматически публикует свои публичные ключи.

**Преимущества:**
- ✅ Не нужно вручную копировать ключи
- ✅ Автоматическое обновление при rotation ключей
- ✅ Стандартный подход (RFC 7517)
- ✅ Кеширование ключей в памяти (обновление раз в 10 минут)

**Конфигурация:**
```yaml
jwt:
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"
  cache_duration: 600  # 10 минут
```

**Всё!** Больше ничего делать не нужно. Сервис сам загрузит и будет обновлять ключи.

### ⚠️ Способ 2: Статический публичный ключ (legacy)

Устаревший способ - вручную копировать ключ. Используйте только если JWKS недоступен.

---

## JWKS Endpoint (рекомендуется)

### Шаг 1: Узнайте JWKS URL

JWKS URL формируется по шаблону:
```
{keycloak_url}/realms/{realm_name}/protocol/openid-connect/certs
```

**Для вашего проекта:**
```
https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs
```

### Шаг 2: Проверьте доступность

```bash
curl https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs

# Должны увидеть JSON с ключами:
{
  "keys": [
    {
      "kid": "X7eKzXgmfS_...",
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "n": "xGOr-H0A-6...",
      "e": "AQAB",
      "x5c": [...],
      "x5t": "...",
      "x5t#S256": "..."
    }
  ]
}
```

### Шаг 3: Добавьте в конфигурацию

**Локальная конфигурация** (`configs/config.local.yaml`):

```yaml
jwt:
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"  # Ваш Client ID
  cache_duration: 600       # 10 минут (опционально)
```

**Production конфигурация** (`configs/config.prod.yaml`):

```yaml
jwt:
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"
  cache_duration: 600
```

### Шаг 4: Готово!

Больше ничего делать не нужно. Сервис:
1. При старте загрузит ключи из JWKS endpoint
2. Будет кешировать их в памяти
3. Автоматически обновит через 10 минут
4. При rotation ключей в Keycloak - подхватит новые автоматически


## Конфигурация main-service

### Локальная конфигурация

**Файл:** `configs/config.local.yaml`

```yaml
jwt:
  # JWKS URL для автоматической загрузки ключей
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  
  # Issuer - URL вашего Keycloak realm
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  
  # Audience - Client ID вашего микросервиса
  audience: "main-service"
  
  # Время кеширования JWKS (опционально)
  cache_duration: 600  # 10 минут
```

### Production конфигурация

**Файл:** `configs/config.prod.yaml`

```yaml
jwt:
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"
  cache_duration: 600
```

### Локальная разработка (альтернатива)

Если используете локальный Keycloak (на localhost):

```yaml
jwt:
  jwks_url: "http://localhost:8080/realms/otus-realm/protocol/openid-connect/certs"
  issuer: "http://localhost:8080/realms/otus-realm"
  audience: "main-service"
```

---

## Тестирование JWT

### 1. Получение JWT токена

```bash
# Логин через Keycloak
curl -X POST http://localhost:8080/realms/otus-realm/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=otus-client" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "username=test@example.com" \
  -d "password=password123" \
  -d "grant_type=password"

# Ответ:
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI...",
  "expires_in": 300,
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI...",
  "token_type": "Bearer"
}
```

### 2. Тестирование эндпоинта с JWT

```bash
# Сохраните токен в переменную
TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI..."

# Тест с валидным токеном
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:38080/api/v1/users/me

# Ожидается: 200 OK + данные пользователя
```

### 3. Проверка декодирования JWT

Используйте https://jwt.io для декодирования токена:

```json
{
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email": "test@example.com",
  "given_name": "Test",
  "family_name": "User",
  "realm_access": {
    "roles": ["user"]
  },
  "iss": "http://localhost:8080/realms/otus-realm",
  "exp": 1708012800
}
```

### 4. Негативные тесты

```bash
# Тест без токена
curl http://localhost:38080/api/v1/users/me
# Ожидается: 401 Unauthorized

# Тест с невалидным токеном
curl -H "Authorization: Bearer invalid_token" \
  http://localhost:38080/api/v1/users/me
# Ожидается: 401 Unauthorized

# Тест с истёкшим токеном
curl -H "Authorization: Bearer $EXPIRED_TOKEN" \
  http://localhost:38080/api/v1/users/me
# Ожидается: 401 Unauthorized
```

---

## Troubleshooting

### Ошибка: "unexpected signing method"

**Причина:** Токен подписан не RSA256 алгоритмом

**Решение:**
1. Проверьте алгоритм в Keycloak: Realm Settings → Keys
2. Убедитесь, что используется RS256, а не HS256

### Ошибка: "failed to parse public key"

**Причина:** Неверный формат публичного ключа

**Решение:**
1. Проверьте формат PEM:
   ```
   -----BEGIN PUBLIC KEY-----
   (base64 строки)
   -----END PUBLIC KEY-----
   ```
2. Убедитесь, что нет лишних пробелов
3. Проверьте, что ключ скопирован полностью

### Ошибка: "invalid token issuer"

**Причина:** Issuer в токене не совпадает с конфигурацией

**Решение:**
1. Декодируйте токен на jwt.io
2. Проверьте поле `iss` в токене
3. Обновите `jwt.issuer` в конфигурации, чтобы совпадало

**Пример:**
```yaml
# Неправильно
issuer: "http://localhost:8080"

# Правильно
issuer: "http://localhost:8080/realms/otus-realm"
```

### Ошибка: "token is expired"

**Причина:** Токен истёк

**Решение:**
1. Получите новый токен через `/token` endpoint
2. Или используйте refresh token для обновления
3. Проверьте время жизни токена в Keycloak: Realm Settings → Tokens

### Логирование для отладки

Включите debug логи в конфигурации:

```yaml
log:
  level: debug  # Включит детальные логи JWT валидации
```

Вы увидите в логах:
```
DEBUG JWT validated successfully user_id=550e8400-... email=test@example.com
WARN invalid JWT token error="token is expired"
```

---

**Обновлено:** 15 февраля 2026
