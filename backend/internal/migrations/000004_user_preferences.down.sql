-- Reverse of 000004_user_preferences.

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS chk_preferred_currency,
    DROP COLUMN IF EXISTS preferred_currency,
    DROP COLUMN IF EXISTS preferred_locale;
