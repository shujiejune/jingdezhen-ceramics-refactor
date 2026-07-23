-- 30_products.sql — 5 products with i18n translations
-- =============================================================================
-- Parent rows (artist link, category, thumbnail, display order) + per-locale
-- translations (title/slug/description + workflow status). en-US published for
-- all five; zh-CN published for two (exercises locale filtering in reads).
-- SKUs (the purchasable units) are seeded separately in 40_skus.sql.
-- =============================================================================

BEGIN;

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

COMMIT;
