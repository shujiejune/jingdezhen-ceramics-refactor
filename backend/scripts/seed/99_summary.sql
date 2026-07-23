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
UNION ALL SELECT 'wishlists',            count(*) FROM wishlists;
