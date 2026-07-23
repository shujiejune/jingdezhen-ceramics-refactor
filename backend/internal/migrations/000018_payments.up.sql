-- =============================================================================
-- 000018_payments: payment records + idempotency (PRD §3.2.3, TDD §3.4/§10)
--
-- One payment per gateway intent. The webhook handler upserts by idempotency_key
-- (UNIQUE) so a gateway retry or duplicate webhook is a no-op (TDD §11 priority:
-- webhook idempotency). The order's created→paid transition is driven by the
-- payment:finalize job enqueued after a verified succeeded webhook.
--
-- `gateway_ref` is the gateway's reference for the intent (Airwallex intent id /
-- PayPal order id / mock-<orderID>). Full refunds only (PRD §3.2.3): a refund
-- moves the row status→refunded and the order paid|shipped→refunded.
--
-- raw_webhook captures the last verified webhook payload for audit.
-- =============================================================================

CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    order_id        BIGINT NOT NULL REFERENCES orders(id),
    gateway         VARCHAR(20) NOT NULL CHECK (gateway IN ('airwallex','paypal','mock')),
    gateway_ref     VARCHAR(200) NOT NULL,          -- gateway intent/order id
    status          VARCHAR(20) NOT NULL CHECK (status IN ('pending','succeeded','failed','refunded')),
    amount_minor    BIGINT NOT NULL,                 -- presentment minor units (matches order currency)
    currency        CHAR(3) NOT NULL,                 -- presentment currency (USD/EUR/GBP)
    raw_webhook     JSONB,
    idempotency_key VARCHAR(200) NOT NULL UNIQUE,    -- gateway ref + event id → replay no-op
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order_id    ON payments (order_id);
CREATE INDEX idx_payments_gateway_ref ON payments (gateway_ref);
