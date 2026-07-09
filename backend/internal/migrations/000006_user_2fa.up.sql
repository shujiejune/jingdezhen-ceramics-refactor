-- =============================================================================
-- 000006_user_2fa: TOTP two-factor authentication (TDD §3.4, PRD §4.3)
--
-- Per TDD §3.4: user_2fa(user_id, totp_secret_enc, enabled, confirmed_at).
-- TOTP is mandatory for super_admin (enforced at login) and optional for other
-- staff roles (PRD §4.3). The TOTP secret is encrypted at rest with an app key
-- (TDD §5.3) so a DB dump alone cannot recover it — the encryption key lives
-- only in server env.
--
-- Enrollment is two-phase: `enabled` is set TRUE only after the user proves
-- they can generate a valid code (confirmed_at). Until then the secret is
-- staged but login does not challenge for it.
-- =============================================================================

CREATE TABLE user_2fa (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    totp_secret_enc BYTEA NOT NULL,             -- AES-GCM-encrypted TOTP secret
    enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    confirmed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
