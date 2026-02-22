# Обновление Production конфигураций

## Проблема

В production конфигах отсутствовали секции JWT и Keycloak, что приводило к:
- Незащищенным API endpoints
- Невозможности валидации JWT токенов
- Отсутствию service-to-service аутентификации

## Что нужно обновить на VPS

### 1. Обновить конфигурационные файлы

Подключитесь к VPS и обновите следующие файлы:

#### `/opt/OtusMS/configs/config.prod.yaml`

Добавьте в конец файла:

```yaml
jwt:
  jwks_url: "https://fishouk-otus-ms.ru/auth/realms/otus-ms/protocol/openid-connect/certs"
  issuer: "https://fishouk-otus-ms.ru/auth/realms/otus-ms"
  audience: "main-service"
  cache_duration: 600  # 10 минут
```

**Важно:** Main Service **не использует** Keycloak клиент напрямую. Он только валидирует JWT токены через JWKS Manager. Keycloak клиент используется только в Auth-Proxy.

#### `/opt/OtusMS/configs/config.auth-proxy.prod.yaml`

Обновите секцию `keycloak` и добавьте секцию `main_service`:

```yaml
keycloak:
  url: "https://fishouk-otus-ms.ru/auth"
  realm: "otus-ms"
  client_id: "auth-proxy"
  client_secret: "YOUR_AUTH_PROXY_CLIENT_SECRET_HERE"  # Замените на реальный secret из Keycloak (см. шаг 2)

main_service:
  url: "http://otus-microservice-be-prod:38080"  # Main Service API (через Docker network, имя контейнера)
```

### 2. Получить client secret для Auth-Proxy из Keycloak

**Где взять client secret:**

1. Откройте Keycloak Admin Console: `https://fishouk-otus-ms.ru/auth/admin`
2. Выберите realm: `otus-ms`
3. Перейдите в `Clients` → `auth-proxy` → вкладка `Credentials` → скопируйте `Secret`
4. Вставьте этот secret в поле `client_secret` в `/opt/OtusMS/configs/config.auth-proxy.prod.yaml`

### 3. Настроить Keycloak клиент Auth-Proxy

В Keycloak настройте клиент `auth-proxy`:

1. **Settings**:
   - Client ID: `auth-proxy`
   - Client Protocol: `openid-connect`
   - Access Type: `confidential`
   - Service Accounts Enabled: `ON`
   - Valid Redirect URIs: `*` (или конкретные URLs)

2. **Service Account Roles** (вкладка `Service Account Roles`):
   - Добавьте роль `manage-users` из `realm-management`
   - Добавьте роль `view-users` из `realm-management`

**Важно:** Main Service **не требует** отдельного клиента в Keycloak, так как он только валидирует JWT токены через публичный JWKS endpoint. Все операции с Keycloak (создание пользователей, аутентификация) выполняет Auth-Proxy.

### 4. Перезапустить сервисы

```bash
cd /opt/OtusMS

# Перезапустить Main Service
docker compose -f deploy/prod/docker-compose.be.prod.yml down
docker compose -f deploy/prod/docker-compose.be.prod.yml up -d

# Перезапустить Auth-Proxy
docker compose -f deploy/prod/docker-compose.auth-proxy.prod.yml down
docker compose -f deploy/prod/docker-compose.auth-proxy.prod.yml up -d
```

### 5. Проверить работу

```bash
# Проверить логи Main Service
docker logs otus-microservice-be-prod --tail 50

# Должны увидеть:
# - "JWKS Manager initialized successfully"
# - НЕ должно быть "JWT not configured - API endpoints are UNPROTECTED!"

# Проверить логи Auth-Proxy
docker logs otus-microservice-auth-proxy-prod --tail 50

# Проверить здоровье сервисов
curl http://localhost:38080/health
curl http://localhost:38081/health
```

## Проверка функциональности

### 1. Проверить логин через Auth-Proxy

```bash
curl -X POST http://localhost:38081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "test@example.com",
    "password": "test123"
  }'
```

Должен вернуть JWT токены.

### 2. Проверить защиту API Main Service

```bash
# Без токена - должен вернуть 401
curl http://localhost:38080/api/v1/users

# С токеном - должен работать
curl http://localhost:38080/api/v1/users \
  -H "Authorization: Bearer <access_token>"
```

### 3. Проверить регистрацию пользователя

```bash
curl -X POST http://localhost:38081/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "password123",
    "first_name": "Test",
    "last_name": "User"
  }'
```

Должен создать пользователя в Keycloak и Main Service.

## Troubleshooting

### "Invalid client or Invalid client credentials"

- Проверьте что client secret в `config.auth-proxy.prod.yaml` правильный
- Проверьте что клиент `auth-proxy` в Keycloak имеет `Service Accounts Enabled: ON`
- Проверьте что секрет скопирован без лишних пробелов и символов
- Перезапустите Auth-Proxy контейнер после изменения конфига

### "JWT not configured"

- Проверьте что в `config.prod.yaml` есть секция `jwt` с заполненными полями
- Проверьте логи на наличие ошибок парсинга конфига
- Убедитесь что `jwks_url` доступен из контейнера

### "Failed to create user in Main Service"

- Проверьте что Auth-Proxy client имеет роли `manage-users` и `view-users`
- Проверьте что Auth-Proxy может получить service account токен
- Проверьте логи Main Service - JWT токен должен валидироваться и содержать `azp: "auth-proxy"` claim
- Проверьте что Main Service видит роль "service-account" (проверяется через claim `azp` или `clientId`)
