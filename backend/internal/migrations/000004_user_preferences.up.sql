-- =============================================================================
-- 000004_user_preferences: preferred locale & currency (TDD §3.4, PRD §3.5/§3.2.3)
--
-- Per TDD §3.4 these live as columns on `users` (not in profile_data JSONB),
-- because they drive i18n locale selection and FX presentment conversion at
-- read time — they are first-class query/scan fields, not loose JSON.
--
-- Launch locales (PRD §3.5.1): en-US (default), zh-CN.
-- Presentment currencies (PRD §3.2.3): USD, EUR, GBP. CNY is base/settlement
-- only and is never a customer-facing presentment choice, so it is excluded
-- by the CHECK. Defaults: en-US / USD.
-- =============================================================================

ALTER TABLE users
    ADD COLUMN preferred_locale   VARCHAR(10) NOT NULL DEFAULT 'en-US',
    ADD COLUMN preferred_currency CHAR(3)    NOT NULL DEFAULT 'USD',
    ADD CONSTRAINT chk_preferred_currency
        CHECK (preferred_currency IN ('USD', 'EUR', 'GBP'));

-- Backfill any rows that pre-date the columns (NOT NULL DEFAULT handles new
-- rows, but existing rows get the default at ADD COLUMN time in PG anyway).
UPDATE users SET preferred_locale = 'en-US'   WHERE preferred_locale   IS NULL;
UPDATE users SET preferred_currency = 'USD'   WHERE preferred_currency IS NULL;
