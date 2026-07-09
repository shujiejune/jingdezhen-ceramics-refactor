-- =============================================================================
-- 000008_activity_translations: i18n for engage → Destinations & Local Lifestyle
-- (TDD §3.2, PRD §3.1.2 / §3.1.3)
--
-- Consolidates the inherited `activities` (card) + `articles` (detail) pair into
-- a single content entity `activities` with a per-locale `activity_translations`
-- table. PRD §3.1.3 says Local Lifestyle "reuses the article content model";
-- merging the article body onto the translation row removes the awkward
-- `activities.article_slug → articles.slug` link. The `type` column on the
-- parent distinguishes a Destination from a Local-Lifestyle entry.
--
-- Destinations gain the PRD §3.1.2 location fields (lat, lng, address,
-- opening_info) on the parent (non-localized; coordinates are locale-neutral).
-- Local-Lifestyle rows leave them NULL.
--
-- Existing data is backfilled into en-US published translations.
-- =============================================================================

-- Add destination location fields to the parent (PRD §3.1.2). Nullable: only
-- Destination-type rows use them; Local-Lifestyle rows leave them NULL.
ALTER TABLE activities
    ADD COLUMN IF NOT EXISTS lat          DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS lng          DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS address      TEXT,
    ADD COLUMN IF NOT EXISTS opening_info JSONB;

-- Per-locale translations (TDD §3.2 pattern).
CREATE TABLE activity_translations (
    id                 BIGSERIAL PRIMARY KEY,
    activity_id        BIGINT NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    locale             VARCHAR(10) NOT NULL,   -- BCP 47: en-US, zh-CN
    slug               VARCHAR(255) NOT NULL,
    title              VARCHAR(255) NOT NULL,
    brief_introduction TEXT,                    -- card summary
    body               TEXT,                     -- full article body (was articles.content)
    meta_title         VARCHAR(255),
    meta_description   TEXT,
    status             VARCHAR(20) NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','in_review','published','rejected')),
    reviewed_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (activity_id, locale),
    UNIQUE (locale, slug)
);

-- Hot-path indexes (TDD §11.1.3).
CREATE INDEX idx_act_trans_activity_locale ON activity_translations (activity_id, locale);
CREATE INDEX idx_act_trans_locale_slug      ON activity_translations (locale, slug);
CREATE INDEX idx_act_trans_locale_published ON activity_translations (locale) WHERE status = 'published';
-- Published listing filtered by parent type (Destinations vs Local Lifestyle).
CREATE INDEX idx_act_trans_published_join   ON activity_translations (activity_id) WHERE status = 'published';
CREATE INDEX idx_activities_type            ON activities (type);

-- Backfill: existing activities → en-US published translations. Pull the article
-- body from the linked articles row (via article_slug) so the consolidated body
-- is populated where the card had a linked article.
INSERT INTO activity_translations (
    activity_id, locale, slug, title, brief_introduction, body, status, published_at
)
SELECT
    a.id, 'en-US', a.article_slug, a.title, a.brief_introduction,
    (SELECT ar.content FROM articles ar WHERE ar.slug = a.article_slug LIMIT 1),
    'published', COALESCE(
        (SELECT ar.published_at FROM articles ar WHERE ar.slug = a.article_slug LIMIT 1),
        a.created_at
    )
FROM activities a;
