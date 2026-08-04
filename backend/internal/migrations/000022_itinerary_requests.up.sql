-- =============================================================================
-- 000022_itinerary_requests: Custom Itinerary Builder customer flow
-- (PRD §3.3.2, TDD §3.4 M3)
--
-- Two tables:
--   itinerary_requests — a submitted request (status state machine, SLA,
--                         immutable form snapshot). user_id FK = NO ACTION so
--                         requests survive GDPR anonymize-in-place erasure
--                         (mirrors orders).
--   itinerary_drafts   — one-per-user save-resume state (user_id UNIQUE).
--
-- This migration scopes to the customer-facing submit + draft flow only. The
-- planner CRM tables (itinerary_quotes, option_rates, crm_notes) and the
-- chatbot tables (chat_sessions, chat_messages) land in follow-up migrations.
-- =============================================================================

CREATE TABLE itinerary_requests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),  -- NO ACTION (survive erasure)
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','processing','quoted','deposit_paid',
                                      'confirmed','cancelled','closed')),
    -- Step 1 — Trip basics
    arrival_date    DATE,
    duration_days   INT NOT NULL CHECK (duration_days > 0),
    flexible        BOOLEAN NOT NULL DEFAULT FALSE,
    adults          INT NOT NULL CHECK (adults >= 1),
    children        INT NOT NULL DEFAULT 0 CHECK (children >= 0),
    -- Step 2 — Preferences
    interests       JSONB NOT NULL DEFAULT '[]'::jsonb,   -- array of string keys
    budget          JSONB,                                -- {currency, min_minor, max_minor}
    pace            VARCHAR(20) NOT NULL CHECK (pace IN ('relaxed','balanced','packed')),
    -- Step 3 — Services
    services        JSONB NOT NULL DEFAULT '{}'::jsonb,   -- guide, hotel, pickup, experience, dietary
    -- Step 4 — Contact & consent
    contact         JSONB NOT NULL,                       -- {channel, whatsapp_number, notes}
    notes           TEXT,                                 -- "anything else we should know"
    -- Auto-attached
    locale          VARCHAR(10) NOT NULL,                 -- BCP 47 (en-US, zh-CN)
    sla_deadline    TIMESTAMPTZ NOT NULL,                 -- submitted_at + 24h (CMS-configurable later)
    assigned_to     UUID REFERENCES users(id) ON DELETE SET NULL,
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancel_reason   TEXT,
    cancelled_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_itin_req_user_id     ON itinerary_requests (user_id);
CREATE INDEX idx_itin_req_status      ON itinerary_requests (status);
CREATE INDEX idx_itin_req_sla_deadline ON itinerary_requests (sla_deadline);

-- One draft per signed-in user (save & resume). ON DELETE CASCADE mirrors
-- cart/wishlist (a deleted user has no draft).
CREATE TABLE itinerary_drafts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    form_state  JSONB NOT NULL DEFAULT '{}'::jsonb,
    step        INT NOT NULL DEFAULT 1 CHECK (step BETWEEN 1 AND 4),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
