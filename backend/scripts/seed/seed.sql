-- =============================================================================
-- Dev seed data — Jingdezhen Ceramics Platform
-- =============================================================================
-- Run via:  make db-seed   (pipes this file into psql inside the db container)
--
-- This is a DEV seed: it TRUNCATES the commerce tables first to guarantee a
-- known state. Do NOT run against a production database.
--
-- Seeds:
--   • 1 normal customer (no user_roles row → customer, per RBAC design)
--   • 3 artists (en-US + zh-CN translations for 2 of them)
--   • 5 products across 4 categories (en-US translations, 2 with zh-CN)
--   • 5 SKUs (one per product) with real prices, stock, packed weight
--   • wishlist entries (customer favorites 3 SKUs)
--
-- Password for the customer is "password123" (bcrypt hash reused from the
-- existing admin test user for consistency).
-- =============================================================================

BEGIN;

-- --- Clean slate (dev only) --------------------------------------------------
-- Truncate in dependency-safe order. RESTART IDENTITY resets sequences.
-- users is NOT truncated (preserves the admin test user).
TRUNCATE
    wishlists,
    skus,
    product_translations,
    products,
    artist_translations,
    artists
RESTART IDENTITY CASCADE;

-- --- Customer ----------------------------------------------------------------
-- A normal customer: is_active=true, no user_roles row. Fixed UUID for
-- reproducible wishlist/orders later.
INSERT INTO users (id, nickname, email, password_hash, is_active, auth_provider, preferred_locale, preferred_currency, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'Test Customer',
    'customer@jingdezhen.test',
    '$2a$10$5Z9E1vghW0OM0xMakIMcoep1Uj2ll8QY5Dq5iQejdA09c5gfOQB3K', -- "password123"
    TRUE,
    'email',
    'en-US',
    'USD',
    NOW(), NOW()
)
ON CONFLICT (email) DO UPDATE SET
    nickname = EXCLUDED.nickname,
    password_hash = EXCLUDED.password_hash,
    is_active = EXCLUDED.is_active,
    preferred_locale = EXCLUDED.preferred_locale,
    preferred_currency = EXCLUDED.preferred_currency,
    updated_at = NOW();

-- --- Artists -----------------------------------------------------------------
-- Parent rows (non-localized: avatar, display order). The legacy name/bio
-- columns are kept in sync so any code still reading them (none after the
-- gallery removal, but additive) sees consistent data.
INSERT INTO artists (id, name, bio, avatar_url, display_order) VALUES
    (1, 'Master Chen', 'A third-generation blue-and-white porcelain master from Jingdezhen.', 'https://picsum.photos/seed/artist-chen/400/400', 1),
    (2, 'Wang Mei', 'Contemporary ceramicist known for revived underglaze-red techniques.', 'https://picsum.photos/seed/artist-wang/400/400', 2),
    (3, 'Li Qing', 'Celadon specialist working with Song-dynasty glaze formulas.', 'https://picsum.photos/seed/artist-li/400/400', 3)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name, bio = EXCLUDED.bio, avatar_url = EXCLUDED.avatar_url, display_order = EXCLUDED.display_order;

SELECT setval(pg_get_serial_sequence('artists', 'id'),
              COALESCE((SELECT MAX(id) FROM artists), 0) + 1, false);

-- Artist translations — en-US (all three, published) + zh-CN (two, published).
INSERT INTO artist_translations (artist_id, locale, name, slug, bio, status, published_at) VALUES
    (1, 'en-US', 'Master Chen',          'master-chen',          'A third-generation blue-and-white porcelain master from Jingdezhen.',                              'published', NOW()),
    (2, 'en-US', 'Wang Mei',              'wang-mei',             'Contemporary ceramicist known for revived underglaze-red techniques.',                            'published', NOW()),
    (3, 'en-US', 'Li Qing',               'li-qing',              'Celadon specialist working with Song-dynasty glaze formulas.',                                    'published', NOW()),
    (1, 'zh-CN', '陈大师',                'chen-dashi',           '来自景德镇的第三代青花瓷器大师。',                                                              'published', NOW()),
    (2, 'zh-CN', '王梅',                  'wang-mei-zh',         '以复烧釉里红技法著称的当代陶瓷艺术家。',                                                          'published', NOW())
ON CONFLICT (artist_id, locale) DO UPDATE SET
    name = EXCLUDED.name, slug = EXCLUDED.slug, bio = EXCLUDED.bio, status = EXCLUDED.status, published_at = EXCLUDED.published_at, updated_at = NOW();

SELECT setval(pg_get_serial_sequence('artist_translations', 'id'),
              COALESCE((SELECT MAX(id) FROM artist_translations), 0) + 1, false);

-- --- Products ----------------------------------------------------------------
INSERT INTO products (id, artist_id, category, thumbnail_url, display_order) VALUES
    (1, 1, 'blue and white',  'https://picsum.photos/seed/vase-bw/600/600',     1),
    (2, 3, 'celadon',         'https://picsum.photos/seed/teaset-cel/600/600', 2),
    (3, 2, 'underglaze red',  'https://picsum.photos/seed/bowl-ur/600/600',     3),
    (4, 1, 'famille rose',    'https://picsum.photos/seed/plate-fr/600/600',    4),
    (5, 2, 'contemporary',    'https://picsum.photos/seed/mug-modern/600/600',  5)
ON CONFLICT (id) DO UPDATE SET
    artist_id = EXCLUDED.artist_id, category = EXCLUDED.category,
    thumbnail_url = EXCLUDED.thumbnail_url, display_order = EXCLUDED.display_order;

SELECT setval(pg_get_serial_sequence('products', 'id'),
              COALESCE((SELECT MAX(id) FROM products), 0) + 1, false);

-- Product translations — en-US (all five, published) + zh-CN (two, published).
INSERT INTO product_translations (product_id, locale, title, slug, description, status, published_at) VALUES
    (1, 'en-US', 'Blue & White Vase',      'blue-and-white-vase',      'A classic Meiping-form vase painted with cobalt-blue landscapes.',                                 'published', NOW()),
    (2, 'en-US', 'Celadon Tea Set',        'celadon-tea-set',          'A six-piece tea set in pale green celadon glaze, fired in reduction.',                            'published', NOW()),
    (3, 'en-US', 'Underglaze Red Bowl',    'underglaze-red-bowl',      'A one-of-a-kind bowl featuring the demanding copper-red underglaze technique.',                   'published', NOW()),
    (4, 'en-US', 'Famille Rose Plate',     'famille-rose-plate',       'An octagonal plate decorated in the famille-rose palette of the Qing court.',                     'published', NOW()),
    (5, 'en-US', 'Modern Studio Mug',      'modern-studio-mug',        'A wheel-thrown studio mug with a matte ash glaze, made for daily use.',                          'published', NOW()),
    (1, 'zh-CN', '青花梅瓶',               'qinghua-meiping',          '经典梅瓶造型，以钴蓝料绘山水图景。',                                                              'published', NOW()),
    (3, 'zh-CN', '釉里红碗',               'youlihong-wan',           '孤品。采用极具挑战的铜红釉下彩工艺。',                                                            'published', NOW())
ON CONFLICT (product_id, locale) DO UPDATE SET
    title = EXCLUDED.title, slug = EXCLUDED.slug, description = EXCLUDED.description,
    status = EXCLUDED.status, published_at = EXCLUDED.published_at, updated_at = NOW();

SELECT setval(pg_get_serial_sequence('product_translations', 'id'),
              COALESCE((SELECT MAX(id) FROM product_translations), 0) + 1, false);

-- --- SKUs (purchasable units) ------------------------------------------------
-- price_cny is in minor units (fen): ¥1280.00 = 128000 fen.
-- weight_grams is the packed weight (drives shipping calc, PRD §3.2.3).
-- attributes: JSONB with size/technique/glaze/edition metadata.
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

-- --- Wishlist ----------------------------------------------------------------
-- The customer favorites 3 SKUs (the vase, the one-of-a-kind bowl, the mug).
INSERT INTO wishlists (user_id, sku_id) VALUES
    ('00000000-0000-0000-0000-000000000002', 1),
    ('00000000-0000-0000-0000-000000000002', 3),
    ('00000000-0000-0000-0000-000000000002', 5)
ON CONFLICT (user_id, sku_id) DO NOTHING;

COMMIT;

-- --- Summary -----------------------------------------------------------------
SELECT 'users' AS table_name, count(*) FROM users WHERE email = 'customer@jingdezhen.test'
UNION ALL SELECT 'artists',            count(*) FROM artists
UNION ALL SELECT 'artist_translations',count(*) FROM artist_translations
UNION ALL SELECT 'products',           count(*) FROM products
UNION ALL SELECT 'product_translations',count(*) FROM product_translations
UNION ALL SELECT 'skus',               count(*) FROM skus
UNION ALL SELECT 'wishlists',          count(*) FROM wishlists;
