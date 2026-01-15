# OtusMS - Microservice Template

Шаблон микросервиса на Go для курса OTUS.

## Быстрый старт

### Локальная разработка

```bash
# Установка зависимостей
go mod download

# Запуск сервиса
go run ./cmd/main-service -config configs/config.local.yaml

# Или через task
task build
./main-service -config configs/config.local.yaml
```

### Проверка работы

```bash
# Healthcheck
curl http://localhost:38080/health

# Главная страница
curl http://localhost:38080/
```

### С Docker Compose

```bash
# Запуск
docker compose -f deploy/local/docker-compose.local.yml up -d

# Логи
docker compose -f deploy/local/docker-compose.local.yml logs -f

# Остановка
docker compose -f deploy/local/docker-compose.local.yml down
```

## Разработка

### Требования

- Go 1.23.8+
- Docker (опционально)
- Task (опционально, для удобства)

### Структура проекта

```
.
├── cmd/
│   └── main-service/       # Точка входа приложения
├── internal/
│   └── config/             # Конфигурация
├── configs/                # Файлы конфигурации
├── deploy/                 # Docker compose и деплой
├── docs/                   # Документация
├── pkg/                    # Публичные библиотеки
└── tests/                  # Тесты
```

### Доступные команды (Task)

```bash
task                 # Полный цикл: tidy, fmt, lint, tests, build
task build          # Сборка проекта
task fmt            # Форматирование кода
task lint           # Запуск линтера
task tests          # Запуск тестов
task tidy           # go mod tidy + vendor
```

## API Endpoints

### `GET /`
Приветственное сообщение

**Response:**
```json
{
  "message": "Welcome to OtusMS Microservice!",
  "version": "1.0.0",
  "status": "running"
}
```

### `GET /health`
Проверка здоровья сервиса

**Response:**
```json
{
  "status": "ok",
  "time": "2026-01-15T17:30:59+03:00"
}
```

## Конфигурация

Конфигурация загружается из YAML файлов:

- `configs/config.local.yaml` - для локальной разработки
- `configs/config.prod.yaml` - для production

### Пример конфигурации

```yaml
global:
  env: local

log:
  level: debug

servers:
  debug:
    addr: localhost:33000
  client:
    addr: localhost:38080
    allow_origins:
      - "*"
```

## Деплой

См. [deploy/README.md](deploy/README.md) для инструкций по деплою в production.

## CI/CD

GitHub Actions автоматически:
- ✅ Проверяет код линтером
- ✅ Запускает тесты
- ✅ Собирает Docker образ
- ✅ Деплоит на production сервер

## Порты

- `38080` - HTTP API
- `33000` - Debug/pprof

## Лицензия

MIT

