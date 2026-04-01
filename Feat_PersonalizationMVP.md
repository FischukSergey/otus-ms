# Personalization MVP - детальный план

> **Документ:** План внедрения персонализации выдачи новостей  
> **Проект:** OtusMS  
> **Статус:** Draft (готов к реализации по итерациям)

## 1. Цель MVP

Сделать персонализированную ленту новостей для авторизованного пользователя на основе:
- явных предпочтений (категории/источники);
- простых событий взаимодействия (просмотр, клик, лайк/дизлайк, скрыть).
- поиска по ключевым словам не только в заголовке, но и по телу новости через PostgreSQL Full-Text Search.

MVP не включает ML-ранжирование и не требует отдельного recommendation-сервиса.

## 2. Границы MVP

### Входит в MVP
- Хранение пользовательских предпочтений.
- Хранение пользовательских событий по новостям.
- PostgreSQL Full-Text Search индекс по новостям (`tsvector` + GIN).
- Новый endpoint персонализированной выдачи.
- Новые endpoint для чтения/обновления предпочтений.
- Новый endpoint записи событий.
- Новая вкладка Streamlit для демонстрации персонализированной ленты.

### Не входит в MVP
- Онлайн-обучение модели ранжирования.
- Векторный поиск (`pgvector`) и semantic retrieval.
- Кросс-девайсные push-уведомления.
- Сложные контентные фильтры (blacklist слов, антифрод и т.д.).

## 3. Архитектурный подход

Персонализация реализуется в `main-service` (HTTP API + PostgreSQL), без новых микросервисов.

```text
Streamlit client
    -> GET/PUT /api/v1/users/me/preferences
    -> GET /api/v1/news/feed
    -> POST /api/v1/news/events
main-service
    -> handlers -> services -> store -> PostgreSQL
                                   -> JOIN news + news_search_index (FTS)
```

## 4. Модель данных (PostgreSQL)

### 4.1 Таблица `user_news_preferences`

Назначение: хранение текущих предпочтений пользователя.

```sql
CREATE TABLE IF NOT EXISTS user_news_preferences (
    user_uuid            UUID PRIMARY KEY REFERENCES users(uuid) ON DELETE CASCADE,
    preferred_categories JSONB NOT NULL DEFAULT '[]', -- ["tech","science"]
    preferred_sources    JSONB NOT NULL DEFAULT '[]', -- ["habr","rbc"]
    preferred_keywords   JSONB NOT NULL DEFAULT '[]', -- ["бпла","сумская область"]
    preferred_language   VARCHAR(10),                 -- "ru", "en"
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_news_preferences_updated_at
    ON user_news_preferences(updated_at DESC);
```

### 4.2 Таблица `user_news_events`

Назначение: журнал взаимодействия пользователя с новостями.

```sql
CREATE TABLE IF NOT EXISTS user_news_events (
    id         UUID PRIMARY KEY,
    user_uuid  UUID NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    news_id    UUID NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    event_type VARCHAR(20) NOT NULL, -- view, click, like, dislike, hide
    event_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata   JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_user_news_events_user_time
    ON user_news_events(user_uuid, event_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_news_events_news_time
    ON user_news_events(news_id, event_at DESC);
```

### 4.3 Таблица `news_search_index` (для Full-Text Search)

Назначение: хранение индексируемого текста новости для быстрого поиска по ключевым словам.

```sql
CREATE TABLE IF NOT EXISTS news_search_index (
    news_id        UUID PRIMARY KEY REFERENCES news(id) ON DELETE CASCADE,
    title          TEXT NOT NULL DEFAULT '',
    summary        TEXT NOT NULL DEFAULT '',
    body_text      TEXT NOT NULL DEFAULT '', -- индексируемое тело (например, усеченное до 20-50 KB)
    tags_text      TEXT NOT NULL DEFAULT '', -- нормализованные теги одной строкой
    search_vector  tsvector NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_news_search_index_vector_gin
    ON news_search_index USING GIN (search_vector);
```

`search_vector` формируется с весами:
- `title` -> A
- `summary` -> B
- `body_text`, `tags_text` -> C

### 4.4 Изменения в `news`

Базовая таблица `news` остаётся без изменений, используется для выдачи карточек, сортировки и score.

## 5. API контракт (main-service)

## 5.1 Preferences API

### GET `/api/v1/users/me/preferences`
- Роль: `user` или `admin`
- Ответ `200`:
```json
{
  "preferredCategories": ["tech", "science"],
  "preferredSources": ["habr", "rbc"],
  "preferredKeywords": ["бпла", "сумская область"],
  "preferredLanguage": "ru",
  "updatedAt": "2026-03-14T09:10:11Z"
}
```

### PUT `/api/v1/users/me/preferences`
- Роль: `user` или `admin`
- Тело:
```json
{
  "preferredCategories": ["tech", "science"],
  "preferredSources": ["habr", "rbc"],
  "preferredKeywords": ["бпла", "сумская область"],
  "preferredLanguage": "ru"
}
```
- Поведение:
  - upsert в `user_news_preferences`;
  - валидация категорий по whitelist (`tech|politics|economy|sports|science|other`).

## 5.2 Feed API

### GET `/api/v1/news/feed`
- Роль: `user` или `admin`
- Query:
  - `limit` (default 20, max 100)
  - `offset` (default 0)
  - `from_hours` (default 168, для "свежести")
  - `q` (optional, поисковая строка; фильтр по FTS в `title/summary/body_text/tags_text`)
- Ответ `200`:
```json
[
  {
    "id": "uuid",
    "topic": "....",
    "source": "rbc",
    "url": "https://....",
    "category": "tech",
    "createdAt": "2026-03-14T09:10:11Z",
    "score": 2.35
  }
]
```

### Scoring для MVP (SQL/Service)
`score = freshness_score + category_boost + source_boost + keyword_boost + fts_rank_boost + event_boost`

- `freshness_score`: чем новее, тем выше (`created_at`, `processed_at`);
- `category_boost`: +1.0 если категория в `preferred_categories`;
- `source_boost`: +0.8 если source в `preferred_sources`;
- `keyword_boost`: +0.6 за каждое совпадение с `preferred_keywords` в `title/summary/body_text/tags_text` (c capped суммой, например max +2.0);
  `preferred_keywords` используются как **сигнал ранжирования**, а не как обязательный фильтр;
- `fts_rank_boost`: добавка на основе `ts_rank(search_vector, websearch_to_tsquery(...))` при наличии `q`;
- `event_boost`:
  - like: +1.5
  - dislike: -1.5
  - hide: исключать из выдачи;
  - click/view: небольшой плюс (+0.1..+0.3) как weak signal.

### Логика фильтрации в MVP (фиксируем `И` между измерениями)

Внутри каждого измерения используется `ИЛИ` по значениям массива, но между измерениями — только `И`.

- Категории: новость проходит, если совпала с любой категорией из `preferred_categories` (или массив пуст).
- Источники: новость проходит, если совпала с любым источником из `preferred_sources` (или массив пуст).
- Язык: должен совпадать с `preferred_language` (если задан).
- Поисковый запрос `q`: если передан, дополнительно применяется `search_vector @@ websearch_to_tsquery('russian', q)`.

Итоговый фильтр:

`category_match AND source_match AND language_match AND q_match_if_present`

Где:
- `q_match_if_present` = `TRUE`, если `q` не задан, иначе результат FTS-условия.
- `preferred_keywords` не входят в `WHERE` как обязательное условие, а влияют на `score`.

Режим `ИЛИ` между измерениями в рамках этого MVP **не реализуется**.

## 5.3 Events API

### POST `/api/v1/news/events`
- Роль: `user` или `admin`
- Тело:
```json
{
  "newsId": "uuid",
  "eventType": "click",
  "metadata": {
    "position": 3,
    "source": "streamlit"
  }
}
```
- Валидируем `eventType` по whitelist: `view|click|like|dislike|hide`.

## 6. Изменения в слоях Go-кода

### 6.1 `internal/models`
- `UserNewsPreferences`
- `UserNewsEvent`
- `PersonalizedNewsItem`

### 6.2 `internal/store`
- новый пакет `internal/store/personalization`:
  - `GetPreferences(userUUID)`
  - `UpsertPreferences(userUUID, payload)`
  - `InsertEvent(event)`
  - `GetPersonalizedFeed(userUUID, limit, offset, fromHours)`

- новый пакет `internal/store/newssearch`:
  - `UpsertSearchIndex(newsID, title, summary, bodyText, tagsText)`
  - `SearchNewsIDs(query, limit, offset)` (если нужно выделить отдельный шаг поиска)

### 6.3 `internal/services`
- новый пакет `internal/services/personalization`:
  - валидация payload;
  - расчет scoring (если не полностью в SQL);
  - фильтрация `hide`.
  - построение tsquery для `preferred_keywords` и `q`.

### 6.4 `internal/handlers`
- новый пакет `internal/handlers/personalization`:
  - `GetPreferences`
  - `UpdatePreferences`
  - `GetFeed`
  - `CreateEvent`

### 6.5 Роутинг в `cmd/main-service/api-server.go`
- Добавить в JWT-защищенную группу:
  - `GET /api/v1/users/me/preferences`
  - `PUT /api/v1/users/me/preferences`
  - `GET /api/v1/news/feed`
  - `POST /api/v1/news/events`
- Доступ через `RequireUser` (user + admin).

## 7. Изменения в Streamlit (`client/`)

### 7.1 Новая вкладка `Персонализация`
- Форма предпочтений:
  - мультиселект категорий;
  - мультиселект источников;
  - ключевые слова (строка через запятую / textarea с парсингом в массив);
  - язык.
- Кнопка "Сохранить предпочтения".
- Использовать:
  - `GET /api/v1/users/me/preferences` для preload;
  - `PUT /api/v1/users/me/preferences` для сохранения.

### 7.2 Новая вкладка `Моя лента`
- Кнопка "Загрузить ленту".
- Параметры: `limit`, `from_hours`, `q`.
- Вывод карточек:
  - topic, source, createdAt, url;
  - (для дебага) score.
- События:
  - при клике "Открыть" отправлять `click`;
  - кнопки `👍 / 👎 / 🙈` отправляют `like/dislike/hide`.

### 7.3 API-клиент (`client/api.py`)
- добавить методы:
  - `get_my_preferences(token)`
  - `update_my_preferences(token, payload)`
  - `get_personalized_feed(token, limit, offset, from_hours, q=None)`
  - `create_news_event(token, news_id, event_type, metadata)`

## 8. RBAC и безопасность

- Все personalization endpoint требуют валидный JWT.
- Разрешенные роли: `user`, `admin`.
- `user_uuid` брать из JWT claims, не из query/body.
- Валидация event payload и ограничение размера `metadata`.

## 9. Наблюдаемость

- Метрики:
  - `personalization_feed_requests_total`
  - `personalization_events_total{event_type=...}`
  - `personalization_feed_latency_ms`
- Логи:
  - `user_uuid`, `limit`, `result_count`, `event_type`.
- Исключить логирование чувствительных данных.

## 10. Пошаговый план реализации (итерации)

### Итерация 1: БД + store + service skeleton
1. Миграции: `user_news_preferences`, `user_news_events`, `news_search_index` + GIN.
2. Репозиторий personalization + newssearch.
3. Сервис personalization (базовая валидация + feed-query с FTS фильтрацией).

### Итерация 2: HTTP API + RBAC
1. Хендлеры personalization.
2. Подключение роутов в `api-server`.
3. Swagger аннотации и регенерация docs.

### Итерация 3: Streamlit
1. Вкладка "Персонализация".
2. Вкладка "Моя лента".
3. Отправка событий (`click/like/dislike/hide`).

### Итерация 4: Улучшение ranking
1. Добавить weights scoring.
2. Добавить event-based boosts.
3. Тонкая настройка FTS ranking (`ts_rank`) и ограничений/filters.

## 11. Критерии готовности MVP

- Пользователь может сохранить предпочтения и увидеть ленту, отличающуюся от дефолтной.
- События записываются и влияют на выдачу (минимум `hide`, `like`, `dislike`).
- Endpoint покрыты unit/integration тестами.
- Streamlit демонстрирует end-to-end сценарий.

## 12. Риски и меры

- Риск: медленные SQL-запросы feed.  
  Мера: GIN индекс по `search_vector` + лимит `from_hours` + пагинация.

- Риск: шумные события (`view` слишком часто).  
  Мера: дедуп по `(user_uuid, news_id, event_type, time_bucket)` или throttle.

- Риск: слабое качество personalization без ML.  
  Мера: начать с rule-based scoring и измерять CTR/engagement.

- Риск: шум в `body_text` снижает качество FTS (служебный/JS-текст).  
  Мера: улучшить очистку контента до индексации, отсекать script/style и ограничивать размер индексируемого тела.
