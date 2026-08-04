-- =============================================================================
-- 000024_itinerary_quotes: quote builder + deposit payment (PRD §3.3.2, TDD §3.4 M3 #3)
--
-- The planner builds a quote from the mocked option_rates CMS table (CNY line
-- items → presentment via the FX pipeline + 30% deposit). The customer pays
-- the deposit through the SAME payment stack as e-commerce (TDD §3.4: "Itinerary
-- deposits reuse payments with order_id NULL + itinerary_quote_id").
--
-- Three changes:
--   option_rates        — the mocked CMS rate table (guide/hotel/pickup/etc).
--   itinerary_quotes    — one active quote per request (UNIQUE request_id;
--                         re-quote replaces). Carries line_items (CNY JSONB) +
--                         presentment totals + deposit_minor + a quote status
--                         state machine (sent→deposit_paid|fully_paid|cancelled)
--                         for refund idempotency + audit.
--   payments (evolve)   — order_id DROP NOT NULL + ADD itinerary_quote_id +
--                         exactly-one CHECK, so a payment row links to EITHER
--                         an order OR an itinerary quote (never both, never
--                         neither). Existing order rows keep order_id set.
-- =============================================================================

-- --- option_rates: mocked per-option rate table (PRD §3.3.2 "operator-
-- configured per-option rate table in the CMS, priced in CNY like products.
-- Real rates are not yet defined — development proceeds with mocked values").
-- option_key is the canonical identifier (lowercase kebab, like tag keys).
CREATE TABLE option_rates (
    id            BIGSERIAL PRIMARY KEY,
    option_key    VARCHAR(60) NOT NULL UNIQUE CHECK (option_key ~ '^[a-z0-9][a-z0-9_-]*$'),
    rate_cny      BIGINT NOT NULL CHECK (rate_cny >= 0),  -- fen (TDD §7: BIGINT minor units)
    unit          VARCHAR(20) NOT NULL CHECK (unit IN ('per_person','per_day','flat')),
    display_label VARCHAR(120),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- --- itinerary_quotes: one active quote per request. UNIQUE(request_id) so a
-- re-quote replaces the prior quote (the planner iterates; only the latest is
-- payable). line_items is the immutable CNY breakdown (option_key, qty, rate,
-- unit, label, line_cny); total_cny is Σ line_cny; total_minor/deposit_minor
-- are the presentment snapshot (fx_rate_used frozen at send time, like orders).
CREATE TABLE itinerary_quotes (
    id            BIGSERIAL PRIMARY KEY,
    request_id    BIGINT NOT NULL REFERENCES itinerary_requests(id) ON DELETE CASCADE,
    line_items    JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [{option_key,qty,rate_cny,unit,label,line_cny}]
    total_cny     BIGINT NOT NULL,                     -- Σ line_cny (fen)
    currency      CHAR(3) NOT NULL,                    -- presentment (USD/EUR/GBP)
    total_minor   BIGINT NOT NULL,                     -- presentment total (fx-frozen)
    deposit_minor BIGINT NOT NULL,                     -- round(total_minor * 0.30) OR total_minor if pay_full
    fx_rate_used  NUMERIC(20,8) NOT NULL,             -- snapshot at send (matches orders)
    status        VARCHAR(20) NOT NULL DEFAULT 'sent'
                  CHECK (status IN ('sent','deposit_paid','fully_paid','cancelled')),
    sent_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One active quote per request (re-quote replaces via ON CONFLICT).
CREATE UNIQUE INDEX idx_itin_quotes_request_id ON itinerary_quotes (request_id);
CREATE INDEX idx_itin_quotes_status ON itinerary_quotes (status);

-- --- payments (evolve): reuse for itinerary deposits (TDD §3.4 line 189).
-- order_id was NOT NULL; make it nullable so a deposit payment can carry
-- itinerary_quote_id instead. Add a CHECK so a payment links to EXACTLY ONE
-- of {order, itinerary_quote} (never both, never neither).
ALTER TABLE payments ALTER COLUMN order_id DROP NOT NULL;
ALTER TABLE payments ADD COLUMN itinerary_quote_id BIGINT REFERENCES itinerary_quotes(id) ON DELETE SET NULL;
ALTER TABLE payments ADD CONSTRAINT payments_exactly_one_owner
    CHECK ((order_id IS NULL) <> (itinerary_quote_id IS NULL));
CREATE INDEX idx_payments_itin_quote_id ON payments (itinerary_quote_id);
