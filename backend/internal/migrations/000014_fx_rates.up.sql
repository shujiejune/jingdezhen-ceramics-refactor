-- =============================================================================
-- 000014_fx_rates: daily FX rate store (TDD §3.4, §7; PRD §3.2.3)
--
-- One row per presentment currency (USD/EUR/GBP). rate_to_cny is the
-- CNY-per-unit-of-currency rate AFTER the 2% markup is applied, so read-time
-- conversion is a single division: presentment = (cnyMinor/100) / rate_to_cny.
--
-- Refreshed daily by the fx:refresh job (ECB EUR-base → derive CNY→{USD,EUR,GBP}
-- → apply markup → upsert here). Cached per day; orders snapshot the rate at
-- checkout (TDD §7).
-- =============================================================================

CREATE TABLE fx_rates (
    currency   CHAR(3) PRIMARY KEY,                 -- USD, EUR, GBP (presentment only)
    rate_to_cny NUMERIC(18,8) NOT NULL,             -- CNY per 1 unit of currency, post-markup
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
