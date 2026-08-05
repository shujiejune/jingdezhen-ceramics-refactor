-- =============================================================================
-- 000027_audit_log: admin audit log for sensitive actions (TDD §135, PRD §3.1.1)
--
-- PRD §3.1.1 line 299: "Admin audit log of sensitive actions (GDPR accountability)."
-- TDD §135: `audit_log(id, actor_id, action, entity_type, entity_id, detail JSONB, created_at)`.
--
-- Records every sensitive admin/staff action: content transitions (approve/
-- reject/unpublish), deletes, role changes, order refunds, itinerary transitions,
-- GDPR erasure, media/shipping-tier/option-rate deletes. 28 endpoints instrumented.
--
-- Design:
--   actor_id  UUID NULL — the authenticated user who performed the action. NULL
--     when the actor has since been GDPR-erased (ON DELETE SET NULL preserves
--     the audit row — accountability survives erasure, mirroring consent_records
--     + content authorship). The erasure action itself is logged BEFORE the
--     user is anonymized, so actor_id is captured for that row.
--   actor_ip_hash — HMAC-SHA256 of the actor's IP (no raw IP stored — GDPR
--     minimisation, mirrors consent_records). Reuses CONSENT_HMAC_KEY (same
--     short-term audit/dedup purpose, not re-identification).
--   action — stable lowercase kebab-case (e.g. 'order.refund',
--     'content.approve', 'user.role.assign', 'privacy.delete-account').
--   entity_type — product|sku|ceramic_story|activity|artist|user|order|
--     itinerary_request|media_asset|shipping_fee_tier|option_rate|account.
--   entity_id VARCHAR — mixed ID types (BIGINT for products/orders, UUID for
--     users) stored as a string; the UI casts as needed.
--   detail JSONB — action-specific payload (e.g. {locale, reason, role}).
--
-- Append-only. NOT partitioned for MVP (same decision as analytics_events: MVP
-- volume is low; BRIN on created_at keeps the list query cheap; convert to
-- RANGE partitioning later in one migration if volume grows).
-- =============================================================================

CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    actor_id      UUID REFERENCES users(id) ON DELETE SET NULL, -- NULL if actor erased
    actor_ip_hash VARCHAR(64),                                 -- hex(HMAC(key, IP)); no raw IP
    action        VARCHAR(60) NOT NULL,                        -- e.g. 'order.refund'
    entity_type   VARCHAR(40) NOT NULL,                        -- e.g. 'order'
    entity_id     VARCHAR(60),                                 -- string (BIGINT or UUID)
    detail        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Read path: list/filter by entity (the "what happened to this thing" view).
CREATE INDEX idx_audit_entity ON audit_log (entity_type, entity_id);
-- Read path: list/filter by actor (the "what did this person do" view).
CREATE INDEX idx_audit_actor  ON audit_log (actor_id, created_at) WHERE actor_id IS NOT NULL;
-- Read path: the default chronological list + date-range filter.
CREATE INDEX idx_audit_created ON audit_log (created_at);