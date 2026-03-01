-- Миграция 002: таблица источников новостей и начальные данные (seed)

CREATE TABLE IF NOT EXISTS news_sources (
    id             VARCHAR(100)  PRIMARY KEY,
    name           VARCHAR(200)  NOT NULL,
    url            VARCHAR(1000) NOT NULL,
    language       VARCHAR(10),
    category       VARCHAR(50),
    fetch_interval INTEGER       NOT NULL DEFAULT 3600,
    is_active      BOOLEAN       NOT NULL DEFAULT TRUE,
    last_collected_at TIMESTAMPTZ,
    last_error     TEXT,
    error_count    INTEGER       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN news_sources.fetch_interval IS 'Интервал опроса в секундах (3600 = 1 час)';
COMMENT ON COLUMN news_sources.error_count    IS 'Сбрасывается при успешном сборе; после 5 ошибок источник деактивируется';

CREATE INDEX IF NOT EXISTS idx_sources_active     ON news_sources(is_active);
CREATE INDEX IF NOT EXISTS idx_sources_next_fetch ON news_sources(last_collected_at, fetch_interval);

-- Seed: 15 начальных источников новостей
INSERT INTO news_sources (id, name, url, language, category, fetch_interval) VALUES
    ('source_1',  'Lenta.ru Технологии',  'https://lenta.ru/rss/tech',                                                    'ru', 'tech',     3600),
    ('source_2',  'RBC Новости',           'https://rssexport.rbc.ru/rbcnews/news/20/full.rss',                             'ru', 'news',     1800),
    ('source_3',  'Habr Все потоки',       'https://habr.com/ru/rss/all/',                                                  'ru', 'tech',     1800),
    ('source_4',  'Kommersant Главное',    'https://www.kommersant.ru/RSS/main.xml',                                        'ru', 'news',     3600),
    ('source_5',  'Фонтанка Новости',       'https://www.fontanka.ru/rss-feeds/rss.xml',                                     'ru', 'news',     1800),
    ('source_6',  'РИА Новости',            'https://ria.ru/export/rss2/archive/index.xml',                                 'ru', 'news',     1800),
    ('source_7',  'Газeta.ru',             'https://www.gazeta.ru/export/rss/first.xml',                                    'ru', 'news',     1800),
    ('source_8',  'Forbes Технологии',     'https://www.forbes.ru/rss-tag-category.xml?tag=2',                              'ru', 'tech',     7200),
    ('source_9',  'VC.ru',                 'https://vc.ru/rss/all',                                                         'ru', 'tech',     1800)
ON CONFLICT (id) DO NOTHING;
