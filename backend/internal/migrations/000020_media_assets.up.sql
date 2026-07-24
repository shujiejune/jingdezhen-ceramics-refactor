-- =============================================================================
-- 000020_media_assets: central media registry + ordered product gallery
-- (TDD §3.4 line 127/143, PRD §3.2.1)
--
-- Replaces the scattered free-text URL columns (products.thumbnail_url,
-- artists.avatar_url, ceramic_stories.image_url) with one central media_assets
-- table (TDD line 127) + per-entity ordered-gallery join tables (TDD line 132:
-- *_media(ordered images/videos)). Media never flows through the VPS: the admin
-- browser uploads directly to OSS via a presigned URL; the API only signs +
-- records the resulting oss_key (TDD §2.1, §4.1). The CDN URL is derived from
-- oss_key at read time, so a bucket/CDN-domain change is a config edit, not a
-- data migration.
--
-- Scope of THIS migration:
--   - media_assets (central registry, image+video)
--   - product_media (ordered gallery for products)
-- Video transcode (media:transcode job, TDD line 230) is registered as a job
-- type but the handler stays no-op for now (FFmpeg unavailable in the dev
-- container). artist_media / ceramic_story_media / activity_media are
-- mechanical follow-ups once the pattern lands.
--
-- The old products.thumbnail_url column is kept DEAD (not read/written by the
-- product module after this; the module prefers the first product_media item,
-- falling back to thumbnail_url for back-compat). Dropped in a later cleanup
-- migration per AGENTS.md rule 4.
-- =============================================================================

-- --- Central media registry (TDD §3.4 line 127) ------------------------------

CREATE TABLE media_assets (
    id           BIGSERIAL PRIMARY KEY,
    kind         VARCHAR(10) NOT NULL CHECK (kind IN ('image','video')),
    oss_key      TEXT UNIQUE NOT NULL,        -- stable storage key; CDN URL derived at read
    mime         VARCHAR(100) NOT NULL,       -- e.g. image/jpeg, video/mp4
    width        INT,                         -- px (images); videos: frame width
    height       INT,                         -- px
    duration     INT,                         -- seconds (videos only; NULL for images)
    hls_key      TEXT,                        -- transcoded HLS playlist key (videos; NULL until transcode)
    uploaded_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_assets_kind       ON media_assets (kind);
CREATE INDEX idx_media_assets_uploaded   ON media_assets (uploaded_by);

-- --- Ordered product gallery (TDD §3.4 line 143) ------------------------------

CREATE TABLE product_media (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    media_id    BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order  INT NOT NULL DEFAULT 0,       -- 0 = primary display image
    caption     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, media_id)            -- a media asset appears once per product
);

CREATE INDEX idx_product_media_product_order ON product_media (product_id, sort_order);
