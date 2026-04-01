# Alert Worker: быстрый запуск (local + prod)

Короткая инструкция по запуску `alert-worker` с нуля: от Keycloak и Telegram-бота до старта сервиса.

## 1) Подготовка Keycloak (обязательно)

Создайте в Keycloak отдельный client для сервисного аккаунта:

- `client_id`: `alert-worker`
- `Client authentication`: `ON`
- `Service accounts roles`: `ON`
- grant type: `Client Credentials`

Нужен `client_secret` этого клиента.

Заполните:

- локально: `configs/config.alert-worker.local.yaml`
- прод: `configs/config.alert-worker.prod.yaml`

Поля:

- `keycloak.url`
- `keycloak.realm`
- `keycloak.client_id`
- `keycloak.client_secret`

## 2) Подготовка Telegram-бота (обязательно)

1. Создайте бота через [@BotFather](https://t.me/BotFather) и получите `bot_token`.
2. Добавьте бота в целевой чат/группу.
3. Получите `chat_id` этого чата.

Заполните в конфиге:

- `telegram.bot_token`
- `telegram.project_chat_id`

## 3) Локальный запуск

### 3.1 Запустите зависимости

- main-service (должен поднять gRPC на `localhost:50051`)
- Kafka (брокер `localhost:39092`)

Пример:

```bash
docker compose -f deploy/local/docker-compose.local.yml --profile db up -d
docker compose -f deploy/local/docker-compose.kafka.local.yml up -d
go run ./cmd/main-service -config configs/config.local.yaml
```

### 3.2 Проверьте локальный конфиг

Файл: `configs/config.alert-worker.local.yaml`

Ключевые поля:

- `main_service.grpc_addr: localhost:50051`
- `kafka.brokers: ["localhost:39092"]`
- заполненные `keycloak.client_secret`, `telegram.bot_token`, `telegram.project_chat_id`

### 3.3 Запустите alert-worker

```bash
go run ./cmd/alert-worker -config configs/config.alert-worker.local.yaml
```

Проверка health:

```bash
curl http://localhost:38084/health
```

## 4) Прод запуск (VPS)

### 4.1 Подготовьте прод конфиг

Файл: `configs/config.alert-worker.prod.yaml`

Проверьте:

- `main_service.grpc_addr` (должен быть доступен из Docker-сети)
- `kafka.brokers`
- `telegram.bot_token`
- `telegram.project_chat_id`
- `keycloak.client_secret`

### 4.2 Загрузите конфиги и compose на VPS

```bash
task -d . -t deploy/prod/Taskfile.yml config:upload
task -d . -t deploy/prod/Taskfile.yml deploy:compose
```

### 4.3 Запустите сервис

```bash
task -d . -t deploy/prod/Taskfile.yml aw:up
```

Проверки:

```bash
task -d . -t deploy/prod/Taskfile.yml aw:health
task -d . -t deploy/prod/Taskfile.yml aw:logs
```

## 5) Полезные команды (prod)

```bash
task -d . -t deploy/prod/Taskfile.yml aw:restart
task -d . -t deploy/prod/Taskfile.yml aw:down
task -d . -t deploy/prod/Taskfile.yml aw:config:edit
```

## 6) Важно по безопасности

- Не коммитьте реальные `client_secret`, `bot_token`, `chat_id`.
- Файлы `configs/config.*.yaml` уже в `.gitignore`.
