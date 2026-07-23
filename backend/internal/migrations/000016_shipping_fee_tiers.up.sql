-- =============================================================================
-- 000016_shipping_fee_tiers: per-country weight-tiered shipping fee table
-- (PRD §3.2.3, TDD §3.7/§3.4)
--
-- E-commerce Operators maintain a fee table in the CMS keyed by destination
-- country; weight tiers are defined independently per country (each country
-- has its own tier boundaries + prices). All fees are CNY minor units (fen).
--
-- Shipping calc (TDD §7): fee = tier(country, ceil(Σ item.weight_grams * qty)).
-- No tier whose max_weight >= order weight → overweight block. No tiers for a
-- country → unshippable block. The fee is converted to presentment currency
-- with the same FX+rounding path at checkout (TDD §7).
-- =============================================================================

CREATE TABLE shipping_fee_tiers (
    id                BIGSERIAL PRIMARY KEY,
    country           CHAR(2) NOT NULL,                        -- ISO 3166-1 alpha-2
    max_weight_grams  INT NOT NULL CHECK (max_weight_grams > 0),
    fee_cny           BIGINT NOT NULL CHECK (fee_cny >= 0),    -- minor units (fen)
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (country, max_weight_grams)
);

CREATE INDEX idx_shipping_fee_tiers_country ON shipping_fee_tiers (country);

-- --- Seed: US / GB / DE / CN tiers (demo data, editable in CMS) ---------------
-- Fees in CNY fen (¥120.00 = 12000). Boundaries are inclusive ceilings: a
-- 1000g order matches the ≤1000g tier; a 1001g order matches the next tier.
INSERT INTO shipping_fee_tiers (country, max_weight_grams, fee_cny) VALUES
    -- United States: 3 tiers up to 5kg
    ('US', 1000, 12000),
    ('US', 3000, 22000),
    ('US', 5000, 35000),
    -- United Kingdom: 3 tiers up to 5kg
    ('GB', 1000, 15000),
    ('GB', 3000, 28000),
    ('GB', 5000, 45000),
    -- Germany (EU): 3 tiers up to 5kg
    ('DE', 1000, 15000),
    ('DE', 3000, 28000),
    ('DE', 5000, 45000),
    -- China mainland: 4 tiers up to 10kg
    ('CN', 1000,  8000),
    ('CN', 3000, 15000),
    ('CN', 5000, 25000),
    ('CN', 10000, 40000);
