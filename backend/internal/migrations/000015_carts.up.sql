-- =============================================================================
-- 000015_carts: M2 commerce — server-side shopping cart (TDD §3.4, PRD §3.2.3)
--
-- One cart per signed-in user (guest carts live in browser localStorage and are
-- merged on login via POST /cart/merge — no anonymous server carts). Cart items
-- are keyed on SKU (the purchasable unit), like wishlists.
--
-- Stock is NOT decremented at the cart stage — the authoritative atomic
-- decrement happens at checkout (TDD §4.3). A qty>stock guard in the cart
-- service is advisory (prevents obviously-stale carts).
--
-- GDPR erasure is anonymize-in-place (not hard delete), so orphaned cart rows
-- on an is_active=false stub are unreachable (login is rejected); ON DELETE
-- CASCADE mirrors wishlists.user_id for a future hard-delete path.
-- =============================================================================

-- --- Carts (one per user) -----------------------------------------------------

CREATE TABLE carts (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_carts_user_id ON carts (user_id);

-- --- Cart items (qty keyed on SKU) -------------------------------------------

CREATE TABLE cart_items (
    id         BIGSERIAL PRIMARY KEY,
    cart_id    BIGINT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    sku_id     BIGINT NOT NULL REFERENCES skus(id) ON DELETE CASCADE,
    qty        INT NOT NULL CHECK (qty > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cart_id, sku_id)
);

CREATE INDEX idx_cart_items_cart_id ON cart_items (cart_id);
