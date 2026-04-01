-- Миграция 003: таблица обработанных новостей

CREATE TABLE IF NOT EXISTS news (
    id           UUID          PRIMARY KEY,
    source_id    VARCHAR(100)  REFERENCES news_sources(id) ON DELETE SET NULL,
    title        VARCHAR(500)  NOT NULL,
    summary      TEXT,
    url          VARCHAR(1000) NOT NULL UNIQUE,
    category     VARCHAR(50),
    tags         JSONB         NOT NULL DEFAULT '[]',
    published_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  news                  IS 'Обработанные новости от news-processor';
COMMENT ON COLUMN news.summary          IS 'Краткое резюме: первые 2-3 предложения очищенного текста';
COMMENT ON COLUMN news.tags             IS 'Массив тегов, извлечённых из контента';
COMMENT ON COLUMN news.processed_at     IS 'Время обработки сервисом news-processor';

CREATE INDEX IF NOT EXISTS idx_news_source_id    ON news(source_id);
CREATE INDEX IF NOT EXISTS idx_news_category     ON news(category);
CREATE INDEX IF NOT EXISTS idx_news_published_at ON news(published_at DESC);
