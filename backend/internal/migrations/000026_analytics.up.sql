-- =============================================================================
-- 000026_analytics: in-house analytics ingest + nightly rollup (TDD §3.4/§4.2,
-- PRD §3.4.2)
--
-- PRD §3.4.2 "in-house lightweight analytics": first-party event endpoint +
-- PostgreSQL, IP geolocation via a local MaxMind GeoLite2 db. Google Analytics
-- is blocked in mainland China (blind spot); first-party aggregation simplifies
-- GDPR; the dashboard requirements are custom anyway.
--
-- Schema (TDD §194):
--   analytics_events(id, ts, kind pageview|event, path, name NULL, country CHAR(2),
--                    locale, visitor_hash, props JSONB)
--   analytics_daily(date, metric, dims JSONB, value)   -- nightly job
--
-- Visitor hash = HMAC(daily-rotating key, IP+UA) → no raw IPs stored (GDPR
-- minimisation, TDD §11). GeoLite2 lookup happens at ingest (country CHAR(2)).
-- Unknown/private IP → 'ZZ'. Consent gate is enforced in code (consent
-- cookie_analytics lookup by IP hash): not-consented → event silently dropped.
--
-- PARTITIONING NOTE: TDD §194 comments "partitioned by month". For the
-- single-VPS MVP volume a single table is simpler and the BRIN index on ts
-- keeps the rollup + prune queries cheap; the schema is shaped (ts NOT NULL,
-- PK on (id, ts)) to convert to RANGE partitioning later in one migration
-- (CREATE TABLE … PARTITION BY RANGE (ts) + monthly partitions + a retention
-- job). Not doing it now avoids a monthly partition-creation cron for no
-- MVP benefit.
--
-- The dashboard's sales/GMV and itinerary funnel (submitted/confirmed) halves
-- are NOT duplicated here — they are queried live from orders + itinerary
-- requests. Only the view/started funnel signals come from analytics_events.
-- =============================================================================

CREATE TABLE analytics_events (
    id            BIGSERIAL,
    ts            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    kind          VARCHAR(20) NOT NULL
                  CHECK (kind IN ('pageview', 'event')),
    path          TEXT NOT NULL,
    name          TEXT,                          -- NULL for pageview; set for events
    country       CHAR(2) NOT NULL DEFAULT 'ZZ',  -- ISO 3166-1 alpha-2; 'ZZ' = unknown
    locale        VARCHAR(10),                    -- BCP 47 (en-US, zh-CN); NULL if client omitted
    visitor_hash  VARCHAR(64) NOT NULL,           -- hex(HMAC(dailyKey, IP+UA)) — no raw IP
    props         JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (id, ts)                          -- composite PK: ready for RANGE partitioning
);

-- Rollup queries scan a recent ts window; BRIN is cheap on append-only ts.
CREATE INDEX idx_analytics_events_ts_brin   ON analytics_events USING BRIN (ts);
CREATE INDEX idx_analytics_events_kind_ts   ON analytics_events (kind, ts);
CREATE INDEX idx_analytics_events_path_ts   ON analytics_events (path, ts);
-- Unique-visitor counts group by visitor_hash.
CREATE INDEX idx_analytics_events_visitor   ON analytics_events (visitor_hash);

-- --- Nightly aggregates (analytics:rollup job, TDD §4.2) ---------------------
-- `value` is the count for that (date, metric, dims). The rollup job INSERTs the
-- previous day's aggregates with ON CONFLICT … DO UPDATE SET value = excluded.value
-- (set, not increment) → re-running the job for a date is idempotent and corrects
-- the row rather than double-counting.
CREATE TABLE analytics_daily (
    id     BIGSERIAL PRIMARY KEY,
    date   DATE NOT NULL,
    metric VARCHAR(40) NOT NULL,                  -- pageviews | events | visitors
    dims   JSONB NOT NULL DEFAULT '{}'::jsonb,    -- {path|name, country, locale}
    value  BIGINT NOT NULL DEFAULT 0,
    UNIQUE (date, metric, dims)
);