-- 50_wishlists.sql — customer favorites 3 SKUs
-- =============================================================================
-- The customer (00000000-...0002) favorites the vase (1), the one-of-a-kind
-- bowl (3), and the mug (5). Keyed on SKU (the purchasable unit), per PRD §3.5.
-- =============================================================================

BEGIN;

INSERT INTO wishlists (user_id, sku_id) VALUES
    ('00000000-0000-0000-0000-000000000002', 1),
    ('00000000-0000-0000-0000-000000000002', 3),
    ('00000000-0000-0000-0000-000000000002', 5)
ON CONFLICT (user_id, sku_id) DO NOTHING;

COMMIT;
