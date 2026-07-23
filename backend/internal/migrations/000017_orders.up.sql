-- =============================================================================
-- 000017_orders: order + order_items (PRD §3.2.3, TDD §3.4/§7/§8)
--
-- Order lifecycle (TDD §8):
--   created →(payment succeeded)→ paid →(operator enters tracking)→ shipped
--        →(auto N days / confirm)→ completed
--   created → cancelled (customer/timeout; stock restored)
--   paid|shipped → refunded (operator; full refunds only — PRD §3.2.3)
--
-- Order snapshot (TDD §7): the order stores the presentment totals
-- (subtotal_minor/shipping_minor/total_minor), currency, fx_rate_used, AND the
-- CNY totals (subtotal_cny/shipping_cny/total_cny) for settlement. Later FX
-- rate changes never affect placed orders.
--
-- Stock is decremented atomically inside the order-creation transaction
-- (TDD §4.3: UPDATE skus SET stock=stock-$1 WHERE id=$2 AND stock>=$1; zero
-- rows → rollback → ErrConflict). Cancelled orders restore stock.
--
-- GDPR: orders outlive erasure (anonymize-in-place, not hard delete) —
-- user_id is a plain FK with NO ACTION (not CASCADE), so an anonymized user
-- stub retains its order history for audit/retention.
-- =============================================================================

CREATE TABLE orders (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),     -- NO ACTION (orders survive erasure)
    status          VARCHAR(20) NOT NULL DEFAULT 'created'
                    CHECK (status IN ('created','paid','shipped','completed','cancelled','refunded')),
    currency        CHAR(3) NOT NULL,                       -- presentment currency (USD/EUR/GBP)
    subtotal_minor  BIGINT NOT NULL DEFAULT 0,              -- presentment Σ line
    shipping_minor  BIGINT NOT NULL DEFAULT 0,              -- presentment shipping
    total_minor     BIGINT NOT NULL DEFAULT 0,              -- subtotal_minor + shipping_minor
    subtotal_cny    BIGINT NOT NULL DEFAULT 0,              -- CNY base (settlement)
    shipping_cny    BIGINT NOT NULL DEFAULT 0,
    total_cny       BIGINT NOT NULL DEFAULT 0,
    fx_rate_used    NUMERIC(18,8),                           -- presentment→CNY rate snapshot
    address         JSONB NOT NULL,                         -- immutable shipping-address snapshot
    locale          VARCHAR(10),                            -- order language (for emails/receipts)
    carrier_name    VARCHAR(100),                           -- operator-entered (PRD §3.2.3: no carrier API)
    tracking_number VARCHAR(200),
    placed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at         TIMESTAMPTZ,
    shipped_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    refunded_at     TIMESTAMPTZ,
    cancel_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_status   ON orders (user_id, status);
CREATE INDEX idx_orders_status        ON orders (status);

-- --- Order items (immutable snapshot; survives product edits) ----------------

CREATE TABLE order_items (
    id                  BIGSERIAL PRIMARY KEY,
    order_id            BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku_id              BIGINT NOT NULL REFERENCES skus(id),
    qty                 INT NOT NULL CHECK (qty > 0),
    unit_price_minor    BIGINT NOT NULL,    -- presentment minor units (charge currency)
    unit_price_cny      BIGINT NOT NULL,    -- CNY minor units (fen) — base
    title_snapshot      JSONB,              -- localized product title at purchase time
    attributes_snapshot JSONB,              -- SKU attributes at purchase time
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);
