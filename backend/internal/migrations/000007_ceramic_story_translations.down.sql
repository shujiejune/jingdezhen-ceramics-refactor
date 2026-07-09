-- Reverse of 000007_ceramic_story_translations.

DROP TABLE IF EXISTS ceramic_story_translations;

ALTER TABLE ceramic_stories
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;
