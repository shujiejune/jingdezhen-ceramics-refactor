-- =============================================================================
-- 000001_baseline: squashed schema baseline for the Jingdezhen Ceramics Platform
--
-- Consolidates the former migrations 000002, 000004-000011, 000034, 000037
-- (learning-platform remnants removed). Fixes applied during the squash:
--   * table `events` renamed to `activities` (code queries `activities`)
--   * `ceramic_stories.slug` added (model/repository select & filter by slug)
--   * tables ordered so all FK targets exist (artworks before artwork_images)
--
-- This baseline matches the CURRENT code. PRD-driven schema changes (RBAC,
-- addresses, SKUs, i18n translation tables, ...) come as new migrations on top.
-- See docs/REFACTOR-TODO.md.
-- =============================================================================

-- --- Users -------------------------------------------------------------------

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname VARCHAR(100),
    email VARCHAR(255) UNIQUE,          -- Can be null if using other auth methods primarily
    password_hash VARCHAR(255),         -- If handling email/password directly

    -- Email activation and password reset
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    activation_token TEXT,
    activation_token_expires_at TIMESTAMPTZ,
    password_reset_token TEXT,
    password_reset_expires_at TIMESTAMPTZ,

    -- OAuth and profile
    avatar_url TEXT,
    auth_provider VARCHAR(50) NOT NULL DEFAULT 'email', -- 'email', 'google', 'whatsapp'
    auth_provider_id TEXT,
    -- NOTE: role model to be replaced by PRD §3.4.1 RBAC in a follow-up migration
    role VARCHAR(20) NOT NULL DEFAULT 'normal_user' CHECK (role IN ('guest', 'normal_user', 'admin')),
    profile_data JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX ON users (activation_token) WHERE activation_token IS NOT NULL;
CREATE UNIQUE INDEX ON users (password_reset_token) WHERE password_reset_token IS NOT NULL;
CREATE UNIQUE INDEX ON users (auth_provider, auth_provider_id) WHERE auth_provider_id IS NOT NULL;

-- --- Shared taxonomy ----------------------------------------------------------

CREATE TABLE tags (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

-- --- Culture & Travel content -------------------------------------------------

-- Local activities: festivals, fairs, museums, exhibitions ("Engage" cards)
CREATE TABLE activities (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,                  -- 'Festival', 'Fair', 'Museum', 'Exhibition'
    brief_introduction TEXT,
    photograph_url TEXT,
    article_slug VARCHAR(255) UNIQUE NOT NULL,  -- Link to the detailed article
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Detailed long-form content for activities (and other editorial pages)
CREATE TABLE articles (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,                      -- Markdown or HTML
    author_id UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- History & Heritage timeline: ceramics characteristics per dynasty
CREATE TABLE ceramic_stories (
    id BIGSERIAL PRIMARY KEY,
    dynasty_name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,          -- URL-friendly access (e.g. "ming-dynasty")
    period VARCHAR(50),                         -- e.g. "Early Ming", "Late Qing"
    start_year INT,
    end_year INT,
    description TEXT NOT NULL,
    characteristics_craft TEXT,
    characteristics_art TEXT,
    image_url TEXT,
    takeaways TEXT,                             -- Brief key points for timeline view
    display_order INT UNIQUE                    -- Timeline ordering
);

-- --- Gallery ------------------------------------------------------------------

CREATE TABLE artists (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    bio TEXT,
    user_id UUID UNIQUE REFERENCES users(id) ON DELETE SET NULL, -- If the artist is a platform user
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- NOTE: to be extended into the PRD §3.2.1 Product/SKU model (price, stock,
-- packed weight, JSONB attributes) in a follow-up migration.
CREATE TABLE artworks (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    artist_id BIGINT REFERENCES artists(id) ON DELETE SET NULL,
    artist_name_override VARCHAR(255),
    thumbnail_url TEXT NOT NULL,
    description TEXT,
    period VARCHAR(50) NOT NULL,
    dimensions VARCHAR(100),                    -- e.g. "20cm x 30cm x 15cm"
    category VARCHAR(100) NOT NULL,             -- e.g. "blue and white"
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE artwork_images (
    id SERIAL PRIMARY KEY,
    artwork_id BIGINT NOT NULL REFERENCES artworks(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    is_primary BOOLEAN DEFAULT FALSE,
    caption TEXT,
    display_order INT DEFAULT 0
);

CREATE TABLE artwork_tags (
    artwork_id BIGINT NOT NULL REFERENCES artworks(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (artwork_id, tag_id)
);

-- NOTE: to evolve into the PRD §3.5 wishlist in a follow-up migration.
CREATE TABLE user_favorite_artworks (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    artwork_id BIGINT NOT NULL REFERENCES artworks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, artwork_id)
);

-- --- Notifications --------------------------------------------------------------

CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    notification_type VARCHAR(50) NOT NULL,     -- 'system'; PRD types added as modules land
    entity_type VARCHAR(50),                    -- e.g. 'order', 'itinerary_request', 'artwork'
    entity_id BIGINT,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_recipient_user_id ON notifications (recipient_user_id);
