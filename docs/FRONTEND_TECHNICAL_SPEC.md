# OtusMS: описание проекта и ТЗ на фронтенд (JavaScript)

## 1. Назначение документа

Документ фиксирует:
- описание текущего проекта `OtusMS`;
- подробное техническое задание на разработку фронтенда на JavaScript;
- отдельное описание логики работы приложения;
- организацию сервисов и окружения на VPS;
- описание API, на которое должен опираться фронтенд.

Документ ориентирован на разработчиков frontend/backend, DevOps и QA.

---

## 2. Краткое описание проекта

`OtusMS` - микросервисная платформа агрегации новостей с персонализацией и алертингом.

Основная бизнес-идея:
1. Собирать новости из внешних источников (RSS/Atom).
2. Обрабатывать и обогащать новости.
3. Выдавать пользователю персонализированную ленту.
4. Отслеживать действия пользователя и улучшать релевантность.
5. Отправлять уведомления по правилам алертинга.

Техническая основа:
- backend: Go;
- авторизация: Keycloak + JWT;
- транспорт и интеграции: REST + gRPC + Kafka;
- хранение: PostgreSQL, S3-compatible object storage, Redis;
- уведомления: Telegram Bot API.

Основные runtime-сервисы:
- `main-service` - бизнес-API (users, personalization, alerts);
- `auth-proxy` - auth API (register/login/refresh/logout);
- `news-collector` - сбор новостей и публикация в Kafka;
- `news-processor` - обработка и сохранение новостей;
- `alert-worker` - доставка алертов.

---

## 3. Отдельно: логика работы приложения

## 3.1 Логика пользовательского контура

1. Пользователь регистрируется (`auth-proxy`), затем логинится.
2. Фронтенд получает `access_token` и `refresh_token`.
3. Пользователь настраивает `preferences` (категории, источники, ключевые слова, язык, горизонт времени).
4. Фронтенд запрашивает персонализированную ленту `GET /api/v1/news/feed`.
5. При действиях с карточкой новости фронтенд отправляет события `view/click/like/dislike/hide`.
6. События учитываются в ранжировании и фильтрации следующей выдачи.
7. Пользователь может создавать правила алертинга по ключевым словам.
8. События доставки алертов отображаются в истории (`pending/sent/failed/dropped`).

## 3.2 Логика data pipeline (внутренний контур)

1. `news-collector` берет источники через gRPC у `main-service`.
2. `news-collector` парсит RSS/Atom и пишет сырье в Kafka (`raw_news`).
3. `news-processor` читает `raw_news`, обрабатывает контент (категория, теги, summary), сохраняет артефакты в S3, итог в `main-service` по gRPC.
4. На основании правил алертинга формируются события в Kafka (`news_alerts`).
5. `alert-worker` читает `news_alerts`, отправляет в Telegram, пишет итог статуса доставки через `main-service`.

## 3.3 Логика авторизации и ролей

- `auth-proxy` работает с Keycloak и выдает токены.
- `main-service` валидирует JWT и применяет RBAC.
- Базовые роли:
  - `user` - пользовательские сценарии;
  - `admin` - админ-сценарии;
  - `service-account` - service-to-service сценарии (в первую очередь для внутреннего взаимодействия).

---

## 4. Техническое задание на фронтенд (JavaScript)

## 4.1 Цель

Разработать SPA-клиент на JavaScript для:
- пользовательской работы с персонализированной лентой;
- управления предпочтениями;
- управления правилами алертов;
- админ-функций управления пользователями и просмотра общих новостей.

## 4.2 Формат решения

- Тип: SPA.
- Язык: JavaScript.
- Рекомендуемый стек: `React + Vite` (допускается эквивалентный SPA-стек).
- Интеграция только по HTTP REST (gRPC фронтенду не нужен).

## 4.3 Границы MVP

Обязательно реализовать:
1. Аутентификация: register/login/refresh/logout.
2. Экран персональных предпочтений.
3. Экран персонализированной ленты.
4. Отправка пользовательских событий по новостям.
5. Раздел алертов:
   - список правил;
   - создание/редактирование/удаление;
   - история событий алертов.
6. Админ-раздел:
   - список пользователей;
   - просмотр пользователя по UUID;
   - мягкое удаление пользователя;
   - просмотр общего списка новостей (`admin` only).

## 4.4 Функциональные требования по экранам

### 4.4.1 Экран аутентификации

- Вкладки/режимы: `Login`, `Register`.
- Валидация обязательных полей до отправки.
- Отображение ошибок сервера (400/401/409/500).
- После login: переход на ленту.

### 4.4.2 Экран "Лента"

- Запрос: `GET /api/v1/news/feed`.
- Параметры UI:
  - `q` (FTS строка);
  - `fromHours`;
  - пагинация `limit/offset`.
- Для каждой карточки:
  - отображать `topic`, `source`, `category`, `tags`, `publishedAt`, `url`, `score`;
  - кнопки действий: `view`, `click`, `like`, `dislike`, `hide`.
- При `hide` карточка удаляется из текущего списка без полной перезагрузки страницы.

### 4.4.3 Экран "Preferences"

- Поля:
  - `preferredCategories` (массив строк);
  - `preferredSources` (массив строк);
  - `preferredKeywords` (массив строк);
  - `preferredLanguage` (`ru|en|""`);
  - `fromHours` (число).
- Операции:
  - загрузка текущих значений;
  - сохранение изменений;
  - обработка валидационных ошибок.

### 4.4.4 Экран "Alerts"

- Правила:
  - список правил;
  - создание;
  - редактирование;
  - удаление.
- События:
  - история событий;
  - фильтр по статусу (`pending|sent|failed|dropped`);
  - пагинация (`limit/offset`).

### 4.4.5 Админ-раздел

- Таблица пользователей (`GET /api/v1/users`).
- Поиск/просмотр пользователя по UUID (`GET /api/v1/users/{uuid}`).
- Удаление пользователя (`DELETE /api/v1/users/{uuid}`).
- Список общих новостей (`GET /api/v1/news`).
- Админ-раздел недоступен для роли `user`.

## 4.5 Нефункциональные требования

- Обработка стандартных API-ошибок: `400/401/403/404/409/500`.
- Глобальная обработка `401` с попыткой refresh и повтором исходного запроса.
- Базовая адаптивность интерфейса: desktop + tablet.
- Показ состояний: `loading`, `empty`, `error`.
- Базовая доступность: корректная tab-навигация, aria-label на ключевых действиях.

## 4.6 Требования к безопасности

- Все запросы на production только по HTTPS.
- `Authorization: Bearer <access_token>`.
- Рекомендуемая стратегия хранения:
  - `access_token` в памяти приложения (допустимо `sessionStorage`);
  - `refresh_token` в `httpOnly` cookie (если доступно через gateway/backend).
- Не хранить токены в `localStorage`.

## 4.7 Требования к архитектуре фронтенда

Рекомендуемая структура:
- `src/app` - bootstrap, роутер, провайдеры;
- `src/shared` - ui-kit, утилиты, HTTP-клиент;
- `src/entities` - сущности (`user`, `news`, `alertRule`, `alertEvent`);
- `src/features` - прикладные сценарии (`auth`, `preferences`, `feed`, `alerts`);
- `src/pages` - страницы;
- `src/widgets` - составные блоки интерфейса.

Минимальные кросс-срезовые модули:
- `apiClient` (baseURL, interceptors, refresh-flow);
- `authStore` (состояние сессии и роль);
- `route guards` (защищенные маршруты + role-based доступ);
- `errorMapper` (перевод API ошибок в пользовательские сообщения).

## 4.8 Окружения и конфигурация

Обязательные переменные окружения:
- `VITE_AUTH_PROXY_URL`;
- `VITE_MAIN_SERVICE_URL`.

Дополнительно (если вводится единый reverse proxy):
- `VITE_API_BASE_URL` с префиксами `/api/main-service` и `/api/auth-proxy`.

## 4.9 Тестирование и приемка

Минимальный набор:
1. Smoke:
   - register/login/logout;
   - загрузка feed;
   - сохранение preferences;
   - создание/редактирование/удаление alert rule.
2. Role tests:
   - `user` не видит admin-раздел;
   - `admin` видит и выполняет admin-операции.
3. Auth tests:
   - истекший access token приводит к refresh и успешному retry.
4. Negative tests:
   - корректная реакция UI на 400/401/403/404/409/500.

Критерий приемки MVP:
- все сценарии из раздела 4.3 выполняются стабильно без критических ошибок.

---

## 5. Подробное описание API для фронтенда

Источник контрактов:
- `api/authproxy/swagger.yaml`;
- `api/mainservice/swagger.yaml`;
- фактический роутинг и handlers в `cmd/main-service/api-server.go` и `internal/handlers/*`.

Важно:
- эндпоинты `alerts/*` присутствуют в рабочем API и коде `main-service`;
- при расхождениях между Swagger и кодом приоритет у фактического роутинга/handlers.

## 5.1 Auth-Proxy API (`/api/v1/auth`)

### `POST /api/v1/auth/register`
- Доступ: публичный.
- Назначение: регистрация пользователя в Keycloak и Main Service.
- Request:
  - `email` (string, required);
  - `firstName` (string, required);
  - `lastName` (string, required);
  - `middleName` (string, optional);
  - `password` (string, required, min 8).
- Responses:
  - `201` - создано;
  - `400` - ошибка валидации;
  - `409` - пользователь уже существует;
  - `500` - внутренняя ошибка.

### `POST /api/v1/auth/login`
- Доступ: публичный.
- Назначение: получение пары токенов.
- Request:
  - `username` (string, required);
  - `password` (string, required).
- Response `200`:
  - `access_token`, `refresh_token`, `expires_in`, `refresh_expires_in`, `token_type`, `scope`.
- Errors:
  - `400`, `401`, `500`.

### `POST /api/v1/auth/refresh`
- Доступ: публичный (по refresh token).
- Request:
  - `refresh_token` (string, required).
- Responses:
  - `200` - новая пара токенов;
  - `400`, `401`, `500`.

### `POST /api/v1/auth/logout`
- Доступ: публичный (по refresh token).
- Request:
  - `refresh_token` (string, required).
- Responses:
  - `204` - успех;
  - `400`, `500`.

## 5.2 Main Service API (`/api/v1`) - пользователей и персонализация

Все endpoints ниже требуют `Authorization: Bearer <token>`.

### `GET /api/v1/users/me/preferences`
- Роли: `user|admin`.
- Query:
  - `userUuid` (optional, только для admin override).
- Response `200`:
  - `preferredCategories: string[]`;
  - `preferredSources: string[]`;
  - `preferredKeywords: string[]`;
  - `preferredLanguage: string`;
  - `fromHours: number`;
  - `updatedAt: datetime`.
- Errors: `401`, `403`, `500`.

### `PUT /api/v1/users/me/preferences`
- Роли: `user|admin`.
- Query:
  - `userUuid` (optional, только для admin override).
- Request body:
  - `preferredCategories: string[]`;
  - `preferredSources: string[]`;
  - `preferredKeywords: string[]`;
  - `preferredLanguage: "ru" | "en" | ""`;
  - `fromHours: number`.
- Бизнес-ограничения:
  - `preferredLanguage` только `ru|en|""`;
  - `fromHours` > 0, максимум `720` (по сервису).
- Responses:
  - `204` - успех;
  - `400`, `401`, `403`, `500`.

### `GET /api/v1/news/feed`
- Роли: `user|admin`.
- Query:
  - `userUuid` (optional, только для admin override);
  - `limit` (default 50, max 100);
  - `offset` (default 0);
  - `fromHours` (если не задан, берется из preferences);
  - `q` (FTS query, websearch syntax).
- Response `200`: массив элементов:
  - `id`, `topic`, `source`, `sourceId`, `url`, `category`, `tags[]`,
  - `publishedAt`, `processedAt`, `createdAt`, `score`.
- Errors:
  - `400` (невалидные query-параметры),
  - `401`, `403`, `500`.

### `POST /api/v1/news/events`
- Роли: `user|admin`.
- Query:
  - `userUuid` (optional, только для admin override).
- Request:
  - `newsId` (UUID, required);
  - `eventType` (required, одно из `view|click|like|dislike|hide`);
  - `metadata` (object, optional).
- Responses:
  - `202` - принято;
  - `400`, `401`, `403`, `500`.

## 5.3 Main Service API - alerts

### `GET /api/v1/alerts/rules`
- Роли: `user|admin`.
- Response `200`: массив правил:
  - `id`, `userUuid`, `keyword`, `isActive`, `channelType`, `channelTarget`, `cooldownSeconds`, `createdAt`, `updatedAt`.
- Errors: `401`, `500`.

### `POST /api/v1/alerts/rules`
- Роли: `user|admin`.
- Request:
  - `keyword` (string, required);
  - `channelType` (string, optional, по умолчанию `telegram`);
  - `channelTarget` (string, optional);
  - `cooldownSeconds` (int, optional, default 300, max 86400).
- Бизнес-ограничения:
  - активных правил на пользователя не более `20`;
  - `channelType` сейчас поддерживается только `telegram`.
- Responses:
  - `201` - создано, возвращает `RuleResponse`;
  - `400` - валидация/бизнес-ограничения;
  - `401`, `500`.

### `PUT /api/v1/alerts/rules/{id}`
- Роли: `user|admin`.
- Path:
  - `id` (UUID правила).
- Request:
  - `keyword` (required),
  - `channelType` (must be `telegram`),
  - `channelTarget` (optional),
  - `cooldownSeconds` (<= 86400),
  - `isActive` (boolean).
- Responses:
  - `204` - обновлено;
  - `400` - невалидный payload/UUID;
  - `401`;
  - `404` - правило не найдено;
  - `500`.

### `DELETE /api/v1/alerts/rules/{id}`
- Роли: `user|admin`.
- Path:
  - `id` (UUID).
- Responses:
  - `204` - удалено;
  - `400`, `401`, `404`, `500`.

### `GET /api/v1/alerts/events`
- Роли: `user|admin`.
- Query:
  - `limit` (default 50, max 200);
  - `offset` (default 0);
  - `status` (optional: `pending|sent|failed|dropped`).
- Response `200`: массив событий:
  - `id`, `ruleId`, `newsId`, `userUuid`, `keyword`, `deliveryStatus`, `errorMessage`, `sentAt`, `createdAt`.
- Errors:
  - `400` (в т.ч. невалидный `status`),
  - `401`, `500`.

## 5.4 Main Service API - admin endpoints

### `GET /api/v1/users`
- Роль: `admin`.
- Response `200`: массив пользователей.
- Errors: `401`, `403`, `500`.

### `GET /api/v1/users/{uuid}`
- Роль: `admin`.
- Response `200`: пользователь.
- Errors: `400`, `401`, `403`, `404`, `500`.

### `DELETE /api/v1/users/{uuid}`
- Роль: `admin`.
- Поведение: soft delete.
- Responses: `204`, `400`, `401`, `403`, `404`, `500`.

### `GET /api/v1/news`
- Роль: `admin`.
- Query:
  - `limit` (1..500, default 50).
- Response `200`: массив новостей (`topic`, `source`, `url`, `createdAt`).
- Errors: `400`, `401`, `403`, `500`.

---

## 6. Отдельно: организация проекта на VPS

## 6.1 Общий принцип

Production разворачивается как набор Docker Compose-стеков в директориях под `/root/otus-microservice/prod/*`.

Сети:
- основная сеть контейнеров: `otus_network`.

Управление:
- централизовано через `deploy/prod/Taskfile.yml`.

## 6.2 Директории на VPS (по Taskfile)

- `/root/otus-microservice/prod/be` - `main-service`.
- `/root/otus-microservice/prod/auth-proxy` - `auth-proxy`.
- `/root/otus-microservice/prod/db` - PostgreSQL.
- `/root/otus-microservice/prod/redis` - Redis/Valkey.
- `/root/otus-microservice/prod/kafka` - Kafka + Kafka UI + init.
- `/root/otus-microservice/prod/news-collector` - collector.
- `/root/otus-microservice/prod/news-processor` - processor.
- `/root/otus-microservice/prod/alert-worker` - alert worker.
- `/root/otus-microservice/prod/swagger` - Swagger UI.
- `/root/otus-microservice/prod/monitoring` - Loki/Promtail (+configs monitoring).

## 6.3 Compose-слои и назначение

- `docker-compose.be.prod.yml`:
  - контейнер `otus-microservice-be-prod`,
  - порты `38080` API, `33000` debug, `39090` metrics.
- `docker-compose.auth-proxy.prod.yml`:
  - контейнер `otus-microservice-auth-proxy-prod`,
  - порт `38081`.
- `docker-compose.db.prod.yml`:
  - контейнер `otus-postgres-prod`,
  - порт `5432`, volume `otus_postgres_data_prod`.
- `docker-compose.redis.prod.yml`:
  - контейнер `otus-redis-prod`,
  - порт `127.0.0.1:6379`, volume `otus_redis_data_prod`.
- `docker-compose.kafka.prod.yml`:
  - `otus-kafka-prod`, `otus-kafka-ui-prod`, init-контейнер;
  - брокер доступен в сети как `kafka:9092`;
  - внешние порты только localhost (`39092`, `39080`).
- `docker-compose.news-collector.prod.yml`:
  - контейнер `otus-news-collector-prod`,
  - health порт `127.0.0.1:38082`.
- `docker-compose.news-processor.prod.yml`:
  - контейнер `otus-news-processor-prod`,
  - health порт `127.0.0.1:38083`.
- `docker-compose.alert-worker.prod.yml`:
  - контейнер `otus-alert-worker-prod`,
  - health порт `127.0.0.1:38084`.
- `docker-compose.swagger.prod.yml`:
  - контейнер `swagger-ui-prod`,
  - порт `127.0.0.1:38090`, внешний доступ через nginx path `/swagger`.
- `docker-compose.loki.prod.yml`:
  - `otus-loki-prod`, `otus-promtail-prod`,
  - Loki на `127.0.0.1:3100`.
- `docker-compose.streamlit.prod.yml`:
  - `otus-streamlit-client-prod` на `38085` (существующий admin UI).

## 6.4 Порядок запуска и зависимости

Рекомендуемый порядок:
1. Инфраструктура: `db`, `kafka`, `redis`.
2. API: `main-service`, затем `auth-proxy`.
3. Pipeline: `news-collector`, `news-processor`, `alert-worker`.
4. Вспомогательные: `swagger`, `loki/promtail`, `streamlit`.

Причины:
- `news-collector` зависит от доступности `main-service`;
- `news-processor` и `alert-worker` зависят от `main-service` и Kafka;
- `auth-proxy` зависит от Keycloak и доступности `main-service` для регистрации.

## 6.5 Конфиги и операционные процедуры

Конфиги сервисов хранятся на VPS в `.../configs/`, загружаются через задачи:
- `task -d . -t deploy/prod/Taskfile.yml config:upload`;
- `task -d . -t deploy/prod/Taskfile.yml config:upload:all`.

Операционные команды:
- запуск/остановка/рестарт каждого сервиса (`*:up`, `*:down`, `*:restart`);
- health-check (`health`, `health:auth`, `nc:health`, `np:health`, `aw:health`);
- логи (`logs:*`);
- обслуживание Kafka (`kafka:*`);
- backup PostgreSQL (`backup:db`, `backup:db:download`).

---

## 7. Риски и допущения для frontend-разработки

1. Возможны расхождения между сгенерированным Swagger и фактическим API в коде.
2. Для production лучше использовать единый публичный gateway (nginx), чтобы избежать CORS-сложностей между `main-service` и `auth-proxy`.
3. Некоторые интеграции (Kafka, Redis, Loki) не являются прямыми frontend API и должны скрываться за backend.
4. В текущей архитектуре уведомления доставляются в Telegram; web push/SSE/websocket канал в MVP не предусмотрен.

---

## 8. Артефакты и source of truth

- API контракты:
  - `api/mainservice/swagger.yaml`
  - `api/authproxy/swagger.yaml`
- Актуальный роутинг:
  - `cmd/main-service/api-server.go`
  - `cmd/auth-proxy/api-server.go`
- Alerting handlers:
  - `internal/handlers/alerting/handler.go`
- Personalization handlers:
  - `internal/handlers/personalization/handler.go`
- Прод-операции:
  - `deploy/prod/Taskfile.yml`
  - `deploy/prod/docker-compose.*.prod.yml`
