-- =============================================================================
-- 000007_ceramic_story_translations: i18n translation tables for ceramic_stories
-- (TDD §3.2, PRD §3.5.1)
--
-- Splits the monolingual ceramic_stories table into a parent (non-localized
-- data) + a per-locale translation table (title/slug/content/meta + an
-- independent workflow status, PRD §3.1.1). Existing data is backfilled into
-- en-US published translations so the public timeline keeps working.
--
-- Per AGENTS.md rule 4, old columns are NOT dropped here — they become dead
-- (the new code reads only from translations). A future cleanup migration can
-- drop dynasty_name, slug, period, description, characteristics_craft,
-- characteristics_art, takeaways from the parent.
-- =============================================================================

-- Add missing timestamps to the parent.
ALTER TABLE ceramic_stories
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Per-locale translations (TDD §3.2 pattern).
CREATE TABLE ceramic_story_translations (
    id                    BIGSERIAL PRIMARY KEY,
    story_id              BIGINT NOT NULL REFERENCES ceramic_stories(id) ON DELETE CASCADE,
    locale                VARCHAR(10) NOT NULL,   -- BCP 47: en-US, zh-CN (later zh-Hant, ja, fr)
    dynasty_name          VARCHAR(100) NOT NULL,   -- localized display name (Ming Dynasty / 明朝)
    slug                  VARCHAR(100) NOT NULL,
    period                VARCHAR(100),            -- localized period label
    description           TEXT NOT NULL,
    characteristics_craft TEXT,
    characteristics_art   TEXT,
    takeaways             TEXT,
    meta_title            VARCHAR(255),
    meta_description      TEXT,
    status                VARCHAR(20) NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft','in_review','published','rejected')),
    reviewed_by           UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (story_id, locale),
    UNIQUE (locale, slug)
);

-- Hot-path indexes (TDD §11.1.3): (entity_id, locale) and (locale, slug).
CREATE INDEX idx_cs_trans_story_locale ON ceramic_story_translations (story_id, locale);
CREATE INDEX idx_cs_trans_locale_slug  ON ceramic_story_translations (locale, slug);
-- Published-only listing/detail (the public read path). display_order lives on
-- the parent ceramic_stories, so the service joins parent for ordering; this
-- index covers the common "all published for a locale" scan.
CREATE INDEX idx_cs_trans_locale_published ON ceramic_story_translations (locale) WHERE status = 'published';

-- Backfill: existing content → en-US published translations so the timeline
-- stays live after the switch. The old columns are the source of truth here.
INSERT INTO ceramic_story_translations (
    story_id, locale, dynasty_name, slug, period, description,
    characteristics_craft, characteristics_art, takeaways, status, published_at
)
SELECT
    id, 'en-US', dynasty_name, slug, period, description,
    characteristics_craft, characteristics_art, takeaways, 'published', NOW()
FROM ceramic_stories;
