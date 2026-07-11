-- =============================================================================
-- 000011_artist_translations: i18n translation table for artist profiles
-- (TDD §3.2, PRD §3.1.3 / §3.2.1)
--
-- Splits the monolingual artists table into a parent (non-localized data:
-- avatar/portrait photo, user_id link, display order) + a per-locale
-- translation table (name, slug, bio, meta + an independent workflow status,
-- PRD §3.1.1). Existing data is backfilled into en-US published translations
-- so cross-links from Art Gallery products keep resolving.
--
-- Per AGENTS.md rule 4, old columns (name, bio) are NOT dropped here — they
-- become dead (the new artist module reads only from translations). The gallery
-- module's artwork→artist JOIN still reads artists.name (the old column) as a
-- fallback until M2 evolves the gallery into the product catalog. A future
-- cleanup migration can drop them then.
-- =============================================================================

-- The parent already has created_at/updated_at from the baseline. Add the
-- non-localized display fields the PRD calls for.
ALTER TABLE artists
    ADD COLUMN IF NOT EXISTS avatar_url TEXT,
    ADD COLUMN IF NOT EXISTS display_order INT NOT NULL DEFAULT 0;

-- Per-locale translations (TDD §3.2 pattern).
CREATE TABLE artist_translations (
    id                BIGSERIAL PRIMARY KEY,
    artist_id         BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    locale            VARCHAR(10) NOT NULL,   -- BCP 47: en-US, zh-CN
    name              VARCHAR(255) NOT NULL,  -- localized display name
    slug              VARCHAR(100) NOT NULL,
    bio               TEXT,                   -- localized artist biography
    meta_title        VARCHAR(255),
    meta_description  TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft','in_review','published','rejected')),
    reviewed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (artist_id, locale),
    UNIQUE (locale, slug)
);

-- Hot-path indexes (TDD §11.1.3): (entity_id, locale) and (locale, slug).
CREATE INDEX idx_artist_trans_artist_locale ON artist_translations (artist_id, locale);
CREATE INDEX idx_artist_trans_locale_slug    ON artist_translations (locale, slug);
-- Published-only listing/detail (the public read path).
CREATE INDEX idx_artist_trans_locale_published ON artist_translations (locale) WHERE status = 'published';

-- Backfill: existing artists → en-US published translations. The old name/bio
-- columns are the source of truth here; COALESCE guards a NULL bio.
INSERT INTO artist_translations (artist_id, locale, name, slug, bio, status, published_at)
SELECT
    id, 'en-US', name,
    -- Derive a slug from the name (lowercase, spaces→dashes). If the name
    -- collides the UNIQUE(locale, slug) constraint will surface it; for the
    -- typical empty-dev-DB case there is no collision.
    LOWER(REPLACE(TRIM(name), ' ', '-')),
    COALESCE(bio, NULL),
    'published', NOW()
FROM artists
WHERE name IS NOT NULL AND name <> '';
