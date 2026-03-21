# Personalization MVP - документация ветки `feat/Personalization`

> **Ветка:** `feat/Personalization`  
> **Статус:** Implemented  
> **Назначение:** зафиксировать реализованный функционал personalization MVP (не план, а фактическая реализация).

## 1. Что реализовано

В рамках ветки добавлен end-to-end backend поток персонализации:

- хранение пользовательских предпочтений;
- сбор пользовательских событий (view/click/like/dislike/hide);
- персонализированная выдача новостей с `score`;
- FTS-поиск (`websearch_to_tsquery`) в feed;
- синхронизация поискового индекса `news_search_index` при сохранении новостей;
- HTTP API под JWT + RBAC (`user|admin`);
- Swagger-документация и unit-тесты сервисного слоя.

## 2. API endpoint-ы

Все endpoint-ы доступны в `main-service` под `/api/v1`, требуют валидный JWT.

### 2.1 Preferences

- `GET /api/v1/users/me/preferences`  
  Возвращает предпочтения текущего пользователя.  
  Если записи нет в БД, возвращаются дефолты:
  - `fromHours = 168`
  - пустые массивы `preferred*`.

- `PUT /api/v1/users/me/preferences`  
  Создает/обновляет предпочтения пользователя (upsert).

### 2.2 Feed

- `GET /api/v1/news/feed?limit=&offset=&fromHours=&q=`  
  Возвращает персонализированную ленту с сортировкой по `score`.

Query-параметры:
- `limit` (default `50`, max `100`);
- `offset` (min `0`);
- `fromHours` (по умолчанию из preferences, fallback `168`, max `720`);
- `q` (опциональный FTS-запрос).

### 2.3 Events

- `POST /api/v1/news/events`  
  Сохраняет событие пользователя по новости.

Поддерживаемые `eventType`:
- `view`
- `click`
- `like`
- `dislike`
- `hide`

Коды ответа:
- `202 Accepted` при успехе;
- `400` при невалидном payload;
- `401/403` по auth/rbac.

## 3. RBAC

Роуты персонализации подключены под `RequireUser`:
- доступны ролям `user` и `admin`.

Админский endpoint `GET /api/v1/news` оставлен отдельно под `RequireAdmin`.

## 4. Миграция БД

Добавлена миграция:
- `internal/store/migrations/005_create_personalization_schema.sql`

Созданные таблицы:
- `user_news_preferences`
  - PK/FK: `user_uuid -> users(uuid)`
  - массивы `preferred_categories/sources/keywords`
  - `preferred_language`
  - `from_hours` с `CHECK (from_hours > 0 AND from_hours <= 720)`
- `user_news_events`
  - `id` UUID PK
  - FK на `users` и `news`
  - `event_type` с CHECK whitelist
  - `metadata` JSONB
  - индексы по `(user_uuid, created_at)`, `news_id`, `event_type`
- `news_search_index`
  - `news_id` PK/FK на `news`
  - `body_text`, `tags_text`, `search_vector`
  - GIN индекс по `search_vector`

## 5. Логика scoring/feed

Реализована в `internal/store/personalization/repository.go`.

Текущая формула `score`:
- freshness (по `processed_at`, с убыванием по времени);
- `category_boost` (+1.0);
- `source_boost` (+0.8);
- `keyword_boost` через `ts_rank` по `preferred_keywords` (c cap);
- `fts_rank_boost` при наличии `q`;
- `event_boost`:
  - like: `+1.5`
  - dislike: `-1.5`
  - click: `+0.3`
  - view: `+0.1`

Особенности:
- `hide` исключает новость из выдачи;
- фильтрация идет по `AND` между измерениями (категория, источник, язык, q);
- сортировка: `score DESC`, затем `processed_at DESC`, `created_at DESC`.

## 6. Индексация FTS

Синхронизация индекса добавлена в `internal/store/news/repository.go`:

- после успешной вставки новости (`ON CONFLICT (url) DO NOTHING`) выполняется upsert в `news_search_index`;
- `search_vector` собирается из:
  - `title` (вес A),
  - `summary` (вес B),
  - `tags_text` (вес C).

Текущий MVP не наполняет `body_text` (пишется `NULL`).

## 7. Валидация и нормализация

Реализовано в `internal/services/personalization/service.go`:

- `preferredLanguage`: только `ru|en` (или пусто);
- `fromHours`: default `168`, max `720`;
- массивы предпочтений: `trim + lowercase + dedup`;
- `newsId` в событиях обязателен и валидируется как UUID.

## 8. Тесты

Добавлены unit-тесты:
- `internal/services/personalization/service_test.go`

Покрывают:
- fallback на дефолтные preferences;
- валидацию language/event/newsId;
- нормализацию и dedup массивов;
- корректную прокладку фильтров в feed запрос.

## 9. Измененные ключевые файлы

- `cmd/main-service/api-server.go`
- `internal/handlers/personalization/handler.go`
- `internal/services/personalization/dto.go`
- `internal/services/personalization/service.go`
- `internal/services/personalization/service_test.go`
- `internal/store/personalization/repository.go`
- `internal/store/migrations/005_create_personalization_schema.sql`
- `internal/store/news/repository.go`
- `internal/models/user_news_preferences.go`
- `internal/models/user_news_event.go`
- `internal/models/personalized_news_item.go`
- `api/mainservice/*` (swagger regen)
- `api/authproxy/*` (swagger regen task)

## 10. Как проверить локально

1. Применить миграции при запуске `main-service`.
2. Получить JWT пользователя с ролью `user` (или `admin`).
3. Вызвать:
   - `PUT /api/v1/users/me/preferences`
   - `GET /api/v1/news/feed`
   - `POST /api/v1/news/events`
   - повторно `GET /api/v1/news/feed` и проверить изменение ранжирования/исключение `hide`.
4. Открыть Swagger:
   - `/swagger/index.html`

## 11. Ограничения MVP

- Нет ML-модели и персонального обучения; ранжирование rule-based.
- `body_text` в FTS-индексе пока не заполняется (поиск идет по title/summary/tags).
- Веса `score` статические, без онлайн-тюнинга.
- Нет отдельной аналитики качества рекомендаций (CTR/precision метрик).

---

Если следующая итерация: логичный шаг — заполнение `body_text` из processor-пайплайна и настройка веса сигналов на основе продуктовых метрик.
