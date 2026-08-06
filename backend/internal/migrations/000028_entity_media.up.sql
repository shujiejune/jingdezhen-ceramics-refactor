-- 000028_entity_media: ordered media galleries for artists, ceramic stories,
-- and activities — mirroring product_media (migration 000020). Each is a M:N
-- join table between the content entity and the central media_assets registry,
-- with a sort_order column (0 = primary display image) so the CMS can
-- drag-and-drop to control the gallery sequence.
--
-- Mechanical follow-up to product_media (TDD §3.4 line 143). Same shape; only
-- the FK column + parent table differ. CASCADE on delete so removing a media
-- asset or its parent entity cleans up the gallery automatically.
-- =============================================================================

-- --- Artist profile gallery (PRD §3.1.3) ------------------------------------
CREATE TABLE artist_media (
    id          BIGSERIAL PRIMARY KEY,
    artist_id   BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    media_id    BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order  INT NOT NULL DEFAULT 0,       -- 0 = primary portrait
    caption     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (artist_id, media_id)              -- a media asset appears once per artist
);
CREATE INDEX idx_artist_media_artist_order ON artist_media (artist_id, sort_order);

-- --- Ceramic story gallery (History & Heritage, PRD §3.1.2) -----------------
CREATE TABLE ceramic_story_media (
    id          BIGSERIAL PRIMARY KEY,
    story_id    BIGINT NOT NULL REFERENCES ceramic_stories(id) ON DELETE CASCADE,
    media_id    BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order  INT NOT NULL DEFAULT 0,       -- 0 = primary image
    caption     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (story_id, media_id)
);
CREATE INDEX idx_ceramic_story_media_story_order ON ceramic_story_media (story_id, sort_order);

-- --- Activity gallery (Destinations & Local Lifestyle, PRD §3.1.2/§3.1.3) ---
CREATE TABLE activity_media (
    id          BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    media_id    BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order  INT NOT NULL DEFAULT 0,       -- 0 = primary image
    caption     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (activity_id, media_id)
);
CREATE INDEX idx_activity_media_activity_order ON activity_media (activity_id, sort_order);
