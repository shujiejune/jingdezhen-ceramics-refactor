-- Recreate the legacy artworks stack (empty — data not restored; this is a
-- pre-prod baseline so dropping loses no production data).
-- Tables recreated in FK-dependency order (parents first).

CREATE TABLE artworks (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    artist_id BIGINT REFERENCES artists(id) ON DELETE SET NULL,
    artist_name_override VARCHAR(255),
    thumbnail_url TEXT NOT NULL,
    description TEXT,
    period VARCHAR(50) NOT NULL,
    dimensions VARCHAR(100),
    category VARCHAR(100) NOT NULL,
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

CREATE TABLE user_favorite_artworks (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    artwork_id BIGINT NOT NULL REFERENCES artworks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, artwork_id)
);

-- Drop the wishlists table.
DROP TABLE IF EXISTS wishlists CASCADE;
