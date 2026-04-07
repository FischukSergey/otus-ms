# План настройки окружения для `alert-worker`

Цель: подготовить полный контур эксплуатации `alert-worker` для local/prod с мониторингом, логами, админкой и CI/CD.

## 0) Что уже сделано

- Добавлен сервис `alert-worker` и его прод compose: `deploy/prod/docker-compose.alert-worker.prod.yml`.
- Добавлены конфиги `configs/config.alert-worker.local.yaml` и `configs/config.alert-worker.prod.yaml`.
- Добавлены команды `aw:*` и загрузка конфига в `deploy/prod/Taskfile.yml`.
- Воркер переведен на модель без прямого доступа к БД `main-service` (через gRPC).

---

## 1) Логи: Loki/Promtail должны собирать новый сервис

### 1.1 Обновить фильтрацию контейнеров в Promtail

Сейчас regex в `configs/promtail.yml`:
- `otus-(microservice|news)-.*`

Он **не включает** `otus-alert-worker-prod`. Нужно расширить regex, например:
- `otus-(microservice|news|alert)-.*`

### 1.2 Проверка labels для удобной фильтрации

- Убедиться, что в логах `alert-worker` есть `service=alert-worker`.
- Если нет, добавить это в формат логирования (единообразно с остальными сервисами).

### 1.3 Валидация на проде

- После деплоя Loki/Promtail выполнить:
  - `task -d . -t deploy/prod/Taskfile.yml aw:logs`
  - запрос к Loki с фильтром `container="otus-alert-worker-prod"`.
- Убедиться, что логи появляются и обновляются.

---

## 2) Streamlit Admin: статус сервиса и его логи

### 2.1 Дашборд статуса

Обновить `client/api.py`:
- добавить `alert_worker_url()` (env `ALERT_WORKER_URL`, default `http://localhost:38084`).

Обновить `client/app.py`:
- в `render_dashboard()` добавить карточку `Alert-worker` с `health_check(alert_worker_url(), "Alert-worker")`.
- отобразить URL в подписи под дашбордом.

### 2.2 Раздел логов

В `client/app.py`, список `_SERVICES` дополнить:
- `("alert-worker", "otus-alert-worker-prod")`

### 2.3 Документация и env-шаблон

- Обновить `client/.env.example` (добавить `ALERT_WORKER_URL`).
- Обновить `client/README.md` (новый сервис в дашборде и логах).

### 2.4 Проверка

- Локально: Streamlit показывает `alert-worker` в status и в выпадающем списке логов.
- Прод: аналогично после деплоя.

---

## 3) CI/CD: сборка и деплой `alert-worker` из GitHub Actions

### 3.1 Docker image для сервиса

Добавить `alert-worker.Dockerfile` (по шаблону `news-processor.Dockerfile`):
- сборка `./cmd/alert-worker`;
- запуск с `-config /app/configs/config.alert-worker.prod.yaml`.

### 3.2 Workflow

Вариант A (рекомендовано): расширить `.github/workflows/ci.yml`:
- добавить env:
  - `AW_IMAGE_NAME=alert-worker`
  - `AW_VPS_PATH=/root/otus-microservice/prod/alert-worker`
  - `AW_COMPOSE_FILE=docker-compose.alert-worker.prod.yml`
- добавить job `build-and-deploy-alert-worker`:
  - depends on `build-and-deploy` (main-service),
  - build/push образа,
  - копирование compose,
  - проверка `configs/config.alert-worker.prod.yaml`,
  - `docker compose up -d` в `AW_VPS_PATH`.

Вариант B: отдельный workflow `deploy-alert-worker.yml` по аналогии с `deploy-streamlit.yml`.

### 3.3 Secrets / prerequisites

Проверить наличие:
- `SELECTEL_REGISTRY_OTUS_USERNAME_PROD`
- `SELECTEL_REGISTRY_OTUS_TOKEN_PROD`
- `VPS_OTUS_HOST`, `VPS_OTUS_USER`, `VPS_OTUS_SSH_KEY`
- Keycloak secret и Telegram token остаются в файле конфига на VPS (не в репозитории).

### 3.4 Проверка пайплайна

- Ручной `workflow_dispatch` с dry-run логикой.
- Smoke-check после деплоя:
  - `aw:health`
  - `aw:logs`
  - тестовое сообщение через Kafka `news_alerts`.

---

## 5) Что еще вы могли упустить (важно)

### 5.1 Kafka topics для алертинга

В `deploy/local/docker-compose.kafka.local.yml` и `deploy/prod/docker-compose.kafka.prod.yml` сейчас нет гарантированного создания:
- `news_alerts`
- `news_alerts.DLT`

Нужно добавить их в `kafka-init`.

### 5.2 Порядок запуска в проде

Для стабильного старта:
1. `main-service`
2. `kafka`
3. `news-processor`
4. `alert-worker`

### 5.3 Тест-план (минимум)

- E2E: правило -> новость -> `news_alerts` -> Telegram.
- Негатив: недоступный Telegram -> retries -> DLT.
- Проверка dedup/cooldown через Reserve/Finalize.

### 5.4 Наблюдаемость на дашбордах

- Добавить панели Grafana (если используется):
  - throughput, fail rate, DLT volume, latency доставки.

### 5.5 Документация эксплуатации

- Обновить `README.md` и `docs/ALERT_WORKER_RUNBOOK.md` после реализации пунктов 1-3.

---

## 6) Рекомендуемый порядок исполнения

1. Kafka topics (`news_alerts`, `news_alerts.DLT`).
2. Promtail regex + проверка логов Loki.
3. Streamlit status/logs для `alert-worker`.
4. Dockerfile + CI/CD job для `alert-worker`.
5. Финальный E2E smoke-test на проде.

## 6.1 Текущий статус исполнения

- Выполнено:
  - шаг 1: Kafka topics (`news_alerts`, `news_alerts.DLT`);
  - шаг 2: Promtail/Loki фильтр для `alert-worker`;
  - шаг 3: Streamlit статус + логи `alert-worker`;
  - шаг 4: CI/CD + `alert-worker.Dockerfile`.
- Осталось:
  - шаг 5: финальный E2E smoke-test на проде.

Чеклист шага 5:
- `task -d . -t deploy/prod/Taskfile.yml config:upload`
- `task -d . -t deploy/prod/Taskfile.yml deploy:compose`
- `task -d . -t deploy/prod/Taskfile.yml kafka:topics`
- `task -d . -t deploy/prod/Taskfile.yml aw:up`
- `task -d . -t deploy/prod/Taskfile.yml aw:health`
- `task -d . -t deploy/prod/Taskfile.yml aw:logs`
- тестовый event в `news_alerts` и проверка доставки в Telegram

---

## Критерии готовности

- В Loki видны логи `otus-alert-worker-prod`.
- В Streamlit есть status карточка и логи `alert-worker`.
- GitHub Actions автоматически деплоит `alert-worker`.
- E2E сценарий доставки Telegram проходит стабильно.

## 7) Опционально (после запуска в прод)

Если позже понадобится усилить наблюдаемость:
- добавить `/metrics` в `alert-worker`;
- подключить scrape-job в Prometheus;
- добавить базовые алерты по error-rate/DLT.
