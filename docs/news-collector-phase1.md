# News Collector Service — Фаза 1: получение источников через gRPC

## Статус

**Реализовано и задеплоено на production** (февраль 2026)

---

## Что реализовано

### Новый сервис `news-collector`

Отдельный микросервис, который при старте получает список источников новостей от `main-service` по gRPC.

```
cmd/news-collector/
├── main.go         # точка входа: инициализация, gRPC-запрос, запуск HTTP-сервера
└── api-server.go   # HTTP сервер (chi router, /health endpoint, graceful shutdown)
```

**Жизненный цикл при запуске:**
1. Читает конфиг (`-config` флаг)
2. Инициализирует Keycloak-клиент (service account)
3. Создаёт gRPC-клиент к `main-service`
4. Делает `GetNewsSources` запрос — получает список источников
5. Запускает HTTP-сервер на `:38082` (health check)
6. Ждёт `SIGINT`/`SIGTERM`, выполняет graceful shutdown

---

### gRPC контракт

**Proto:** `proto/news_sources/v1/news_sources.proto`  
**Сгенерированный код:** `pkg/news_sources/v1/`

```protobuf
service NewsSourcesService {
  rpc GetNewsSources(GetNewsSourcesRequest) returns (GetNewsSourcesResponse);
}

message NewsSource {
  string id             = 1;
  string name           = 2;
  string url            = 3;
  string language       = 4;
  string category       = 5;
  int32  fetch_interval = 6;  // секунды
  bool   is_active      = 7;
}
```

---

### Изменения в `main-service`

| Компонент | Файл | Описание |
|-----------|------|----------|
| gRPC сервер | `cmd/main-service/grpc-server.go` | Поднимает gRPC на `:50051`, регистрирует `NewsSourcesServiceServer` |
| gRPC хендлер | `internal/handlers/sources/grpc_handler.go` | Читает источники из БД через `SourceRepository`, конвертирует в proto |
| JWT Middleware | `internal/middleware/grpc_auth.go` | Перехватчик — валидирует Bearer-токен через Keycloak JWKS перед каждым RPC |

---

### gRPC клиент в `news-collector`

**Файл:** `internal/clients/mainservice/grpc_client.go`

- Подключается к `main-service` по адресу из конфига (`main_service.grpc_addr`)
- Перед каждым запросом получает `service account` JWT от Keycloak через `TokenProvider`
- Добавляет токен в gRPC metadata: `authorization: Bearer <token>`
- Конвертирует proto-ответ в `[]models.Source`

---

### Модель данных

**Файл:** `internal/models/source.go`

```go
type Source struct {
    ID            string       // UUID источника
    Name          string       // название (напр. "BBC News")
    URL           string       // RSS/Atom URL
    Language      string       // язык (напр. "en")
    Category      string       // категория (напр. "world")
    FetchInterval int          // интервал сбора в секундах
    IsActive      bool         // активен ли источник
    LastCollectedAt sql.NullTime
    LastError       sql.NullString
    ErrorCount      int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

Вспомогательные методы:
- `NextFetchAt()` — время следующего запланированного сбора
- `IsDue()` — пора ли собирать новости сейчас

---

## Конфигурация

### `configs/config.news-collector.*.yaml`

```yaml
global:
  env: prod              # local | prod

log:
  level: info            # debug | info | warn | error
  format: json           # json (prod) | text (local)
  service_name: news-collector

servers:
  client:
    addr: 0.0.0.0:38082  # HTTP health check

keycloak:
  url: "https://fishouk-otus-ms.ru/auth"
  realm: "otus-ms"
  client_id: "news-collector"      # service account клиент в Keycloak
  client_secret: "<secret>"        # хранится только локально/на VPS, не в git

main_service:
  grpc_addr: "otus-microservice-be-prod:50051"  # имя контейнера в Docker-сети
```

### `configs/config.prod.yaml` (main-service) — добавлена секция gRPC

```yaml
servers:
  grpc:
    addr: 0.0.0.0:50051  # gRPC для news-collector
```

---

## Деплой

### Инфраструктура на VPS

```
/root/otus-microservice/prod/
├── be/                                        # main-service
│   └── configs/config.prod.yaml              # теперь включает servers.grpc
└── news-collector/                            # новый сервис
    ├── docker-compose.news-collector.prod.yml
    ├── configs/
    │   └── config.news-collector.prod.yaml   # содержит keycloak secret (не в git)
    └── logs/
```

Docker-сеть `otus_network` — оба контейнера видят друг друга по имени.  
gRPC порт `50051` **не пробрасывается** на хост — только внутри сети.

### CI/CD

В `.github/workflows/ci.yml` добавлен джоб `build-and-deploy-news-collector`:
- Зависит от `build-and-deploy` (main-service) — деплоится **строго после** него
- Перед запуском контейнера ждёт healthcheck main-service (до 60 сек)
- Собирает образ из `news-collector.Dockerfile`
- Пушит в Selectel Registry: `cr.selcloud.ru/otus-microservice-be/news-collector:latest`

### Управление через Taskfile

```bash
# Залить конфиги (все сервисы, включая news-collector)
task -d . -t deploy/prod/Taskfile.yml config:upload

# Первый запуск / обновление news-collector
task -d . -t deploy/prod/Taskfile.yml nc:up

# Логи
task -d . -t deploy/prod/Taskfile.yml nc:logs

# Перезапуск
task -d . -t deploy/prod/Taskfile.yml nc:restart

# Healthcheck
task -d . -t deploy/prod/Taskfile.yml nc:health
```

---

## Что не реализовано (следующие фазы)

- [ ] Периодический опрос источников по расписанию (на основе `FetchInterval`)
- [ ] Парсинг RSS/Atom фидов
- [ ] Сохранение собранных новостей (`raw_news`)
- [ ] Дедупликация новостей по URL/хешу
- [ ] Метрики (Prometheus)
- [ ] Передача собранных новостей обратно в `main-service`
