-- =============================================================================
-- 000013_evolve_artworks_drop: drop the legacy artworks stack (TDD §3.4)
--
-- The artworks data was backfilled into products + product_translations + a
-- default SKU per product by migration 000012. The gallery module that read
-- artworks is deleted in this step; products is now the sole catalog read path.
--
-- This migration:
--   1. Migrates user_favorite_artworks → wishlists(user_id, sku_id, created_at)
--      by joining artwork_id → the default SKU created in 000012 (sku_code =
--      'SKU-' || artwork_id).
--   2. Drops the legacy tables in FK-dependency order.
--
-- Data loss (acceptable for dev DB, pre-prod):
--   - artwork_images rows (bare URL strings, not OSS-backed; multi-image
--     product galleries come with the media_assets infra, M1 TODO).
--   - artwork_tags junction rows (tags taxonomy is a separate M1 TODO;
--     products don't yet have a product_tags table).
-- =============================================================================

-- --- Wishlists (PRD §3.5, TDD §3.4) -------------------------------------------
-- Favorites are now keyed on SKU (the purchasable unit), not artwork/product.
-- One wishlist row per (user, sku).

CREATE TABLE wishlists (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sku_id     BIGINT NOT NULL REFERENCES skus(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, sku_id)
);

CREATE INDEX idx_wishlists_user_id ON wishlists (user_id);

-- Migrate user_favorite_artworks → wishlists via the default-SKU mapping.
-- Each artwork_id maps to exactly one SKU with sku_code = 'SKU-' || artwork_id
-- (created by the 000012 backfill). Favorites whose artwork has no
-- corresponding product/SKU are dropped (shouldn't happen on a clean DB, but
-- the WHERE guards against stale favorites if the backfill was partial).
INSERT INTO wishlists (user_id, sku_id, created_at)
SELECT ufa.user_id, s.id, ufa.created_at
FROM user_favorite_artworks ufa
JOIN skus s ON s.sku_code = 'SKU-' || ufa.artwork_id::text
ON CONFLICT (user_id, sku_id) DO NOTHING;

-- --- Drop legacy tables (FK-dependency order) --------------------------------

DROP TABLE IF EXISTS user_favorite_artworks CASCADE;
DROP TABLE IF EXISTS artwork_tags CASCADE;
DROP TABLE IF EXISTS artwork_images CASCADE;
DROP TABLE IF EXISTS artworks CASCADE;
