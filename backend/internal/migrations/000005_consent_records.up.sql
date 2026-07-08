-- =============================================================================
-- 000005_consent_records: GDPR consent ledger (TDD §3.4, PRD §4.3)
--
-- Immutable append-only record of every consent action. `user_id` is NULLABLE
-- so anonymous visitor cookie consent (before signup/login) can be recorded;
-- it is back-linked once the user is known. `granted = FALSE` records a
-- withdrawal/refusal (GDPR requires recording refusals too, not just grants).
--
-- `ip_hash` stores an HMAC of the visitor's IP (no raw IPs — GDPR minimisation,
-- TDD §11). The HMAC key rotates daily, so the hash is only useful for
-- short-term dedup/audit, not re-identification.
--
-- `doc_version` ties the consent to a specific version of the policy/terms,
-- so a policy change requires fresh consent (re-consent flow).
-- =============================================================================

CREATE TABLE consent_records (
    id           BIGSERIAL PRIMARY KEY,
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL, -- NULL = anonymous visitor
    kind         VARCHAR(30) NOT NULL CHECK (
        kind IN ('privacy_policy', 'tos', 'cookie_analytics', 'cookie_marketing')
    ),
    doc_version  VARCHAR(20) NOT NULL,           -- e.g. "1.0"; ties consent to a policy version
    granted      BOOLEAN   NOT NULL,             -- FALSE = withdrawn/refused
    ip_hash      VARCHAR(64),                    -- HMAC(IP), daily-rotating key; no raw IPs
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Latest-consent lookups: "what did this user most recently say for this kind?"
CREATE INDEX idx_consent_records_user_kind
    ON consent_records (user_id, kind, created_at DESC)
    WHERE user_id IS NOT NULL;

-- Anonymous (pre-login) consent lookups by IP hash.
CREATE INDEX idx_consent_records_ip_kind
    ON consent_records (ip_hash, kind, created_at DESC)
    WHERE user_id IS NULL;
