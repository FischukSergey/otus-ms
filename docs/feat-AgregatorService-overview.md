# feat/AgregatorService — обзор реализованных изменений

## Общая цель ветки

Расширение монолитного `main-service` до микросервисной архитектуры:
- Добавление отдельного сервиса аутентификации (`auth-proxy`)
- Добавление сервиса агрегации новостей (`news-collector`)
- Внедрение безопасной межсервисной коммуникации через JWT + gRPC
- Веб-клиент, Swagger UI, централизованное логирование

---

## Новые сервисы

### 1. `auth-proxy` — прокси аутентификации

**Роль:** единственная точка входа для всех операций с Keycloak.  
**Dockerfile:** `auth-proxy.Dockerfile`  
**Порт:** `:38081`

**HTTP API:**
| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/auth/login` | Аутентификация через Keycloak (username + password → JWT) |
| POST | `/api/v1/auth/logout` | Инвалидация refresh token |
| POST | `/api/v1/auth/refresh` | Обновление access token |
| POST | `/api/v1/auth/register` | Регистрация нового пользователя |

**При регистрации** `auth-proxy` выполняет транзакционную операцию:
1. Создаёт пользователя в Keycloak
2. Вызывает `main-service` HTTP API (`POST /api/v1/users`) с service account токеном
3. При ошибке создания в `main-service` — откатывает создание в Keycloak

Это и есть **Lazy User Creation**: пользователь синхронизируется в основную БД в момент первой регистрации.

---

### 2. `news-collector` — агрегатор новостей (Фаза 1)

**Роль:** сбор источников новостей от `main-service` по gRPC и (в следующих фазах) парсинг RSS/Atom.  
**Dockerfile:** `news-collector.Dockerfile`  
**Порт:** `:38082` (только health check)

**Что делает при старте:**
1. Получает service account JWT от Keycloak
2. Вызывает `main-service` gRPC метод `GetNewsSources` с JWT в metadata
3. Логирует полученные источники (9 активных в prod)
4. Запускает HTTP сервер с `/health`

---

### 3. `client` — Streamlit веб-клиент

**Роль:** UI для демонстрации и тестирования API.  
**Dockerfile:** `client.Dockerfile`

**Возможности:**
- Форма авторизации (login/logout)
- Управление пользователями
- Панель статуса микросервисов (healthcheck всех сервисов, включая Loki)

---

## Архитектурные решения

### Keycloak как IAM

Все сервисы используют Keycloak как единственный источник аутентификации:
- Пользователи хранятся в Keycloak, не в PostgreSQL
- `auth-proxy` — единственный сервис, знающий Keycloak client secret для управления пользователями
- Остальные сервисы только **валидируют** JWT токены через JWKS

### JWT + JWKS валидация

**Файл:** `internal/jwks/manager.go`, `internal/middleware/jwt.go`

`JWKSManager` — singleton с автоматическим обновлением публичных ключей Keycloak:
- Кеширует JWKS (по умолчанию 10 минут)
- Поддерживает фоновое обновление ключей
- Используется в `main-service` и `news-collector`

`ValidateJWT` middleware:
- Извлекает Bearer токен из `Authorization` header
- Верифицирует подпись через JWKS
- Помещает `JWTClaims` в контекст запроса

### RBAC (Role-Based Access Control)

**Файлы:** `internal/middleware/rbac.go`, `internal/middleware/jwt_claims.go`

Роли читаются из `realm_access.roles` в JWT. Три уровня:
- `RequireAdmin` — только администраторы
- `RequireUser` — пользователи и администраторы
- `RequireServiceAccount` — только service account токены (межсервисные вызовы)

**`JWTClaims`** умеет различать обычные токены и service account токены Keycloak (по наличию `azp`/`clientId` клейма).

### Межсервисная коммуникация

#### HTTP (auth-proxy → main-service)

`auth-proxy` вызывает `main-service` с **service account** JWT токеном:
```
auth-proxy  →  [Client Credentials Flow]  →  Keycloak  →  JWT
auth-proxy  →  POST /api/v1/users + Bearer JWT  →  main-service
```

Клиент: `internal/clients/mainservice/client.go`

#### gRPC (news-collector → main-service)

`news-collector` вызывает gRPC `GetNewsSources` с **service account** JWT в gRPC metadata:
```
news-collector  →  [Client Credentials Flow]  →  Keycloak  →  JWT
news-collector  →  gRPC GetNewsSources + metadata:authorization  →  main-service
```

Клиент: `internal/clients/mainservice/grpc_client.go`  
Интерцептор на стороне `main-service`: `internal/middleware/grpc_auth.go`

---

## Изменения в `main-service`

### gRPC сервер

**Файл:** `cmd/main-service/grpc-server.go`

- Новый gRPC сервер на `:50051`
- JWT auth interceptor на всех методах
- Зарегистрирован `NewsSourcesServiceServer`

### Новые HTTP эндпоинты

- `POST /api/v1/users` — создание пользователя (только `service-account` роль)

### Новая таблица `news_sources`

**Миграция:** `internal/store/migrations/002_create_news_sources.sql`

```sql
CREATE TABLE news_sources (
    id             VARCHAR(100) PRIMARY KEY,
    name           VARCHAR(200) NOT NULL,
    url            VARCHAR(1000) NOT NULL,
    language       VARCHAR(10),
    category       VARCHAR(50),
    fetch_interval INTEGER NOT NULL DEFAULT 3600,  -- секунды
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    last_collected_at TIMESTAMPTZ,
    last_error     TEXT,
    error_count    INTEGER NOT NULL DEFAULT 0,
    ...
);
```

Seed: **9 русскоязычных источников** (Lenta, RBC, Habr, Kommersant, Fontanka, РИА, Газета.ru, Forbes, VC.ru).

Индексы: `is_active`, `(last_collected_at, fetch_interval)` — для эффективного планировщика (следующие фазы).

### Proto / Buf

**Файлы:** `proto/news_sources/v1/news_sources.proto`, `buf.yaml`, `buf.gen.yaml`

```protobuf
service NewsSourcesService {
  rpc GetNewsSources(GetNewsSourcesRequest) returns (GetNewsSourcesResponse);
}
```

Генерированный код: `pkg/news_sources/v1/`

---

## Инфраструктура

### Loki + Promtail — централизованное логирование

- `configs/loki.yml`, `configs/promtail.yml`
- `deploy/prod/docker-compose.loki.prod.yml`
- Promtail собирает Docker-логи всех контейнеров, фильтрует internal Streamlit-запросы
- Отображение статуса Loki в Streamlit Dashboard

### Swagger UI

- Автогенерация документации (`swag init`) для `main-service` и `auth-proxy`
- Документация коммитится в `api/mainservice/` и `api/authproxy/`
- Разворачивается на VPS как отдельный контейнер: `docker-compose.swagger.prod.yml`

### CI/CD

В `.github/workflows/ci.yml` добавлены джобы:
| Джоб | Что делает |
|------|-----------|
| `build-and-deploy` | Собирает и деплоит `main-service` |
| `build-and-deploy-auth-proxy` | Собирает и деплоит `auth-proxy` |
| `build-and-deploy-news-collector` | Собирает и деплоит `news-collector` (после main-service) |

Дополнительные workflows:
- `deploy-loki.yml` — деплой Loki/Promtail
- `deploy-streamlit.yml` — деплой Streamlit клиента
- `deploy-swagger.yml` — деплой Swagger UI

### Образы в Selectel Registry

| Сервис | Образ |
|--------|-------|
| main-service | `cr.selcloud.ru/otus-microservice-be/backend:latest` |
| auth-proxy | `cr.selcloud.ru/otus-microservice-be/auth-proxy:latest` |
| news-collector | `cr.selcloud.ru/otus-microservice-be/news-collector:latest` |
| client | `cr.selcloud.ru/otus-microservice-be/client:latest` |

---

## Документация

Добавлена в `docs/`:
| Файл | Содержание |
|------|-----------|
| `JWT_SETUP.md` | Настройка JWT / JWKS валидации |
| `RBAC_GUIDE.md` | Руководство по ролевой модели |
| `KEYCLOAK_AUTH_PROXY_SETUP.md` | Настройка Keycloak для auth-proxy |
| `KEYCLOAK_DEPLOYMENT.md` | Деплой Keycloak на VPS |
| `KEYCLOAK_REGISTER_SETUP.md` | Настройка регистрации через Keycloak |
| `SWAGGER_SETUP.md` | Генерация и деплой Swagger |
| `QUICKSTART_METRICS.md` | Быстрый старт с Prometheus/Grafana |
| `news-collector-phase1.md` | Детальное описание Фазы 1 news-collector |

---

## Что не реализовано (следующие фазы)

- [ ] Периодический опрос RSS/Atom источников по `FetchInterval`
- [ ] Парсинг и нормализация новостей в `raw_news`
- [ ] Дедупликация по URL/хешу
- [ ] Передача новостей обратно в `main-service` (через Kafka или gRPC)
- [ ] Метрики `news-collector` (Prometheus)
- [ ] Управление источниками через API (`main-service`)
