# Отчет по ветке `feat/UpKeycloak`

## 📋 Краткое описание

Реализована система централизованной аутентификации на базе Keycloak с микросервисом Auth-Proxy, который изолирует Keycloak от прямых запросов клиентской части.

---

##  Что сделано

### 1.  Развертывание Keycloak

- **Поднят Keycloak** на production VPS через Docker Compose
- Настроен realm `otus-ms` для приложения
- Создан Keycloak client `auth-proxy` с Client Credentials
- Настроены времена жизни токенов:
  - Access Token: 5 минут
  - Refresh Token: 30 дней
- Автоматический деплой через GitHub Actions (`.github/workflows/deploy-keycloak.yml`)
- **URL**: https://fishouk-otus-ms.ru/auth/

### 2.  Auth-Proxy микросервис

Создан новый микросервис для изоляции Keycloak от клиентских запросов.

**Реализованные компоненты:**

#### Структура кода
```
cmd/auth-proxy/
├── main.go           # Инициализация сервиса
└── api-server.go     # HTTP сервер с роутами

internal/
├── keycloak/
│   ├── client.go     # Клиент для взаимодействия с Keycloak API
│   └── models.go     # Модели данных (TokenResponse, LoginRequest и т.д.)
└── handlers/auth/
    └── handler.go    # HTTP handlers для auth endpoints
```

#### API Endpoints

**Реализованные хендлеры:**

| Endpoint | Метод | Описание |
|----------|-------|----------|
| `/health` | GET | Health check |
| `/api/v1/auth/login` | POST | Аутентификация (username + password) → токены |
| `/api/v1/auth/refresh` | POST | Обновление access token через refresh token |
| `/api/v1/auth/logout` | POST | Выход и инвалидация токенов |

**Особенности:**
- Проксирует запросы к Keycloak через библиотеку `gocloak`
- Валидация запросов (`go-playground/validator`)
- Структурированное логирование (audit log: user_id, IP)
- Graceful shutdown
- Health checks для мониторинга

### 3. 🧪 Интеграционные тесты

Написан полный набор интеграционных тестов для Auth-Proxy.

**Файл**: `tests/integration/auth_test.go`

**Покрытие (8 тестов):**
-  `TestAuthProxyHealthCheck` - проверка доступности
-  `TestLoginSuccess` - успешный логин
-  `TestLoginInvalidCredentials` - логин с неверными credentials
-  `TestLoginMissingFields` - валидация обязательных полей
-  `TestRefreshTokenSuccess` - обновление токена
-  `TestRefreshTokenInvalid` - невалидный refresh token
-  `TestLogoutSuccess` - logout и инвалидация
-  `TestAuthFullFlow` - полный флоу Login→Refresh→Logout

**Инфраструктура для тестов:**
- Тесты запускаются в Docker контейнерах (`deploy/test/docker-compose.test.yml`)
- Автоматическое поднятие окружения: PostgreSQL + Main Service + Auth-Proxy
- Health checks для ожидания готовности сервисов

### 4. 🔄 CI/CD интеграция

Обновлен GitHub Actions workflow (`.github/workflows/ci.yml`):

**Добавлено в pipeline:**
- Сборка Auth-Proxy (`go build ./cmd/auth-proxy`)
- Запуск Auth-Proxy в фоне на порту 38081
- Использование GitHub Secrets для `KEYCLOAK_CLIENT_SECRET`
- Запуск интеграционных тестов для Auth-Proxy
- Graceful handling если Auth-Proxy не настроен

**GitHub Secrets (требуются):**
- `KEYCLOAK_CLIENT_SECRET` - для запуска Auth-Proxy тестов
- `TEST_KEYCLOAK_USERNAME` (опционально)
- `TEST_KEYCLOAK_PASSWORD` (опционально)
