-- Reverse 000021_product_tags: drop the join + translations, restore the
-- baseline `tags(name)` shape. Does NOT drop `tags` (shared baseline infra).

DROP TABLE IF EXISTS product_tags;
DROP TABLE IF EXISTS tag_translations;

ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_key_format;
ALTER TABLE tags RENAME COLUMN key TO name;
