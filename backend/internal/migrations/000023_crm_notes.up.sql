-- =============================================================================
-- 000023_crm_notes: Planner CRM — internal notes + SLA-breach tracking
-- (PRD §3.3.2 "Backend/CRM (Travel Planner role)", TDD §3.4 M3 sub-track #2)
--
-- The customer-facing wizard tables (itinerary_requests, itinerary_drafts)
-- landed in 000022. This migration adds the planner-facing pieces that were
-- explicitly deferred there:
--   - crm_notes       — internal planner notes (contact history) per request.
--   - sla_notified_at — exactly-once notification flag for the sla:check cron
--                       (a recurring 15min cron would otherwise re-notify every
--                       breached request; CAS-set this on first breach).
--
-- itinerary_requests already carries the full status state machine + assigned_to
-- (000022), so no status/assignment columns are added here.
-- =============================================================================

-- Internal planner notes — a narrative trail of CRM follow-ups (PRD §3.3.2
-- "internal notes, contact history"). author_id = ON DELETE NO ACTION so a
-- planner's notes survive their account erasure (mirrors orders). request_id
-- = ON DELETE CASCADE (a deleted request takes its notes; requests are never
-- hard-deleted in normal operation — they move to cancelled/closed).
CREATE TABLE crm_notes (
    id          BIGSERIAL PRIMARY KEY,
    request_id  BIGINT NOT NULL REFERENCES itinerary_requests(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL REFERENCES users(id) ON DELETE NO ACTION,
    body        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_crm_notes_request_id ON crm_notes (request_id);

-- SLA-breach notification flag. NULL = not yet notified; set once by the
-- sla:check cron when a pending/processing request first breaches its
-- sla_deadline. The CAS `SET sla_notified_at=NOW() WHERE sla_notified_at IS
-- NULL` makes the notification exactly-once even under concurrent cron runs.
ALTER TABLE itinerary_requests ADD COLUMN sla_notified_at TIMESTAMPTZ;

-- Partial index for the cron's breach scan: only rows still pending/processing
-- and not yet notified. Keeps the scan cheap even as the table grows.
CREATE INDEX idx_itin_req_sla_breach
    ON itinerary_requests (sla_deadline)
    WHERE sla_notified_at IS NULL AND status IN ('pending','processing');
