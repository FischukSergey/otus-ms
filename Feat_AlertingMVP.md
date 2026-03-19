# Alerting MVP (Kafka + alerter worker) - план на 1-2 спринта

> **Документ:** Минимальный MVP алертинга по ключевым словам  
> **Проект:** OtusMS  
> **Статус:** Draft  
> **Подход:** Kafka + отдельный worker для доставки уведомлений

## 1. Цель

Реализовать near-real-time алертинг по ключевым словам в новостях:
- пользователь создаёт правила (keywords);
- при появлении новой новости и совпадении ключевого слова формируется alert-событие;
- отдельный worker отправляет уведомление в канал доставки.

## 2. Границы MVP

### Входит
- CRUD правил алертинга (`main-service` API).
- Проверка совпадений в `news-processor` по мере обработки новостей.
- Публикация alert-событий в Kafka topic.
- Отдельный `alert-worker` consumer для доставки.
- История событий и статус доставки в БД.

### Не входит
- UI настройки каналов (email/telegram) с OAuth.
- Сложные query-правила (regex, boolean-expression, proximity).
- SLA/retention для high-scale enterprise.

## 3. Архитектура

```text
User/Streamlit
   -> main-service (alert rules API)
   -> PostgreSQL (rules)

news-processor
   -> получает news после normal pipeline
   -> матчинг rules (по keywords)
   -> Kafka topic: news_alerts

alert-worker (новый сервис)
   -> consumer news_alerts
   -> отправка уведомления (MVP: Telegram webhook или internal webhook)
   -> запись статуса в PostgreSQL (alert_events)
```

## 4. Kafka topics (схема)

### 4.1 Основной topic
- `news_alerts`
- partitions: `3` (достаточно для MVP)
- replication-factor: по окружению
- retention: `7d`

### 4.2 DLT topic
- `news_alerts.DLT`
- partitions: `1`
- retention: `30d`
- назначение: события, не доставленные после N retries

### 4.3 Формат сообщения `news_alerts` (JSON)

```json
{
  "eventId": "uuid",
  "ruleId": "uuid",
  "userUuid": "uuid",
  "newsId": "uuid",
  "keyword": "сумская область",
  "matchedField": "title",
  "matchedSnippet": "Разведрота ... в Сумской области ...",
  "newsTitle": "Разведрота ВСУ потеряла боеспособность...",
  "newsUrl": "https://ria.ru/....",
  "createdAt": "2026-03-18T10:11:12Z"
}
```

## 5. Модель данных (PostgreSQL)

### 5.1 Таблица `alert_rules`

```sql
CREATE TABLE IF NOT EXISTS alert_rules (
    id                 UUID PRIMARY KEY,
    user_uuid          UUID NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    keyword            VARCHAR(200) NOT NULL,
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    channel_type       VARCHAR(20) NOT NULL DEFAULT 'webhook', -- webhook, telegram
    channel_target     TEXT NOT NULL, -- URL/webhook/chat identifier
    cooldown_seconds   INTEGER NOT NULL DEFAULT 300,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_user_active
    ON alert_rules(user_uuid, is_active);

CREATE INDEX IF NOT EXISTS idx_alert_rules_keyword
    ON alert_rules(lower(keyword));
```

### 5.2 Таблица `alert_events`

```sql
CREATE TABLE IF NOT EXISTS alert_events (
    id               UUID PRIMARY KEY,
    rule_id          UUID NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    news_id          UUID NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    user_uuid        UUID NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    keyword          VARCHAR(200) NOT NULL,
    delivery_status  VARCHAR(20) NOT NULL, -- pending, sent, failed, dropped
    error_message    TEXT,
    sent_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (rule_id, news_id) -- дедуп алертов на одно правило/новость
);

CREATE INDEX IF NOT EXISTS idx_alert_events_user_created
    ON alert_events(user_uuid, created_at DESC);
```

## 6. API (main-service)

Роль доступа для MVP: `RequireUser` (`user` и `admin`).

## 6.1 Alert Rules

### GET `/api/v1/alerts/rules`
- список правил текущего пользователя.

### POST `/api/v1/alerts/rules`
Создание правила.

Тело:
```json
{
  "keyword": "сумская область",
  "channelType": "webhook",
  "channelTarget": "https://example.com/alerts",
  "cooldownSeconds": 300
}
```

### PUT `/api/v1/alerts/rules/{id}`
Обновление правила (keyword/channel/cooldown/isActive).

### DELETE `/api/v1/alerts/rules/{id}`
Мягкое удаление или hard delete (для MVP можно hard delete).

## 6.2 Alert Events

### GET `/api/v1/alerts/events`
Query:
- `limit` (default 50, max 200)
- `offset` (default 0)
- `status` (optional)

Возвращает историю доставок для текущего пользователя.

## 7. Изменения в сервисах

## 7.1 `news-processor`
- Добавить зависимость на read-only client правил (`GetActiveAlertRules` через `main-service`).
- Кешировать активные правила (например, refresh каждые 30-60 секунд).
- После обработки новости:
  - нормализовать текст (`title + summary + body_text_fragment`);
  - матчить keywords;
  - для каждого match публиковать событие в `news_alerts`.

> Для MVP достаточно simple match: `strings.Contains(lowerText, lowerKeyword)`.

## 7.2 Новый сервис `alert-worker`
- Kafka consumer `news_alerts`.
- Доставка уведомления:
  - MVP: `webhook POST` (универсально),
  - опционально `telegram bot API`.
- Запись результата в `alert_events`.
- Retry политика:
  - 3 попытки (экспоненциальный backoff),
  - затем в `news_alerts.DLT`.

## 8. Антиспам и надежность

- Дедуп на уровне БД: `UNIQUE(rule_id, news_id)`.
- Cooldown:
  - перед отправкой проверять последнее `sent_at` по `rule_id`;
  - если меньше `cooldown_seconds`, ставить `delivery_status='dropped'`.
- Ограничения:
  - не более 20 активных правил на пользователя (MVP).

## 9. Наблюдаемость

- Метрики:
  - `alerts_matches_total`
  - `alerts_published_total`
  - `alerts_sent_total`
  - `alerts_failed_total`
  - `alerts_dropped_cooldown_total`
- Логи:
  - `rule_id`, `user_uuid`, `news_id`, `keyword`, `delivery_status`.

## 10. План внедрения на 1-2 спринта

## Спринт 1 (MVP core)
1. Миграции: `alert_rules`, `alert_events`.
2. API в `main-service`: CRUD `alert_rules`, список `alert_events`.
3. Kafka topics: `news_alerts`, `news_alerts.DLT`.
4. Базовый matcher в `news-processor` + publish в Kafka.
5. `alert-worker` с webhook-доставкой.

**Definition of Done (Sprint 1):**
- пользователь создал правило;
- новая новость с keyword создала событие;
- событие доставлено в webhook;
- статус виден через `/api/v1/alerts/events`.

## Спринт 2 (hardening)
1. Cooldown и расширенный дедуп.
2. Retry + DLT + ручной reprocess команды.
3. Метрики/дашборд алертинга.
4. Streamlit вкладка "Алерты" (правила + история событий).

**Definition of Done (Sprint 2):**
- алертинг стабилен под burst-нагрузкой;
- нет дубликатов при повторной обработке;
- есть наблюдаемость и базовая диагностика.

## 11. Минимальные изменения в Streamlit (после Sprint 1)

- Новая вкладка `Алерты`:
  - список правил;
  - создание/редактирование/удаление;
  - история последних событий (status/sent_at/error).

MVP-UI можно сделать только для webhook-канала.

## 12. Риски и меры

- Риск: большое число правил -> рост времени матчинга.  
  Мера: кеш правил + ограничение числа правил + нормализация текста.

- Риск: webhook endpoint недоступен.  
  Мера: retry + DLT + история ошибок.

- Риск: false-positive matches по `contains`.  
  Мера: на Sprint 2 добавить word-boundary/FTS matching.
