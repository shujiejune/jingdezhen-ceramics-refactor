-- 40_skus.sql — 5 SKUs (one per product)
-- =============================================================================
-- The purchasable units. Referenced by wishlists, cart_items, order_items.
--
-- price_cny is in minor units (fen): ¥1280.00 = 128000 fen (TDD §7 — never
-- float). weight_grams is the packed weight (drives shipping calc, PRD §3.2.3).
-- attributes is JSONB with size/technique/glaze/edition metadata.
-- =============================================================================

BEGIN;

INSERT INTO skus (id, product_id, sku_code, price_cny, stock, weight_grams, low_stock_threshold, attributes, is_active) VALUES
    (1, 1, 'SKU-VASE-BW-01',  128000, 5,  1200, 2, '{"size":"H30cm","technique":"underglaze blue","glaze":"clear","edition_type":"limited"}'::jsonb, TRUE),
    (2, 2, 'SKU-TEA-CEL-01',   88000, 10, 900,  2, '{"size":"6-piece","technique":"celadon","glaze":"pale green","edition_type":"open"}'::jsonb,     TRUE),
    (3, 3, 'SKU-BOWL-UR-01',  240000, 1,  400,  2, '{"size":"D15cm","technique":"underglaze red","glaze":"clear","edition_type":"one_of_a_kind"}'::jsonb, TRUE),
    (4, 4, 'SKU-PLATE-FR-01', 360000, 3,  600,  2, '{"size":"D22cm","technique":"famille rose","glaze":"clear","edition_type":"limited"}'::jsonb,     TRUE),
    (5, 5, 'SKU-MUG-MOD-01',   32000, 20, 350,  2, '{"size":"300ml","technique":"wheel-thrown","glaze":"ash","edition_type":"open"}'::jsonb,         TRUE)
ON CONFLICT (sku_code) DO UPDATE SET
    product_id = EXCLUDED.product_id, price_cny = EXCLUDED.price_cny, stock = EXCLUDED.stock,
    weight_grams = EXCLUDED.weight_grams, attributes = EXCLUDED.attributes, is_active = EXCLUDED.is_active, updated_at = NOW();

SELECT setval(pg_get_serial_sequence('skus', 'id'),
              COALESCE((SELECT MAX(id) FROM skus), 0) + 1, false);

COMMIT;
