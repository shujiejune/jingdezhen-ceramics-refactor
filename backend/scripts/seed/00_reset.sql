-- 00_reset.sql — dev clean slate
-- =============================================================================
-- Truncates the commerce tables to guarantee a known state before seeding.
-- DEV ONLY — never run against production.
--
-- users is intentionally NOT truncated (preserves the admin test user). The
-- customer row is upserted by 10_users.sql.
--
-- Truncate order is child-first (dependency-safe); CASCADE covers any
-- referring tables not listed. RESTART IDENTITY resets sequences so explicit
-- IDs in the seed don't collide with auto-generated ones later.
-- =============================================================================

BEGIN;

TRUNCATE
    wishlists,
    skus,
    product_translations,
    products,
    artist_translations,
    artists
RESTART IDENTITY CASCADE;

COMMIT;
