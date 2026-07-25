-- 35_product_tags.sql — translatable tags for the 5 seeded products
-- =============================================================================
-- PRD §3.2.1 line 173: "Category/tag browsing, filtering, sorting, and keyword
-- search." TDD §3.2 line 130: `tags(id) + tag_translations(name)`.
--
-- Six language-neutral canonical keys (lowercase kebab-case) + en-US + zh-CN
-- display names, attached to the 5 seeded products. Idempotent (ON CONFLICT).
-- =============================================================================

BEGIN;

-- --- Parent rows (canonical keys) --------------------------------------------
INSERT INTO tags (id, key) VALUES
    (1, 'hand-painted'),
    (2, 'cobalt-blue'),
    (3, 'celadon-glaze'),
    (4, 'one-of-a-kind'),
    (5, 'limited-edition'),
    (6, 'studio-piece')
ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key;

SELECT setval(pg_get_serial_sequence('tags', 'id'),
              COALESCE((SELECT MAX(id) FROM tags), 0) + 1, false);

-- --- en-US + zh-CN display names ---------------------------------------------
INSERT INTO tag_translations (tag_id, locale, name) VALUES
    (1, 'en-US', 'Hand-painted'),    (1, 'zh-CN', '手绘'),
    (2, 'en-US', 'Cobalt blue'),     (2, 'zh-CN', '青花'),
    (3, 'en-US', 'Celadon glaze'),   (3, 'zh-CN', '青瓷釉'),
    (4, 'en-US', 'One of a kind'),   (4, 'zh-CN', '孤品'),
    (5, 'en-US', 'Limited edition'), (5, 'zh-CN', '限量版'),
    (6, 'en-US', 'Studio piece'),    (6, 'zh-CN', '工作室作品')
ON CONFLICT (tag_id, locale) DO UPDATE SET name = EXCLUDED.name;

-- --- Product ↔ tag attachments -----------------------------------------------
-- product 1 (Blue & White Vase):        hand-painted, cobalt-blue, one-of-a-kind
-- product 2 (Celadon Tea Set):          celadon-glaze, limited-edition
-- product 3 (Underglaze Red Bowl):      hand-painted, one-of-a-kind
-- product 4 (Famille Rose Plate):       hand-painted, limited-edition
-- product 5 (Modern Studio Mug):        studio-piece
INSERT INTO product_tags (product_id, tag_id) VALUES
    (1, 1), (1, 2), (1, 4),
    (2, 3), (2, 5),
    (3, 1), (3, 4),
    (4, 1), (4, 5),
    (5, 6)
ON CONFLICT DO NOTHING;

COMMIT;
