-- Миграция 004: добавить ключ S3-артефакта в таблицу новостей

ALTER TABLE news
ADD COLUMN IF NOT EXISTS s3_key TEXT;

COMMENT ON COLUMN news.s3_key IS 'Ключ объекта с артефактом в S3-совместимом хранилище';
