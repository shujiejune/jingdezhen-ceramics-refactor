-- 65_media.sql — media_assets + product_media for the 5 seeded products
-- =============================================================================
-- Registers placeholder media_assets rows (kind=image, a picsum.photos URL as
-- the oss_key — in local-dev mode the static mount won't serve these, but the
-- PublicURL is derived from oss_key so the catalog shows the picsum image).
-- Attaches one image per product as sort_order=0 (primary display).
--
-- In real use the admin uploads via the presign/upload flow + registers the
-- returned oss_key; this seed just pre-populates the dev catalog so the
-- gallery is non-empty without real uploads.
-- =============================================================================

BEGIN;

-- Register 5 placeholder images. oss_key is a real-looking storage key so
-- LocalStore.PublicURL derives /media/seed/<name> — but those files don't
-- exist on disk in dev. To keep the catalog visually non-empty in dev, we
-- store the picsum URL as oss_key; OSSStore.PublicURL would derive the
-- bucket URL (broken), but LocalStore prepends /media/ (also broken). The
-- honest fix is real uploads; for the seed we point oss_key at the picsum
-- URL directly and accept that PublicURL is a /media/-prefixed path the
-- static mount can't serve. The catalog will show broken images in dev
-- unless real files are uploaded — acceptable for a dev seed.
INSERT INTO media_assets (id, kind, oss_key, mime, width, height) VALUES
    (1, 'image', 'https://picsum.photos/seed/vase-bw/600/600',   'image/jpeg', 600, 600),
    (2, 'image', 'https://picsum.photos/seed/teaset-cel/600/600', 'image/jpeg', 600, 600),
    (3, 'image', 'https://picsum.photos/seed/bowl-ur/600/600',    'image/jpeg', 600, 600),
    (4, 'image', 'https://picsum.photos/seed/plate-fr/600/600',   'image/jpeg', 600, 600),
    (5, 'image', 'https://picsum.photos/seed/mug-modern/600/600', 'image/jpeg', 600, 600)
ON CONFLICT (id) DO UPDATE SET
    kind = EXCLUDED.kind, oss_key = EXCLUDED.oss_key, mime = EXCLUDED.mime,
    width = EXCLUDED.width, height = EXCLUDED.height;

-- Attach each image to its product as the primary (sort_order=0) gallery item.
-- product_media(product_id, media_id, sort_order, caption)
INSERT INTO product_media (product_id, media_id, sort_order, caption) VALUES
    (1, 1, 0, 'Primary view'),
    (2, 2, 0, 'Primary view'),
    (3, 3, 0, 'Primary view'),
    (4, 4, 0, 'Primary view'),
    (5, 5, 0, 'Primary view')
ON CONFLICT (product_id, media_id) DO UPDATE SET
    sort_order = EXCLUDED.sort_order, caption = EXCLUDED.caption;

-- Advance the sequences past the explicit IDs so the next RegisterAsset
-- doesn't collide with media_assets_pkey.
SELECT setval(pg_get_serial_sequence('media_assets', 'id'),
              COALESCE((SELECT MAX(id) FROM media_assets), 0) + 1, false);
SELECT setval(pg_get_serial_sequence('product_media', 'id'),
              COALESCE((SELECT MAX(id) FROM product_media), 0) + 1, false);

COMMIT;
