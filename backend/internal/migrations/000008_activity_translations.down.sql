-- Reverse of 000008_activity_translations.

DROP TABLE IF EXISTS activity_translations;

ALTER TABLE activities
    DROP COLUMN IF EXISTS opening_info,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS lng,
    DROP COLUMN IF EXISTS lat;
