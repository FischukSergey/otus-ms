-- Миграция 007: индекс для политики хранения новостей (retention policy)
-- Ускоряет периодическое удаление устаревших записей: DELETE FROM news WHERE created_at < $1

CREATE INDEX IF NOT EXISTS idx_news_created_at ON news(created_at);
