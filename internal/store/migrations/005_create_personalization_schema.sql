-- Миграция 005: таблицы персонализации и FTS-индекса

CREATE TABLE IF NOT EXISTS user_news_preferences (
    user_uuid             UUID PRIMARY KEY REFERENCES users(uuid) ON DELETE CASCADE,
    preferred_categories  TEXT[]          NOT NULL DEFAULT '{}'::TEXT[],
    preferred_sources     TEXT[]          NOT NULL DEFAULT '{}'::TEXT[],
    preferred_keywords    TEXT[]          NOT NULL DEFAULT '{}'::TEXT[],
    preferred_language    VARCHAR(10),
    from_hours            INTEGER         NOT NULL DEFAULT 168 CHECK (from_hours > 0 AND from_hours <= 720),
    created_at            TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_news_preferences IS 'Предпочтения пользователя для персонализированной ленты новостей';
COMMENT ON COLUMN user_news_preferences.from_hours IS 'Окно новостей в часах для выдачи (по умолчанию 168 = 7 дней)';

CREATE TABLE IF NOT EXISTS user_news_events (
    id          UUID PRIMARY KEY,
    user_uuid   UUID         NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    news_id     UUID         NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    event_type  VARCHAR(20)  NOT NULL CHECK (event_type IN ('view', 'click', 'like', 'dislike', 'hide')),
    metadata    JSONB        NOT NULL DEFAULT '{}'::JSONB,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_news_events IS 'События взаимодействия пользователя с новостями для personalization MVP';

CREATE INDEX IF NOT EXISTS idx_user_news_events_user_created
    ON user_news_events(user_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_news_events_news
    ON user_news_events(news_id);
CREATE INDEX IF NOT EXISTS idx_user_news_events_type
    ON user_news_events(event_type);

CREATE TABLE IF NOT EXISTS news_search_index (
    news_id         UUID PRIMARY KEY REFERENCES news(id) ON DELETE CASCADE,
    body_text       TEXT,
    tags_text       TEXT,
    search_vector   TSVECTOR    NOT NULL DEFAULT ''::TSVECTOR,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE news_search_index IS 'Поисковый индекс для FTS и ранжирования новостей';
COMMENT ON COLUMN news_search_index.search_vector IS 'tsvector по полям новости (title/summary/body/tags) для websearch_to_tsquery';

CREATE INDEX IF NOT EXISTS idx_news_search_index_vector
    ON news_search_index USING GIN(search_vector);
