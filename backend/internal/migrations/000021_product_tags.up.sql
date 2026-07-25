-- =============================================================================
-- 000021_product_tags: translatable tags for products (TDD §3.2, PRD §3.2.1)
--
-- PRD §3.2.1 line 173: "Category/tag browsing, filtering, sorting, and keyword
-- search." TDD §3.2 line 130: `tags(id) + tag_translations(name)`.
--
-- Evolves the dead `tags(id, name)` table from the baseline into a proper i18n
-- taxonomy: a parent keyed on a language-neutral canonical `key` (lowercase
-- kebab-case, e.g. `hand-painted`) + per-locale display names in
-- `tag_translations` + a `product_tags` join.
--
-- Tags are taxonomy, NOT editorial content: no draft/publish workflow lives on
-- tag_translations (unlike artist/product translations). A tag is visible iff
-- it is attached to a published product. Public reads JOIN tag_translations for
-- the requested locale with an en-US → key fallback (COALESCE in the query).
--
-- No backfill: the inherited `tags` table is empty.
-- =============================================================================

-- --- Evolve the parent: name → key (language-neutral canonical identifier) --
-- The CHECK enforces lowercase kebab-case (mirrors the slug convention) so tags
-- are stable references across locales + CSV imports.
ALTER TABLE tags RENAME COLUMN name TO key;
ALTER TABLE tags ADD CONSTRAINT tags_key_format CHECK (key ~ '^[a-z0-9][a-z0-9-]{0,49}$');

-- --- Per-locale display names (TDD §3.2 pattern: parent + *_translations) ---
CREATE TABLE tag_translations (
    id        BIGSERIAL PRIMARY KEY,
    tag_id    BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    locale    VARCHAR(10) NOT NULL,   -- BCP 47: en-US, zh-CN
    name      VARCHAR(100) NOT NULL,  -- localized display name
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tag_id, locale)
);

CREATE INDEX idx_tag_trans_tag_locale ON tag_translations (tag_id, locale);

-- --- Product ↔ tag join (set-replace assignment from the CMS) ---------------
CREATE TABLE product_tags (
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    tag_id     BIGINT NOT NULL REFERENCES tags(id)     ON DELETE CASCADE,
    PRIMARY KEY (product_id, tag_id)
);

-- Supports "list all tags" + "filter by tag" without scanning products.
CREATE INDEX idx_product_tags_tag_id ON product_tags (tag_id);
