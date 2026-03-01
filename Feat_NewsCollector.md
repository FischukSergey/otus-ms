# News Collector Service - Концептуальное решение

> **Документ:** Техническое решение для реализации News Collector Service  
> **Проект:** Агрегатор новостей с персонализацией  
> **Дата:** 15 февраля 2026

## 📋 Содержание

1. [Обзор](#обзор)
2. [Архитектура сервиса](#архитектура-сервиса)
3. [Модель данных](#модель-данных)
4. [Технологический стек](#технологический-стек)
5. [Ключевые компоненты](#ключевые-компоненты)
6. [API эндпоинты](#api-эндпоинты)
7. [Конфигурация](#конфигурация)
8. [Обработка ошибок](#обработка-ошибок)
9. [Метрики и мониторинг](#метрики-и-мониторинг)
10. [Деплой и инфраструктура](#деплой-и-инфраструктура)
11. [План реализации](#план-реализации)
12. [Рекомендации](#рекомендации)

---

## Обзор

### Назначение

News Collector Service — это **первое звено** в pipeline обработки новостей агрегатора. Сервис отвечает за:

- Периодический опрос RSS/Atom источников (до 15 штук)
- Парсинг фидов новостей
- Отправку сырых новостей в Kafka топик `raw_news`
- Обработку ошибок и retry логику
- Управление статусом источников

### Позиция в архитектуре

```
┌─────────────────────────────────────────────────────────────┐
│                    Общая архитектура                         │
└─────────────────────────────────────────────────────────────┘

RSS/Atom источники (15 шт)
           │
           ▼
┌──────────────────────┐
│  News Collector      │  ◄── Этот сервис
│  - RSS парсинг       │
│  - Scheduling        │
│  - Error handling    │
└──────────┬───────────┘
           │
           ▼
    Kafka: raw_news
           │
           ▼
┌──────────────────────┐
│  News Processor      │
│  - Классификация     │
│  - Дедупликация      │
│  - Очистка           │
└──────────┬───────────┘
           │
           ▼
      PostgreSQL
```

### Ключевые требования

- ✅ Поддержка до 15 RSS/Atom источников
- ✅ Периодический опрос с настраиваемым интервалом (по умолчанию 1 час)
- ✅ Отказоустойчивость: retry логика, обработка ошибок
- ✅ Параллельная обработка источников
- ✅ Административный API для управления источниками
- ✅ Метрики Prometheus
- ✅ Graceful shutdown

---

## Архитектура сервиса

### Структура проекта

Следуем паттерну существующих микросервисов (`main-service`, `auth-proxy`):

```
OtusMS/
├── cmd/
│   ├── main-service/          # Существующий
│   ├── auth-proxy/            # Существующий
│   └── news-collector/        # 🆕 Новый сервис
│       ├── main.go            # Инициализация и запуск
│       ├── api-server.go      # HTTP сервер
│       ├── debug-server.go    # Debug endpoints
│       └── metrics-server.go  # Prometheus metrics
│
├── internal/
│   ├── config/               # Общая конфигурация (расширить)
│   │
│   ├── handlers/
│   │   └── collector/        # 🆕 HTTP handlers
│   │       ├── handler.go    # CRUD операции с источниками
│   │       └── status.go     # Статус коллектора
│   │
│   ├── services/
│   │   └── collector/        # 🆕 Бизнес-логика
│   │       ├── service.go    # Главная логика коллектора
│   │       ├── scheduler.go  # Планировщик задач
│   │       ├── parser.go     # RSS/Atom парсер
│   │       └── producer.go   # Kafka producer
│   │
│   ├── store/
│   │   └── sources/          # 🆕 Репозиторий источников
│   │       └── repository.go # CRUD для news_sources
│   │
│   └── models/
│       ├── source.go         # 🆕 Модель источника
│       └── raw_news.go       # 🆕 Модель сырой новости
│
├── configs/
│   ├── config.news-collector.local.yaml
│   ├── config.news-collector.prod.yaml
│   └── config.news-collector.example.yaml
│
├── deploy/
│   ├── local/
│   │   └── docker-compose.local.yml  # Добавить news-collector
│   └── prod/
│       └── docker-compose.news-collector.prod.yml
│
├── news-collector.Dockerfile  # 🆕 Dockerfile для сервиса
└── Feat_NewsCollector.md      # 🆕 Этот документ
```

### Слои архитектуры

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Handlers                        │
│  - CRUD для источников                                  │
│  - Административные endpoints                           │
│  - Health checks                                        │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                 Collector Service                       │
│  - Главная бизнес-логика                               │
│  - Координация компонентов                             │
│  - Управление жизненным циклом                         │
└─┬──────────────┬──────────────┬────────────────────────┘
  │              │              │
  ▼              ▼              ▼
┌────────┐  ┌─────────┐  ┌──────────┐
│Scheduler│  │ Parser  │  │ Producer │
│(Cron)   │  │ (RSS)   │  │ (Kafka)  │
└────────┘  └─────────┘  └──────────┘
     │           │              │
     └───────────┴──────────────┘
                 │
                 ▼
         ┌──────────────┐
         │  Repository  │
         │ (PostgreSQL) │
         └──────────────┘
```

---

## Модель данных

### Таблица `news_sources` (PostgreSQL)

```sql
-- Миграция: internal/store/migrations/003_create_news_sources.sql

CREATE TABLE news_sources (
    id VARCHAR(100) PRIMARY KEY,           -- 'source_1', 'source_2'...
    name VARCHAR(200) NOT NULL,            -- 'Lenta.ru Tech', 'RBC News'
    url VARCHAR(1000) NOT NULL,            -- RSS feed URL
    language VARCHAR(10),                  -- 'ru', 'en'
    category VARCHAR(50),                  -- 'tech', 'politics', 'sport'
    fetch_interval INTEGER DEFAULT 3600,   -- Интервал опроса (секунды)
    is_active BOOLEAN DEFAULT true,        -- Активен ли источник
    last_collected_at TIMESTAMPTZ,         -- Последний успешный сбор
    last_error TEXT,                       -- Текст последней ошибки
    error_count INTEGER DEFAULT 0,         -- Счётчик последовательных ошибок
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индексы для оптимизации
CREATE INDEX idx_sources_active ON news_sources(is_active);
CREATE INDEX idx_sources_next_fetch ON news_sources(last_collected_at, fetch_interval);

-- Комментарии к полям
COMMENT ON COLUMN news_sources.fetch_interval IS 'Интервал опроса в секундах (3600 = 1 час)';
COMMENT ON COLUMN news_sources.error_count IS 'Сбрасывается при успешном сборе, после 5 ошибок источник деактивируется';
```

### Seed данные (начальные источники)

```sql
-- Миграция: internal/store/migrations/004_seed_news_sources.sql

INSERT INTO news_sources (id, name, url, language, category, fetch_interval) VALUES
('source_1', 'Lenta.ru Технологии', 'https://lenta.ru/rss/tech', 'ru', 'tech', 3600),
('source_2', 'RBC Новости', 'https://rssexport.rbc.ru/rbcnews/news/20/full.rss', 'ru', 'news', 1800),
('source_3', 'Habr Все потоки', 'https://habr.com/ru/rss/all/', 'ru', 'tech', 1800),
('source_4', 'Kommersant Главное', 'https://www.kommersant.ru/RSS/main.xml', 'ru', 'news', 3600),
('source_5', 'Meduza Новости', 'https://meduza.io/rss/all', 'ru', 'news', 1800),
('source_6', 'The Verge', 'https://www.theverge.com/rss/index.xml', 'en', 'tech', 3600),
('source_7', 'TechCrunch', 'https://techcrunch.com/feed/', 'en', 'tech', 3600),
('source_8', 'Wired', 'https://www.wired.com/feed/rss', 'en', 'tech', 7200),
('source_9', 'Ars Technica', 'https://feeds.arstechnica.com/arstechnica/index', 'en', 'tech', 7200),
('source_10', 'BBC News Tech', 'http://feeds.bbci.co.uk/news/technology/rss.xml', 'en', 'tech', 3600),
('source_11', 'Reuters Tech', 'https://www.reutersagency.com/feed/?taxonomy=best-topics&post_type=best', 'en', 'news', 3600),
('source_12', 'Engadget', 'https://www.engadget.com/rss.xml', 'en', 'tech', 3600),
('source_13', 'Gazeta.ru', 'https://www.gazeta.ru/export/rss/first.xml', 'ru', 'news', 1800),
('source_14', 'Forbes Технологии', 'https://www.forbes.ru/rss-tag-category.xml?tag=2', 'ru', 'tech', 7200),
('source_15', 'VC.ru', 'https://vc.ru/rss/all', 'ru', 'tech', 1800);
```

### Go модели

#### Source (internal/models/source.go)

```go
package models

import (
    "database/sql"
    "time"
)

// Source представляет источник новостей (RSS/Atom feed).
type Source struct {
    ID              string         `json:"id" db:"id"`
    Name            string         `json:"name" db:"name"`
    URL             string         `json:"url" db:"url"`
    Language        string         `json:"language" db:"language"`
    Category        string         `json:"category" db:"category"`
    FetchInterval   int            `json:"fetch_interval" db:"fetch_interval"`
    IsActive        bool           `json:"is_active" db:"is_active"`
    LastCollectedAt sql.NullTime   `json:"last_collected_at" db:"last_collected_at"`
    LastError       sql.NullString `json:"last_error" db:"last_error"`
    ErrorCount      int            `json:"error_count" db:"error_count"`
    CreatedAt       time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
}

// NextFetchAt вычисляет время следующего запланированного сбора.
func (s *Source) NextFetchAt() time.Time {
    if !s.LastCollectedAt.Valid {
        return time.Now() // Если никогда не собирали - можно сразу
    }
    return s.LastCollectedAt.Time.Add(time.Duration(s.FetchInterval) * time.Second)
}

// IsDue проверяет, пора ли собирать новости из этого источника.
func (s *Source) IsDue() bool {
    return s.IsActive && time.Now().After(s.NextFetchAt())
}
```

#### RawNews (internal/models/raw_news.go)

```go
package models

import "time"

// RawNews представляет сырую новость из RSS/Atom фида.
// Это сообщение отправляется в Kafka топик raw_news.
type RawNews struct {
    ID          string    `json:"id"`           // UUID
    SourceID    string    `json:"source_id"`    // ID источника
    Title       string    `json:"title"`        // Заголовок
    Description string    `json:"description"`  // Краткое описание
    Content     string    `json:"content"`      // Полный текст (если есть)
    URL         string    `json:"url"`          // Ссылка на оригинал
    PublishedAt time.Time `json:"published_at"` // Дата публикации
    CollectedAt time.Time `json:"collected_at"` // Дата сбора
    Author      string    `json:"author"`       // Автор (опционально)
    ImageURL    string    `json:"image_url"`    // URL изображения (опционально)
    RawData     string    `json:"raw_data"`     // Сырые данные для отладки
}
```

### Структура Kafka сообщения

Пример сообщения в топике `raw_news`:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "source_id": "source_1",
  "title": "Новая версия Go 1.24 выпущена",
  "description": "Команда Go анонсировала релиз Go 1.24 с улучшениями производительности",
  "content": "Полный текст статьи...",
  "url": "https://lenta.ru/news/2026/02/15/go124/",
  "published_at": "2026-02-15T10:00:00Z",
  "collected_at": "2026-02-15T10:05:23Z",
  "author": "Иван Иванов",
  "image_url": "https://lenta.ru/images/go-logo.jpg",
  "raw_data": "..."
}
```

---

## Технологический стек

### Основные библиотеки Go

```go
// RSS/Atom парсинг
import "github.com/mmcdole/gofeed"  // v1.2+

// Kafka producer
import "github.com/IBM/sarama"      // v1.43+ (или github.com/segmentio/kafka-go)

// Планировщик
import "github.com/robfig/cron/v3"  // v3.0+

// UUID генерация
import "github.com/google/uuid"     // v1.6+

// HTTP клиент (стандартный)
import "net/http"

// Уже используются в проекте:
import "github.com/go-chi/chi/v5"   // HTTP роутер
import "log/slog"                   // Логирование
import "github.com/prometheus/client_golang/prometheus"  // Метрики
```

### Инфраструктура

- **PostgreSQL**: Хранение источников (таблица `news_sources`)
- **Kafka**: Message broker для передачи новостей в обработчик
- **Docker**: Контейнеризация
- **Prometheus**: Метрики
- **Nginx**: API Gateway (уже настроен)

---

## Ключевые компоненты

### 1. Scheduler (Планировщик)

**Файл:** `internal/services/collector/scheduler.go`

**Назначение:** Периодически проверяет источники и запускает сбор для тех, которые готовы.

```go
package collector

import (
    "context"
    "log/slog"
    "time"

    "github.com/robfig/cron/v3"
)

type Scheduler struct {
    cron    *cron.Cron
    service *Service
    logger  *slog.Logger
}

func NewScheduler(service *Service, logger *slog.Logger) *Scheduler {
    return &Scheduler{
        cron:    cron.New(cron.WithSeconds()),
        service: service,
        logger:  logger,
    }
}

// Start запускает планировщик.
func (s *Scheduler) Start(ctx context.Context) error {
    // Проверяем источники каждую минуту
    _, err := s.cron.AddFunc("0 * * * * *", func() {
        if err := s.service.CollectFromDueSources(ctx); err != nil {
            s.logger.Error("failed to collect from due sources", "error", err)
        }
    })
    if err != nil {
        return err
    }

    s.logger.Info("scheduler started")
    s.cron.Start()
    return nil
}

// Stop останавливает планировщик.
func (s *Scheduler) Stop() {
    s.logger.Info("stopping scheduler")
    s.cron.Stop()
}
```

**Логика работы:**
1. Каждую минуту вызывается `CollectFromDueSources()`
2. Сервис проверяет, какие источники пора обновить
3. Для каждого подходящего источника запускается сбор

**Альтернатива:** Можно использовать разные интервалы для разных категорий источников.

### 2. RSS Parser

**Файл:** `internal/services/collector/parser.go`

**Назначение:** Парсинг RSS/Atom фидов и преобразование в `RawNews`.

```go
package collector

import (
    "fmt"
    "log/slog"
    "math"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/mmcdole/gofeed"

    "github.com/FischukSergey/otus-ms/internal/models"
)

type Parser struct {
    httpClient *http.Client
    logger     *slog.Logger
}

func NewParser(timeout time.Duration, logger *slog.Logger) *Parser {
    return &Parser{
        httpClient: &http.Client{
            Timeout: timeout,
        },
        logger: logger,
    }
}

// ParseFeed парсит RSS/Atom фид по URL.
func (p *Parser) ParseFeed(url string) ([]*models.RawNews, error) {
    fp := gofeed.NewParser()
    fp.Client = p.httpClient

    feed, err := fp.ParseURL(url)
    if err != nil {
        return nil, fmt.Errorf("failed to parse feed: %w", err)
    }

    var news []*models.RawNews
    for _, item := range feed.Items {
        rawNews := &models.RawNews{
            ID:          uuid.New().String(),
            Title:       item.Title,
            Description: item.Description,
            Content:     extractContent(item),
            URL:         item.Link,
            PublishedAt: parsePublishedTime(item),
            CollectedAt: time.Now(),
            Author:      extractAuthor(item),
            ImageURL:    extractImageURL(item),
        }
        news = append(news, rawNews)
    }

    return news, nil
}

// ParseFeedWithRetry парсит фид с повторными попытками.
func (p *Parser) ParseFeedWithRetry(url string, maxRetries int) ([]*models.RawNews, error) {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        news, err := p.ParseFeed(url)
        if err == nil {
            return news, nil
        }

        lastErr = err
        backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        
        p.logger.Warn("retry parsing feed",
            "url", url,
            "attempt", attempt+1,
            "backoff", backoff,
            "error", err,
        )

        time.Sleep(backoff)
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Вспомогательные функции

func extractContent(item *gofeed.Item) string {
    if item.Content != "" {
        return item.Content
    }
    return item.Description
}

func parsePublishedTime(item *gofeed.Item) time.Time {
    if item.PublishedParsed != nil {
        return *item.PublishedParsed
    }
    return time.Now()
}

func extractAuthor(item *gofeed.Item) string {
    if item.Author != nil {
        return item.Author.Name
    }
    return ""
}

func extractImageURL(item *gofeed.Item) string {
    if item.Image != nil {
        return item.Image.URL
    }
    // Можно добавить поиск в enclosures
    for _, enc := range item.Enclosures {
        if enc.Type == "image/jpeg" || enc.Type == "image/png" {
            return enc.URL
        }
    }
    return ""
}
```

### 3. Kafka Producer

**Файл:** `internal/services/collector/producer.go`

**Назначение:** Отправка сырых новостей в Kafka топик `raw_news`.

```go
package collector

import (
    "encoding/json"
    "fmt"
    "log/slog"

    "github.com/IBM/sarama"

    "github.com/FischukSergey/otus-ms/internal/models"
)

type KafkaProducer struct {
    producer sarama.SyncProducer
    topic    string
    logger   *slog.Logger
}

func NewKafkaProducer(brokers []string, topic string, logger *slog.Logger) (*KafkaProducer, error) {
    config := sarama.NewConfig()
    config.Producer.RequiredAcks = sarama.WaitForAll // Ждём подтверждения от всех реплик
    config.Producer.Retry.Max = 3
    config.Producer.Return.Successes = true

    producer, err := sarama.NewSyncProducer(brokers, config)
    if err != nil {
        return nil, fmt.Errorf("failed to create kafka producer: %w", err)
    }

    return &KafkaProducer{
        producer: producer,
        topic:    topic,
        logger:   logger,
    }, nil
}

// SendRawNews отправляет одну новость в Kafka.
func (p *KafkaProducer) SendRawNews(news *models.RawNews) error {
    data, err := json.Marshal(news)
    if err != nil {
        return fmt.Errorf("marshal error: %w", err)
    }

    msg := &sarama.ProducerMessage{
        Topic: p.topic,
        Key:   sarama.StringEncoder(news.SourceID), // Партиционирование по source_id
        Value: sarama.ByteEncoder(data),
    }

    partition, offset, err := p.producer.SendMessage(msg)
    if err != nil {
        return fmt.Errorf("send to kafka error: %w", err)
    }

    p.logger.Debug("message sent to kafka",
        "topic", p.topic,
        "partition", partition,
        "offset", offset,
        "news_id", news.ID,
    )

    return nil
}

// SendBatch отправляет пакет новостей.
func (p *KafkaProducer) SendBatch(newsList []*models.RawNews) error {
    for _, news := range newsList {
        if err := p.SendRawNews(news); err != nil {
            return err
        }
    }
    return nil
}

// Close закрывает producer.
func (p *KafkaProducer) Close() error {
    return p.producer.Close()
}
```

### 4. Collector Service (главная логика)

**Файл:** `internal/services/collector/service.go`

**Назначение:** Координация всех компонентов, управление процессом сбора.

```go
package collector

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/FischukSergey/otus-ms/internal/models"
)

// Repository определяет интерфейс для работы с источниками.
type Repository interface {
    GetAllActive(ctx context.Context) ([]*models.Source, error)
    GetDueSources(ctx context.Context) ([]*models.Source, error)
    UpdateLastCollected(ctx context.Context, id string, collectedAt time.Time, err error) error
    IncrementErrorCount(ctx context.Context, id string) error
    ResetErrorCount(ctx context.Context, id string) error
    DeactivateSource(ctx context.Context, id string) error
}

type Service struct {
    repo         Repository
    parser       *Parser
    producer     *KafkaProducer
    logger       *slog.Logger
    maxWorkers   int
    maxRetries   int
    maxErrCount  int
}

type ServiceConfig struct {
    MaxWorkers  int // Максимум параллельных воркеров
    MaxRetries  int // Максимум попыток парсинга
    MaxErrCount int // Максимум ошибок до деактивации
}

func NewService(
    repo Repository,
    parser *Parser,
    producer *KafkaProducer,
    logger *slog.Logger,
    config ServiceConfig,
) *Service {
    return &Service{
        repo:        repo,
        parser:      parser,
        producer:    producer,
        logger:      logger,
        maxWorkers:  config.MaxWorkers,
        maxRetries:  config.MaxRetries,
        maxErrCount: config.MaxErrCount,
    }
}

// CollectFromDueSources собирает новости из всех источников, которые пора обновить.
func (s *Service) CollectFromDueSources(ctx context.Context) error {
    sources, err := s.repo.GetDueSources(ctx)
    if err != nil {
        return fmt.Errorf("failed to get due sources: %w", err)
    }

    if len(sources) == 0 {
        s.logger.Debug("no sources due for collection")
        return nil
    }

    s.logger.Info("starting collection", "sources_count", len(sources))

    // Worker pool для параллельной обработки
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, s.maxWorkers)

    for _, source := range sources {
        wg.Add(1)
        go func(src *models.Source) {
            defer wg.Done()

            semaphore <- struct{}{}        // Захватываем слот
            defer func() { <-semaphore }() // Освобождаем слот

            s.collectFromSource(ctx, src)
        }(source)
    }

    wg.Wait()
    s.logger.Info("collection completed", "sources_count", len(sources))

    return nil
}

// collectFromSource собирает новости из одного источника.
func (s *Service) collectFromSource(ctx context.Context, source *models.Source) {
    start := time.Now()
    s.logger.Info("collecting from source",
        "source_id", source.ID,
        "name", source.Name,
        "url", source.URL,
    )

    // Парсим RSS с retry
    news, err := s.parser.ParseFeedWithRetry(source.URL, s.maxRetries)
    if err != nil {
        s.handleError(ctx, source, err)
        return
    }

    // Устанавливаем source_id для каждой новости
    for _, item := range news {
        item.SourceID = source.ID
    }

    // Отправляем в Kafka
    var successCount, failCount int
    for _, item := range news {
        if err := s.producer.SendRawNews(item); err != nil {
            s.logger.Error("failed to send to kafka",
                "news_id", item.ID,
                "error", err,
            )
            failCount++
        } else {
            successCount++
        }
    }

    // Обновляем статус источника
    s.repo.UpdateLastCollected(ctx, source.ID, time.Now(), nil)
    s.repo.ResetErrorCount(ctx, source.ID)

    duration := time.Since(start)
    s.logger.Info("collection complete",
        "source_id", source.ID,
        "success", successCount,
        "failed", failCount,
        "duration", duration,
    )

    // Метрики
    collectionDuration.WithLabelValues(source.ID).Observe(duration.Seconds())
    newsCollected.WithLabelValues(source.ID).Add(float64(successCount))
    feedsFetched.WithLabelValues(source.ID, "success").Inc()
}

// handleError обрабатывает ошибку при сборе.
func (s *Service) handleError(ctx context.Context, source *models.Source, err error) {
    s.logger.Error("collection failed",
        "source_id", source.ID,
        "name", source.Name,
        "error", err,
    )

    // Обновляем статус с ошибкой
    s.repo.UpdateLastCollected(ctx, source.ID, time.Now(), err)
    s.repo.IncrementErrorCount(ctx, source.ID)

    // После N ошибок деактивируем источник
    if source.ErrorCount+1 >= s.maxErrCount {
        s.logger.Warn("deactivating source after max errors",
            "source_id", source.ID,
            "error_count", source.ErrorCount+1,
        )
        s.repo.DeactivateSource(ctx, source.ID)
    }

    // Метрики
    feedsFetched.WithLabelValues(source.ID, "error").Inc()
}

// ForceCollectAll принудительно собирает новости из всех активных источников.
func (s *Service) ForceCollectAll(ctx context.Context) error {
    sources, err := s.repo.GetAllActive(ctx)
    if err != nil {
        return err
    }

    for _, source := range sources {
        s.collectFromSource(ctx, source)
    }

    return nil
}
```

### 5. Repository (работа с БД)

**Файл:** `internal/store/sources/repository.go`

```go
package sources

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/FischukSergey/otus-ms/internal/models"
)

type Repository struct {
    db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{db: db}
}

// GetAllActive возвращает все активные источники.
func (r *Repository) GetAllActive(ctx context.Context) ([]*models.Source, error) {
    query := `
        SELECT id, name, url, language, category, fetch_interval, is_active,
               last_collected_at, last_error, error_count, created_at, updated_at
        FROM news_sources
        WHERE is_active = true
        ORDER BY name
    `

    rows, err := r.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query error: %w", err)
    }
    defer rows.Close()

    var sources []*models.Source
    for rows.Next() {
        var s models.Source
        if err := rows.Scan(
            &s.ID, &s.Name, &s.URL, &s.Language, &s.Category,
            &s.FetchInterval, &s.IsActive, &s.LastCollectedAt,
            &s.LastError, &s.ErrorCount, &s.CreatedAt, &s.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("scan error: %w", err)
        }
        sources = append(sources, &s)
    }

    return sources, rows.Err()
}

// GetDueSources возвращает источники, которые пора обновить.
func (r *Repository) GetDueSources(ctx context.Context) ([]*models.Source, error) {
    query := `
        SELECT id, name, url, language, category, fetch_interval, is_active,
               last_collected_at, last_error, error_count, created_at, updated_at
        FROM news_sources
        WHERE is_active = true
          AND (
              last_collected_at IS NULL
              OR last_collected_at + (fetch_interval * interval '1 second') <= NOW()
          )
        ORDER BY last_collected_at NULLS FIRST
    `

    rows, err := r.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query error: %w", err)
    }
    defer rows.Close()

    var sources []*models.Source
    for rows.Next() {
        var s models.Source
        if err := rows.Scan(
            &s.ID, &s.Name, &s.URL, &s.Language, &s.Category,
            &s.FetchInterval, &s.IsActive, &s.LastCollectedAt,
            &s.LastError, &s.ErrorCount, &s.CreatedAt, &s.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("scan error: %w", err)
        }
        sources = append(sources, &s)
    }

    return sources, rows.Err()
}

// UpdateLastCollected обновляет время последнего сбора.
func (r *Repository) UpdateLastCollected(ctx context.Context, id string, collectedAt time.Time, errMsg error) error {
    var lastError sql.NullString
    if errMsg != nil {
        lastError = sql.NullString{String: errMsg.Error(), Valid: true}
    }

    query := `
        UPDATE news_sources
        SET last_collected_at = $1,
            last_error = $2,
            updated_at = NOW()
        WHERE id = $3
    `

    _, err := r.db.ExecContext(ctx, query, collectedAt, lastError, id)
    return err
}

// IncrementErrorCount увеличивает счётчик ошибок.
func (r *Repository) IncrementErrorCount(ctx context.Context, id string) error {
    query := `
        UPDATE news_sources
        SET error_count = error_count + 1,
            updated_at = NOW()
        WHERE id = $1
    `

    _, err := r.db.ExecContext(ctx, query, id)
    return err
}

// ResetErrorCount сбрасывает счётчик ошибок.
func (r *Repository) ResetErrorCount(ctx context.Context, id string) error {
    query := `
        UPDATE news_sources
        SET error_count = 0,
            updated_at = NOW()
        WHERE id = $1
    `

    _, err := r.db.ExecContext(ctx, query, id)
    return err
}

// DeactivateSource деактивирует источник.
func (r *Repository) DeactivateSource(ctx context.Context, id string) error {
    query := `
        UPDATE news_sources
        SET is_active = false,
            updated_at = NOW()
        WHERE id = $1
    `

    _, err := r.db.ExecContext(ctx, query, id)
    return err
}

// CRUD методы...

// Create создаёт новый источник.
func (r *Repository) Create(ctx context.Context, source *models.Source) error {
    query := `
        INSERT INTO news_sources (id, name, url, language, category, fetch_interval, is_active)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

    _, err := r.db.ExecContext(ctx, query,
        source.ID, source.Name, source.URL, source.Language,
        source.Category, source.FetchInterval, source.IsActive,
    )
    return err
}

// GetByID возвращает источник по ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*models.Source, error) {
    query := `
        SELECT id, name, url, language, category, fetch_interval, is_active,
               last_collected_at, last_error, error_count, created_at, updated_at
        FROM news_sources
        WHERE id = $1
    `

    var s models.Source
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &s.ID, &s.Name, &s.URL, &s.Language, &s.Category,
        &s.FetchInterval, &s.IsActive, &s.LastCollectedAt,
        &s.LastError, &s.ErrorCount, &s.CreatedAt, &s.UpdatedAt,
    )

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("source not found")
    }

    return &s, err
}

// Update обновляет источник.
func (r *Repository) Update(ctx context.Context, source *models.Source) error {
    query := `
        UPDATE news_sources
        SET name = $1, url = $2, language = $3, category = $4,
            fetch_interval = $5, is_active = $6, updated_at = NOW()
        WHERE id = $7
    `

    _, err := r.db.ExecContext(ctx, query,
        source.Name, source.URL, source.Language, source.Category,
        source.FetchInterval, source.IsActive, source.ID,
    )
    return err
}

// Delete удаляет источник.
func (r *Repository) Delete(ctx context.Context, id string) error {
    query := `DELETE FROM news_sources WHERE id = $1`
    _, err := r.db.ExecContext(ctx, query, id)
    return err
}
```

---

## API эндпоинты

### Административные эндпоинты

```
# Health checks
GET    /health                          # Проверка работоспособности
GET    /ready                           # Готовность к работе

# Управление источниками
GET    /api/v1/sources                  # Список всех источников
GET    /api/v1/sources/{id}             # Получить источник по ID
POST   /api/v1/sources                  # Создать новый источник
PUT    /api/v1/sources/{id}             # Обновить источник
DELETE /api/v1/sources/{id}             # Удалить источник
POST   /api/v1/sources/{id}/activate    # Активировать источник
POST   /api/v1/sources/{id}/deactivate  # Деактивировать источник

# Управление коллектором
POST   /api/v1/collector/run            # Принудительный запуск сбора
POST   /api/v1/collector/run/{id}       # Запуск сбора для конкретного источника
GET    /api/v1/collector/status         # Статус коллектора
```

### Примеры запросов

#### Получить список источников

```bash
curl http://localhost:38082/api/v1/sources
```

Ответ:
```json
{
  "sources": [
    {
      "id": "source_1",
      "name": "Lenta.ru Технологии",
      "url": "https://lenta.ru/rss/tech",
      "language": "ru",
      "category": "tech",
      "fetch_interval": 3600,
      "is_active": true,
      "last_collected_at": "2026-02-15T10:00:00Z",
      "error_count": 0,
      "created_at": "2026-02-01T00:00:00Z"
    }
  ]
}
```

#### Создать новый источник

```bash
curl -X POST http://localhost:38082/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "id": "source_16",
    "name": "Custom Feed",
    "url": "https://example.com/rss",
    "language": "ru",
    "category": "tech",
    "fetch_interval": 3600
  }'
```

#### Принудительный запуск сбора

```bash
curl -X POST http://localhost:38082/api/v1/collector/run
```

Ответ:
```json
{
  "status": "started",
  "message": "collection started for all active sources"
}
```

---

## Конфигурация

### Файл конфигурации

**Путь:** `configs/config.news-collector.local.yaml`

```yaml
global:
  env: local  # local, dev, prod

log:
  level: info     # debug, info, warn, error
  format: text    # text, json

servers:
  client:
    addr: 0.0.0.0:38082      # API сервер
    allow_origins:
      - "*"                   # CORS origins
  debug:
    addr: 0.0.0.0:33002      # Debug endpoints (pprof)
  metrics:
    addr: 0.0.0.0:9092       # Prometheus метрики

db:
  host: localhost
  port: 5432
  name: news_db
  user: news_user
  password: ${NEWS_DB_PASSWORD}  # Из environment variable
  ssl_mode: disable

kafka:
  brokers:
    - localhost:9092
  topics:
    raw_news: raw_news
  producer:
    max_retries: 3
    retry_backoff: 100ms

collector:
  workers: 3                     # Параллельных воркеров для сбора
  http_timeout: 30s              # Таймаут HTTP запросов к RSS
  max_retries: 3                 # Максимум попыток парсинга одного фида
  max_error_count: 5             # Максимум ошибок до деактивации источника
  check_interval: 60s            # Как часто проверять источники (cron)
```

### Production конфигурация

**Путь:** `configs/config.news-collector.prod.yaml`

```yaml
global:
  env: prod

log:
  level: info
  format: json  # JSON для production

servers:
  client:
    addr: 0.0.0.0:38082
    allow_origins:
      - "https://fishouk-otus-ms.ru"
  debug:
    addr: 0.0.0.0:33002
  metrics:
    addr: 0.0.0.0:9092

db:
  host: postgres
  port: 5432
  name: news_db
  user: news_user
  password: ${NEWS_DB_PASSWORD}
  ssl_mode: require

kafka:
  brokers:
    - kafka-1:9092
    - kafka-2:9092
    - kafka-3:9092
  topics:
    raw_news: raw_news
  producer:
    max_retries: 5
    retry_backoff: 200ms

collector:
  workers: 5
  http_timeout: 30s
  max_retries: 3
  max_error_count: 5
  check_interval: 60s
```

---

## Обработка ошибок

### Стратегия обработки ошибок

#### 1. Таймауты HTTP

```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
}
```

**Зачем:** Предотвращает зависание на медленных источниках.

#### 2. Retry логика для RSS парсинга

```go
func (p *Parser) ParseFeedWithRetry(url string, maxRetries int) ([]*models.RawNews, error) {
    for attempt := 0; attempt < maxRetries; attempt++ {
        news, err := p.ParseFeed(url)
        if err == nil {
            return news, nil
        }
        
        backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        time.Sleep(backoff)
    }
    return nil, fmt.Errorf("max retries exceeded")
}
```

**Backoff:** 1s, 2s, 4s (экспоненциальная задержка)

#### 3. Kafka retry

Встроенный механизм Sarama:
```go
config.Producer.Retry.Max = 3
```

#### 4. Деактивация проблемных источников

После **5 последовательных ошибок** источник автоматически деактивируется:

```go
if source.ErrorCount >= 5 {
    s.repo.DeactivateSource(ctx, source.ID)
}
```

**Реактивация:** Вручную через API или после исправления URL.

#### 5. Circuit Breaker (опционально)

Для защиты от каскадных сбоев можно добавить circuit breaker:

```go
import "github.com/sony/gobreaker"

cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "rss-parser",
    MaxRequests: 3,
    Interval:    time.Minute,
    Timeout:     time.Minute * 2,
})
```

### Логирование ошибок

Все ошибки логируются с контекстом:

```go
s.logger.Error("collection failed",
    "source_id", source.ID,
    "name", source.Name,
    "url", source.URL,
    "error", err,
    "attempt", attempt,
)
```

### Dead Letter Queue (будущее)

Для более сложной обработки ошибок можно добавить DLQ:
- Неудачные сообщения отправляются в отдельный топик `raw_news_dlq`
- Processor может повторно обработать их позже

---

## Метрики и мониторинг

### Prometheus метрики

**Файл:** `internal/services/collector/metrics.go`

```go
package collector

import "github.com/prometheus/client_golang/prometheus"

var (
    // Количество собранных фидов
    feedsFetched = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "news_collector_feeds_fetched_total",
            Help: "Total number of RSS feeds fetched",
        },
        []string{"source_id", "status"}, // status: success, error
    )

    // Количество собранных новостей
    newsCollected = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "news_collector_news_collected_total",
            Help: "Total number of news items collected",
        },
        []string{"source_id"},
    )

    // Длительность сбора
    collectionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "news_collector_collection_duration_seconds",
            Help:    "Duration of news collection per source",
            Buckets: prometheus.DefBuckets,
        },
        []string{"source_id"},
    )

    // Активные источники
    activeSources = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "news_collector_active_sources",
            Help: "Number of currently active news sources",
        },
    )

    // Источники с ошибками
    errorSources = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "news_collector_error_sources",
            Help: "Number of sources with errors",
        },
        []string{"error_count"}, // 1, 2, 3, 4, 5
    )
)

func init() {
    prometheus.MustRegister(feedsFetched)
    prometheus.MustRegister(newsCollected)
    prometheus.MustRegister(collectionDuration)
    prometheus.MustRegister(activeSources)
    prometheus.MustRegister(errorSources)
}
```

### Health checks

```go
// GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
    })
}

// GET /ready
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
    // Проверяем доступность БД и Kafka
    if !h.checkDatabase() || !h.checkKafka() {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "not ready",
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ready",
    })
}
```

### Grafana дашборд

Рекомендуемые графики:

1. **Сбор новостей** (rate per minute)
   ```promql
   rate(news_collector_news_collected_total[5m])
   ```

2. **Успешные vs неудачные запросы**
   ```promql
   sum(rate(news_collector_feeds_fetched_total{status="success"}[5m]))
   sum(rate(news_collector_feeds_fetched_total{status="error"}[5m]))
   ```

3. **Длительность сбора** (P50, P95, P99)
   ```promql
   histogram_quantile(0.95, rate(news_collector_collection_duration_seconds_bucket[5m]))
   ```

4. **Активные источники**
   ```promql
   news_collector_active_sources
   ```

---

## Деплой и инфраструктура

### Dockerfile

**Файл:** `news-collector.Dockerfile`

```dockerfile
# Multi-stage build для минимального размера образа

# Stage 1: Builder
FROM golang:1.23.8-alpine AS builder

WORKDIR /build

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o news-collector \
    ./cmd/news-collector

# Stage 2: Runtime
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS и tzdata для часовых поясов
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем бинарник из builder
COPY --from=builder /build/news-collector .

# Создаём непривилегированного пользователя
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 38082 33002 9092

CMD ["./news-collector", "-config", "configs/config.news-collector.yaml"]
```

### Docker Compose (локальная разработка)

**Файл:** `deploy/local/docker-compose.local.yml` (добавить сервис)

```yaml
services:
  # ... существующие сервисы (postgres, redis, keycloak)

  # Kafka + Zookeeper
  zookeeper:
    image: wurstmeister/zookeeper:latest
    ports:
      - "2181:2181"
    networks:
      - otus-network

  kafka:
    image: wurstmeister/kafka:latest
    ports:
      - "9092:9092"
    environment:
      KAFKA_ADVERTISED_HOST_NAME: localhost
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_CREATE_TOPICS: "raw_news:1:1,processed_news:1:1,user_actions:1:1"
    depends_on:
      - zookeeper
    networks:
      - otus-network

  # News Collector Service
  news-collector:
    build:
      context: ../..
      dockerfile: news-collector.Dockerfile
    ports:
      - "38082:38082"  # API
      - "33002:33002"  # Debug
      - "9092:9092"    # Metrics
    environment:
      NEWS_DB_PASSWORD: ${NEWS_DB_PASSWORD:-postgres}
    volumes:
      - ../../configs:/app/configs:ro
    depends_on:
      - postgres
      - kafka
    networks:
      - otus-network
    restart: unless-stopped

networks:
  otus-network:
    driver: bridge
```

### Production (docker-compose)

**Файл:** `deploy/prod/docker-compose.news-collector.prod.yml`

```yaml
version: '3.8'

services:
  news-collector:
    image: cr.selcloud.ru/otus-microservice-be/news-collector:latest
    container_name: news-collector
    ports:
      - "38082:38082"
      - "9092:9092"
    environment:
      NEWS_DB_PASSWORD: ${NEWS_DB_PASSWORD}
    volumes:
      - ./configs/config.news-collector.prod.yaml:/app/configs/config.news-collector.yaml:ro
    restart: unless-stopped
    networks:
      - otus-network
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

networks:
  otus-network:
    external: true
```

### CI/CD (GitHub Actions)

**Файл:** `.github/workflows/deploy-news-collector.yml`

```yaml
name: Deploy News Collector

on:
  push:
    branches: [main]
    paths:
      - 'cmd/news-collector/**'
      - 'internal/services/collector/**'
      - 'internal/store/sources/**'
      - 'news-collector.Dockerfile'
  workflow_dispatch:

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Selectel Registry
        uses: docker/login-action@v3
        with:
          registry: cr.selcloud.ru
          username: ${{ secrets.SELECTEL_REGISTRY_USERNAME }}
          password: ${{ secrets.SELECTEL_REGISTRY_PASSWORD }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./news-collector.Dockerfile
          push: true
          tags: cr.selcloud.ru/otus-microservice-be/news-collector:latest
          cache-from: type=registry,ref=cr.selcloud.ru/otus-microservice-be/news-collector:buildcache
          cache-to: type=registry,ref=cr.selcloud.ru/otus-microservice-be/news-collector:buildcache,mode=max

      - name: Deploy to production
        uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.SERVER_HOST }}
          username: ${{ secrets.SERVER_USER }}
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          script: |
            cd /opt/otus-ms
            docker compose -f deploy/prod/docker-compose.news-collector.prod.yml pull
            docker compose -f deploy/prod/docker-compose.news-collector.prod.yml up -d
            docker image prune -f
```

### Nginx конфигурация

Добавить в существующий `nginx.conf`:

```nginx
# News Collector Service
location /api/v1/sources {
    proxy_pass http://news-collector:38082;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}

location /api/v1/collector {
    proxy_pass http://news-collector:38082;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    
    # Только для администраторов (опционально)
    # auth_request /auth/validate;
}
```

---

## План реализации

### Фаза 1: MVP (День 1-3) ⭐⭐⭐

**Цель:** Минимально работающий коллектор

- [x] Создать структуру проекта `cmd/news-collector/`
- [x] Добавить модели `models/source.go`, `models/raw_news.go`
- [x] Создать миграцию `003_create_news_sources.sql`
- [x] Seed данные для 15 источников
- [x] Реализовать `Parser` (без retry)
- [x] Простой `Scheduler` (каждые N минут)
- [x] Базовый `KafkaProducer`
- [x] Простая логика `Service.CollectFromDueSources()`
- [x] Health check `/health`
- [x] Docker Compose с Kafka

**Проверка:** Сервис запускается, собирает новости, отправляет в Kafka.

### Фаза 2: Устойчивость (День 4-5) ⭐⭐

**Цель:** Надёжная обработка ошибок

- [x] Retry логика в `Parser`
- [x] Обработка ошибок в `Service`
- [x] Деактивация проблемных источников
- [x] Счётчик ошибок в БД
- [x] Структурированное логирование
- [x] Таймауты HTTP

**Проверка:** Сервис корректно обрабатывает недоступные источники.

### Фаза 3: Управление (День 6-7) ⭐⭐

**Цель:** API для администрирования

- [x] HTTP handlers `handlers/collector/`
- [x] CRUD для источников
- [x] Принудительный запуск `/api/v1/collector/run`
- [x] Статус коллектора `/api/v1/collector/status`
- [x] Активация/деактивация источников

**Проверка:** Можно управлять источниками через API.

### Фаза 4: Мониторинг (День 8) ⭐

**Цель:** Observability

- [x] Prometheus метрики
- [x] Metrics server на порту 9092
- [x] Debug server (pprof)
- [x] Grafana дашборд (опционально)

**Проверка:** Метрики доступны в Prometheus.

### Фаза 5: Оптимизация (День 9-10) ⭐⭐

**Цель:** Производительность

- [x] Worker pool для параллельных запросов
- [x] Connection pooling для БД
- [x] Batch отправка в Kafka (опционально)
- [x] Graceful shutdown

**Проверка:** Сервис обрабатывает 15 источников параллельно.

### Фаза 6: Production (День 11-12) ⭐⭐⭐

**Цель:** Готовность к продакшену

- [x] Production конфигурация
- [x] CI/CD pipeline
- [x] Dockerfile оптимизация
- [x] Nginx интеграция
- [x] Документация API (Swagger)
- [x] Тесты (unit + integration)

**Проверка:** Сервис задеплоен на production, работает стабильно.

---

## Рекомендации

### 🎯 Начните с простого

1. **Первый MVP:** Один источник, без Kafka — просто логируйте результаты
2. **Инкрементальное тестирование:** После каждого компонента проверяйте работу
3. **Не оптимизируйте преждевременно:** Сначала работающий код, потом оптимизация

### 🏗️ Используйте существующую архитектуру

- Ваша структура `config/`, `logger/`, `store/` уже готова — переиспользуйте
- Следуйте паттерну `main-service` и `auth-proxy`
- Используйте те же инструменты: `chi`, `slog`, `cleanenv`

### 📦 Kafka через Docker

```yaml
# Простой вариант для локальной разработки
services:
  zookeeper:
    image: wurstmeister/zookeeper:latest
  kafka:
    image: wurstmeister/kafka:latest
    environment:
      KAFKA_ADVERTISED_HOST_NAME: localhost
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
```

### 🔍 Тестирование

**Unit тесты:**
```bash
go test ./internal/services/collector/...
go test ./internal/store/sources/...
```

**Интеграционные тесты:**
- Поднимите тестовый Kafka в Docker
- Используйте testcontainers для изоляции

**Мануальное тестирование:**
```bash
# 1. Проверить health
curl http://localhost:38082/health

# 2. Получить источники
curl http://localhost:38082/api/v1/sources

# 3. Запустить принудительный сбор
curl -X POST http://localhost:38082/api/v1/collector/run

# 4. Проверить Kafka
docker exec -it kafka kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic raw_news \
  --from-beginning
```

### 🚀 Деплой

1. **Локально:** `docker compose up`
2. **Production:** GitHub Actions автоматически соберёт и задеплоит

### 📊 Мониторинг

- Prometheus: `http://localhost:9092/metrics`
- Grafana: создать дашборд с графиками
- Логи: `docker logs news-collector -f`

### ⚡ Оптимизация (для будущего)

1. **Кеширование RSS:** Использовать `ETag` / `Last-Modified` заголовки
2. **Инкрементальный сбор:** Хранить `last_entry_id` и собирать только новые
3. **Batch processing:** Отправлять в Kafka пакетами по 10-20 новостей
4. **CDN для изображений:** Скачивать и хостить изображения локально

### 🔐 Безопасность

1. **Валидация URL:** Проверять, что URL ведёт на допустимый домен
2. **Rate limiting:** Не более N запросов в минуту к одному домену
3. **Sanitization:** Очищать HTML от вредоносных скриптов
4. **Authentication:** Защитить административные API через Auth-Proxy

---

## Заключение

News Collector Service — это фундамент вашего агрегатора новостей. Следуя этому плану, вы создадите:

✅ Отказоустойчивый сервис сбора новостей  
✅ Интеграцию с Kafka для передачи данных  
✅ Административный API для управления  
✅ Мониторинг и метрики  
✅ Production-ready решение  

**Следующие шаги:**
1. Начните с MVP (Фаза 1)
2. Протестируйте на локальном окружении
3. Добавьте обработку ошибок (Фаза 2)
4. Реализуйте API (Фаза 3)
5. Подключите мониторинг (Фаза 4)
6. Задеплойте на production (Фаза 6)

Удачи с реализацией! 🚀

---

**Автор:** AI Assistant  
**Дата:** 15 февраля 2026  
**Версия:** 1.0
