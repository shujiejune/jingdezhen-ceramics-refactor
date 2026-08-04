-- 99_summary.sql — row counts for confirmation (no data changes)
-- =============================================================================
-- Printed after seeding so you can see at a glance that everything landed.
-- Add new tables here as the seed grows.

SELECT 'users'                  AS table_name, count(*) FROM users WHERE email = 'customer@jingdezhen.test'
UNION ALL SELECT 'artists',              count(*) FROM artists
UNION ALL SELECT 'artist_translations',  count(*) FROM artist_translations
UNION ALL SELECT 'products',             count(*) FROM products
UNION ALL SELECT 'product_translations', count(*) FROM product_translations
UNION ALL SELECT 'skus',                 count(*) FROM skus
UNION ALL SELECT 'wishlists',            count(*) FROM wishlists
UNION ALL SELECT 'carts',                count(*) FROM carts
UNION ALL SELECT 'cart_items',           count(*) FROM cart_items
UNION ALL SELECT 'shipping_fee_tiers',   count(*) FROM shipping_fee_tiers
UNION ALL SELECT 'orders',               count(*) FROM orders
UNION ALL SELECT 'order_items',           count(*) FROM order_items
UNION ALL SELECT 'payments',              count(*) FROM payments
UNION ALL SELECT 'certificates',          count(*) FROM certificates
UNION ALL SELECT 'provenance_records',    count(*) FROM provenance_records
UNION ALL SELECT 'media_assets',          count(*) FROM media_assets
UNION ALL SELECT 'product_media',          count(*) FROM product_media
UNION ALL SELECT 'itinerary_requests',      count(*) FROM itinerary_requests
UNION ALL SELECT 'itinerary_drafts',        count(*) FROM itinerary_drafts
UNION ALL SELECT 'crm_notes',               count(*) FROM crm_notes;
