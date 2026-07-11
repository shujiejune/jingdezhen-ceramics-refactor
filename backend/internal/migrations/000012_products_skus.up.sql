-- =============================================================================
-- 000012_products_skus: M2 commerce — product/SKU model (TDD §3.4, PRD §3.2.1)
--
-- Evolves the inherited `artworks` table into a proper product catalog with
-- i18n translations (TDD §3.2 pattern), multi-dimensional SKUs, money as
-- BIGINT minor units (fen), JSONB attributes, and the editorial workflow.
--
-- `artworks` is NOT dropped (additive — the gallery module still reads it).
-- Existing artwork data is backfilled into `products` + en-US translations +
-- a default SKU per artwork so the catalog isn't empty on a seeded DB.
--
-- Cart/checkout/orders/payments build on top of SKUs in later migrations.
-- =============================================================================

-- --- Product parent (non-localized) -------------------------------------------

CREATE TABLE products (
    id            BIGSERIAL PRIMARY KEY,
    artist_id     BIGINT REFERENCES artists(id) ON DELETE SET NULL,
    category      VARCHAR(100),                 -- bare string for now; category-tree migration lands later
    thumbnail_url TEXT,                         -- primary display image (media_assets FK deferred)
    display_order INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_artist_id    ON products (artist_id) WHERE artist_id IS NOT NULL;
CREATE INDEX idx_products_display_order ON products (display_order);

-- --- Product translations (i18n + editorial workflow) ------------------------

CREATE TABLE product_translations (
    id                BIGSERIAL PRIMARY KEY,
    product_id        BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    locale            VARCHAR(10) NOT NULL,   -- BCP 47: en-US, zh-CN
    title             VARCHAR(255) NOT NULL,
    slug              VARCHAR(100) NOT NULL,
    description       TEXT,
    meta_title        VARCHAR(255),
    meta_description  TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft','in_review','published','rejected')),
    reviewed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    published_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, locale),
    UNIQUE (locale, slug)
);

CREATE INDEX idx_prod_trans_product_locale ON product_translations (product_id, locale);
CREATE INDEX idx_prod_trans_locale_slug    ON product_translations (locale, slug);
CREATE INDEX idx_prod_trans_locale_published ON product_translations (locale) WHERE status = 'published';

-- --- SKUs (purchasable units; not localized) ----------------------------------

CREATE TABLE skus (
    id                    BIGSERIAL PRIMARY KEY,
    product_id            BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku_code              VARCHAR(100) NOT NULL UNIQUE,
    price_cny             BIGINT NOT NULL DEFAULT 0,   -- minor units (fen); CNY base currency
    stock                 INT NOT NULL DEFAULT 0,
    weight_grams          INT NOT NULL DEFAULT 0,      -- packed weight, drives shipping calc (PRD §3.2.3)
    low_stock_threshold   INT NOT NULL DEFAULT 2,      -- PRD §3.2.1 default
    attributes            JSONB NOT NULL DEFAULT '{}'::jsonb,  -- size, technique, glaze, edition, year, kiln
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_skus_product_id ON skus (product_id);
CREATE INDEX idx_skus_price_cny  ON skus (price_cny) WHERE is_active = TRUE;
-- GIN for JSONB attribute filtering (e.g. WHERE attributes @> '{"edition_type":"one_of_a_kind"}').
CREATE INDEX idx_skus_attributes ON skus USING GIN (attributes);

-- --- RBAC: add product.publish permission (mirrors content.publish) -----------
-- Only super_admin can approve/publish products (PRD §3.1.1 — approval required
-- for all content types including commerce content).

INSERT INTO permissions (key, description)
VALUES ('product.publish', 'Approve and publish products (Super Admin only in v1)')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.key = 'super_admin' AND p.key = 'product.publish'
ON CONFLICT DO NOTHING;

-- Also grant product.read + product.write + product.publish to super_admin
-- (product.read + product.write already assigned via the 000002 CROSS JOIN for
-- super_admin = all permissions, but product.publish was added after that seed).
-- The CROSS JOIN in 000002 only ran for permissions existing at that time, so
-- the INSERT above is sufficient; super_admin already has product.read/write.

-- --- Backfill: artworks → products + translations + default SKU ---------------

INSERT INTO products (id, artist_id, category, thumbnail_url, display_order, created_at, updated_at)
SELECT
    a.id,
    a.artist_id,
    a.category,
    a.thumbnail_url,
    0,
    a.created_at,
    COALESCE(a.updated_at, a.created_at)
FROM artworks a
ON CONFLICT (id) DO NOTHING;

-- Reset the products sequence so new products don't collide with backfilled IDs.
SELECT setval(pg_get_serial_sequence('products', 'id'),
              COALESCE((SELECT MAX(id) FROM products), 0) + 1, false);

-- en-US translations from artwork title/description.
INSERT INTO product_translations (product_id, locale, title, slug, description, status, published_at)
SELECT
    a.id, 'en-US',
    a.title,
    -- Derive a slug from the title (lowercase, spaces→dashes). Collisions surface
    -- as a unique-violation at apply time — operator fixes in CMS. For the typical
    -- empty-dev-DB case there is no collision.
    LOWER(REPLACE(TRIM(a.title), ' ', '-')),
    a.description,
    'published', NOW()
FROM artworks a
ON CONFLICT (product_id, locale) DO NOTHING;

-- Default SKU per product (price=0, stock=0, weight=0; operator fills in CMS).
-- One-of-a-kind artworks get stock=1 per PRD §3.2.1 (we can't distinguish them
-- at backfill time, so default stock=0 and let operators correct).
INSERT INTO skus (product_id, sku_code, price_cny, stock, weight_grams, attributes, is_active)
SELECT
    p.id,
    'SKU-' || p.id,
    0, 0, 0, '{}'::jsonb, TRUE
FROM products p
ON CONFLICT (sku_code) DO NOTHING;
