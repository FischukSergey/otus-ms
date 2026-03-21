-- Миграция 006: исправление типов preferred_* в user_news_preferences на TEXT[]

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'user_news_preferences'
          AND column_name = 'preferred_categories'
          AND data_type = 'text'
    ) THEN
        ALTER TABLE user_news_preferences
            ALTER COLUMN preferred_categories TYPE TEXT[]
            USING CASE
                WHEN preferred_categories IS NULL OR btrim(preferred_categories) = '' THEN '{}'::TEXT[]
                ELSE ARRAY[preferred_categories]
            END;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'user_news_preferences'
          AND column_name = 'preferred_sources'
          AND data_type = 'text'
    ) THEN
        ALTER TABLE user_news_preferences
            ALTER COLUMN preferred_sources TYPE TEXT[]
            USING CASE
                WHEN preferred_sources IS NULL OR btrim(preferred_sources) = '' THEN '{}'::TEXT[]
                ELSE ARRAY[preferred_sources]
            END;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'user_news_preferences'
          AND column_name = 'preferred_keywords'
          AND data_type = 'text'
    ) THEN
        ALTER TABLE user_news_preferences
            ALTER COLUMN preferred_keywords TYPE TEXT[]
            USING CASE
                WHEN preferred_keywords IS NULL OR btrim(preferred_keywords) = '' THEN '{}'::TEXT[]
                ELSE ARRAY[preferred_keywords]
            END;
    END IF;
END $$;

ALTER TABLE user_news_preferences
    ALTER COLUMN preferred_categories SET DEFAULT '{}'::TEXT[],
    ALTER COLUMN preferred_sources SET DEFAULT '{}'::TEXT[],
    ALTER COLUMN preferred_keywords SET DEFAULT '{}'::TEXT[];
