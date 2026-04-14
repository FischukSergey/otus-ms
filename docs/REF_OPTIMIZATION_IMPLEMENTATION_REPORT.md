# Отчет по реализации в ветке `ref/Optimization`

Документ фиксирует, какие задачи из учебной инициативы были реализованы в коде ветки `ref/Optimization`, каким способом, и что осталось невыполненным.

## Контекст учебной задачи

Изначально ставились 4 направления:

1. Практические рекомендации и изменения по CQRS.
2. Практические рекомендации и изменения по Saga/Temporal.
3. Повышение надежности распределенной системы.
4. Рефакторинг в сторону DDD.

В данной ветке выполнена часть пунктов: существенные изменения по надежности Kafka-потоков и CQRS-light для personalization feed, плюс операционные улучшения rollout/rollback.

## Краткая сводка статуса

- **CQRS**: реализовано частично (целевой сценарий `personalization feed`).
- **Saga/Temporal**: в коде ветки не внедрено.
- **Надежность распределенной системы**: реализовано частично (consumer-side надежность, эксплуатационные процедуры).
- **DDD-рефакторинг**: реализовано частично в рамках локального разделения ответственности (service/store split), без полной доменной декомпозиции.

## 1) Что реализовано по надежности распределенной системы

### 1.1. Устранение риска потери сообщений в Kafka consumer-потоке

#### Что было до изменений
- Consumers использовали `ReadMessage`, что автоматически фиксирует смещение, и при ошибке обработки часть сообщений могла теряться.

#### Что сделано
- В `news-processor` внедрен explicit commit:
  - `FetchMessage` вместо `ReadMessage`;
  - `CommitMessages` только после успешной обработки;
  - при ошибке обработки offset не коммитится (сообщение переобрабатывается);
  - для невалидного JSON добавлен skip + commit, чтобы не блокировать партицию poison-message.
- В `alert-worker` внедрен explicit commit:
  - `FetchMessage` вместо `ReadMessage`;
  - commit после успешной обработки;
  - retryable ошибки помечаются и приводят к повторному чтению (без commit);
  - для невалидного JSON — skip + commit.

#### Где реализовано
- `internal/services/processor/service.go`
- `internal/services/alertworker/service.go`

#### Дополнительные защитные механизмы
- Добавлен `commitWithRetry` с backoff в обоих сервисах:
  - `commitRetryAttempts = 3`
  - `commitInitialRetryBackoff = 100ms`
  - `commitMaxRetryBackoff = 800ms`

### 1.2. Тесты на надежность новой commit-схемы

Добавлены unit-тесты, проверяющие ключевые инварианты:

- Для `processor`:
  - commit при успешной обработке;
  - отсутствие commit при ошибке обработки;
  - retries commit при ошибке фиксации.
- Для `alert-worker`:
  - commit после успешной обработки;
  - отсутствие commit при retryable ошибке;
  - retries commit при ошибке фиксации.

#### Где реализовано
- `internal/services/processor/service_test.go`
- `internal/services/alertworker/service_test.go`

### 1.3. Операционные улучшения прод-выкатки

Добавлены безопасные задачи rollout/rollback для потоковых сервисов:

- `np:rollout:safe`, `np:rollback`
- `aw:rollout:safe`, `aw:rollback`

Включены шаги health-check и проверка consumer lag через Kafka CLI.

#### Где реализовано
- `deploy/prod/Taskfile.yml`

### 1.4. Обновление runbook по семантике доставки

В runbook зафиксирована новая семантика at-least-once:
- когда offset коммитится;
- когда сообщение читается повторно;
- почему downstream операции должны быть идемпотентными.

#### Где реализовано
- `docs/ALERT_WORKER_RUNBOOK.md`

## 2) Что реализовано по CQRS (Personalization Feed)

Реализован CQRS-light без изменения внешнего HTTP API.

### 2.1. Разделение read/write на уровне репозиториев

#### Что сделано
- Из общего personalization-репозитория вынесен feed SQL в отдельный query-репозиторий.
- В write-репозитории оставлены только command-операции preferences/events.

#### Где реализовано
- Write-репозиторий: `internal/store/personalization/repository.go`
- Query-репозиторий: `internal/store/personalization/feed_query_repository.go`

### 2.2. Разделение read/write на уровне сервисов

#### Что сделано
- Введен `FeedQueryService` (read-path).
- Введен `PreferencesEventCommandService` (command-path).
- Сохранен фасад `Service`, который делегирует вызовы во внутренние сервисы, чтобы не ломать handler API.

#### Где реализовано
- Фасад: `internal/services/personalization/service.go`
- Command-сервис: `internal/services/personalization/command_service.go`
- Query-сервис: `internal/services/personalization/feed_query_service.go`

### 2.3. Обновление DI и тестов

#### Что сделано
- DI в `main-service` обновлен: отдельно создаются write/query personalization repositories, затем собирается фасадный сервис.
- Тесты адаптированы под разделенные зависимости (отдельные mock для command/query).

#### Где реализовано
- DI: `cmd/main-service/api-server.go`
- Тесты: `internal/services/personalization/service_test.go`

### 2.4. Что важно: API-контракт не изменен

Маршруты и контракт хендлеров сохранены:
- `/api/v1/news/feed`
- `/api/v1/users/me/preferences`
- `/api/v1/news/events`

Хендлеры не потребовали изменения публичных сигнатур.

## 3) Что по Saga/Temporal в этой ветке

В кодовой базе ветки `ref/Optimization` внедрение Temporal/Saga **не выполнено**.

Что сделано фактически:
- улучшена надежность событийной обработки на уровне consumer поведения и retry/commit;
- создана эксплуатационная основа для безопасного выката/отката потоковых сервисов.

Что остается (следующий этап):
- оркестрация бизнес-транзакций (например, регистрация пользователя Keycloak + main-service) через workflow/activities;
- централизованные compensation-процессы и workflow-история.

## 4) Что по DDD-рефакторингу в этой ветке

Полный DDD-рефакторинг в ветке не выполнялся, но сделан важный шаг в этом направлении:

- разделена ответственность read и command в personalization;
- reduced coupling между сценариями ленты и сценариями preferences/events;
- введены более узкие интерфейсы зависимостей.

Это соответствует эволюционному подходу DDD: сначала локальная декомпозиция в bounded area, без big-bang переделки всей системы.

## 5) Итог по учебной задаче

### Решенные пункты
- Практически устранен consumer-side риск потери сообщений (Kafka explicit commit + retry + тесты).
- Реализован CQRS-light для personalization feed (store/service/DI/tests split).
- Улучшен operational контур прод-выкатки и rollback для потоковых сервисов.

### Частично решенные пункты
- DDD: выполнено локальное структурное разделение, но не full-domain decomposition.

### Не решенные в коде ветки пункты
- Saga/Temporal (пока на уровне рекомендаций и архитектурного направления, без реализации workflow).

---

Если требуется, можно подготовить отдельный roadmap `Phase 2` с минимальным внедрением Temporal для одного критического потока (регистрация), не затрагивая текущие API-контракты.
