-- 75_option_rates.sql — mocked itinerary option rate table (PRD §3.3.2)
-- =============================================================================
-- PRD §3.3.2: "The quoted price is calculated from the selected options (guide,
-- hotel level, pickup, experience session, group size, duration), from an
-- operator-configured per-option rate table in the CMS (priced in CNY like
-- products). Real rates are not yet defined — development proceeds with mocked
-- values." These are illustrative fen amounts; real rates are CMS data later.
--
-- unit: per_person (× group size), per_day (× duration_days), flat (× 1).
-- option_key is canonical lowercase kebab (CHECK in the schema).
-- =============================================================================

BEGIN;

INSERT INTO option_rates (option_key, rate_cny, unit, display_label) VALUES
    -- Step-3 services (PRD §3.3.2). Rates are illustrative fen (per-unit CNY × 100).
    ('guide-english',      20000, 'per_person', 'English-speaking guide'),      -- ¥200/person
    ('guide-other',        30000, 'per_person', 'Other-language guide'),        -- ¥300/person
    ('hotel-budget',       15000, 'per_person', 'Budget hotel (per night)'),    -- ¥150/person/night
    ('hotel-comfort',      40000, 'per_person', 'Comfort hotel (per night)'),   -- ¥400/person/night
    ('hotel-luxury',       90000, 'per_person', 'Luxury hotel (per night)'),    -- ¥900/person/night
    ('pickup',             5000,  'flat',        'Airport/station pickup'),     -- ¥50 flat
    ('experience-session', 8000,  'per_person', 'Hands-on ceramic experience'),  -- ¥80/person
    ('base-itinerary',     1000,  'per_day',     'Base itinerary planning')     -- ¥10/day
ON CONFLICT (option_key) DO UPDATE SET
    rate_cny = EXCLUDED.rate_cny,
    unit = EXCLUDED.unit,
    display_label = EXCLUDED.display_label,
    updated_at = NOW();

COMMIT;
