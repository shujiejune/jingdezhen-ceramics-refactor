-- 20_artists.sql — 3 artists with i18n translations
-- =============================================================================
-- Parent rows (non-localized: avatar, display order) + per-locale translations
-- (TDD §3.2 pattern). en-US published for all three; zh-CN published for two
-- (to exercise locale filtering in reads).
--
-- Legacy name/bio columns are kept in sync so any code still reading them
-- (none after the gallery removal, but additive) sees consistent data.
-- =============================================================================

BEGIN;

INSERT INTO artists (id, name, bio, avatar_url, display_order) VALUES
    (1, 'Master Chen', 'A third-generation blue-and-white porcelain master from Jingdezhen.', 'https://picsum.photos/seed/artist-chen/400/400', 1),
    (2, 'Wang Mei', 'Contemporary ceramicist known for revived underglaze-red techniques.', 'https://picsum.photos/seed/artist-wang/400/400', 2),
    (3, 'Li Qing', 'Celadon specialist working with Song-dynasty glaze formulas.', 'https://picsum.photos/seed/artist-li/400/400', 3)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name, bio = EXCLUDED.bio, avatar_url = EXCLUDED.avatar_url, display_order = EXCLUDED.display_order;

SELECT setval(pg_get_serial_sequence('artists', 'id'),
              COALESCE((SELECT MAX(id) FROM artists), 0) + 1, false);

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

COMMIT;
