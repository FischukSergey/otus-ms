# Интеграционные тесты

Этот пакет содержит интеграционные тесты для API сервиса OtusMS.

## Подход

Интеграционные тесты используют **реальный production-like стек** и поддерживают два режима:

### Локально (Docker Compose)
- API сервер в Docker контейнере (как в production)
- PostgreSQL в Docker контейнере
- Тесты → http://localhost:8081

### CI/CD (GitHub Actions)
- PostgreSQL в service container (быстрее)
- API сервер на хосте (собранный бинарник)
- Тесты → http://localhost:8080

## Запуск тестов

**Важно:** Интеграционные тесты используют build tag `integration` для разделения от unit-тестов.

### Быстрый старт

```bash
# Запуск интеграционных тестов (все автоматически)
task test:integration

# Или напрямую с тегом
go test -tags=integration -v ./tests/integration/...
```

Эта команда:
1. Останавливает старые контейнеры
2. Поднимает PostgreSQL + API сервер
3. Ждет готовности (healthcheck)
4. Запускает тесты
5. Останавливает и удаляет контейнеры

### Ручной режим (для отладки)

```bash
# 1. Поднять окружение
task test:env:up

# 2. Запустить тесты
go test -v ./tests/integration/...

# 3. Проверить API вручную
curl http://localhost:8081/health

# 4. Посмотреть логи
task test:env:logs

# 5. Остановить окружение
task test:env:down
```

## Структура тестов

### user_test.go

**TestUserBasicFlow** - основной флоу:
1. POST /api/v1/users - создание пользователя
2. GET /api/v1/users/{uuid} - получение пользователя
3. DELETE /api/v1/users/{uuid} - мягкое удаление пользователя
4. GET /api/v1/users/{uuid} - проверка что пользователь помечен как удаленный (deleted=true)

**TestUserValidation** - проверка валидации:
- Невалидный UUID
- Невалидный email
- Ошибки возвращают 400 Bad Request

**TestHealthCheck** - проверка health endpoint

## Конфигурация

### config.test.yaml

```yaml
db:
  host: postgres      # Имя сервиса в Docker Compose
  port: "5432"        # Внутренний порт в сети Docker
  name: otus_ms_test
  user: otus_ms_test
  password: otus_ms_test
```

### docker-compose.test.yml

**PostgreSQL:**
- Внешний порт: 38433
- Внутренний порт: 5432
- Healthcheck: pg_isready

**API Server:**
- Внешний порт: 8081
- Внутренний порт: 8080
- Healthcheck: wget /health
- depends_on: postgres (с condition: service_healthy)

## CI/CD

В GitHub Actions используется **оптимизированный подход** с service containers:

```yaml
test-integration:
  services:
    postgres:  # PostgreSQL как service container
      image: postgres:16-alpine
      ports: [38433:5432]
      
  steps:
    - Build приложения
    - Запуск API сервера в фоне
    - Ожидание готовности сервера
    - Запуск тестов (TEST_SERVER_URL=localhost:8080)
    - Остановка сервера
```

**Преимущества service containers:**
- ⚡ Быстрее чем docker-compose
- 🔒 Изолированная БД для каждого job
- 🚀 Параллельное выполнение jobs
- 📦 GitHub Actions управляет жизненным циклом

**Конфигурация:**
- Локально: `config.test.yaml` (host: postgres, port: 5432)
- CI: `config.ci.yaml` (host: localhost, port: 38433)

## Преимущества подхода

✅ **Простота** - тесты это просто HTTP запросы
✅ **Production-like** - тестируем реальный собранный сервер
✅ **Отладка** - можно проверить API руками (curl, Postman)
✅ **Изоляция** - все в Docker, не влияет на локальную БД
✅ **Реиспользование** - тот же Dockerfile что и в production
