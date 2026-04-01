-- Миграция 008: таблицы алертинга по ключевым словам (Telegram MVP)

CREATE TABLE IF NOT EXISTS alert_rules (
    id               UUID PRIMARY KEY,
    user_uuid        UUID         NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    keyword          VARCHAR(200) NOT NULL,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    channel_type     VARCHAR(20)  NOT NULL DEFAULT 'telegram' CHECK (channel_type IN ('telegram')),
    channel_target   TEXT,
    cooldown_seconds INTEGER      NOT NULL DEFAULT 300 CHECK (cooldown_seconds >= 0),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE alert_rules IS 'Правила алертинга пользователя по ключевым словам';
COMMENT ON COLUMN alert_rules.channel_target IS 'Опционально: target канала (в MVP используется общий project chat)';

CREATE INDEX IF NOT EXISTS idx_alert_rules_user_active
    ON alert_rules(user_uuid, is_active);
CREATE INDEX IF NOT EXISTS idx_alert_rules_keyword
    ON alert_rules(lower(keyword));

CREATE TABLE IF NOT EXISTS alert_events (
    id              UUID PRIMARY KEY,
    rule_id         UUID         NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    news_id         UUID         NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    user_uuid       UUID         NOT NULL REFERENCES users(uuid) ON DELETE CASCADE,
    keyword         VARCHAR(200) NOT NULL,
    delivery_status VARCHAR(20)  NOT NULL CHECK (delivery_status IN ('pending', 'sent', 'failed', 'dropped')),
    error_message   TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (rule_id, news_id)
);

COMMENT ON TABLE alert_events IS 'История сгенерированных и доставленных алертов';

CREATE INDEX IF NOT EXISTS idx_alert_events_user_created
    ON alert_events(user_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_rule_sent
    ON alert_events(rule_id, sent_at DESC);
