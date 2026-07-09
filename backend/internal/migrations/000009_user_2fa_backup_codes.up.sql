-- =============================================================================
-- 000009_user_2fa_backup_codes: one-time backup codes for 2FA recovery
-- (PRD §4.3 — a super_admin who loses their authenticator must not be locked out)
--
-- Generated when 2FA is enabled (the confirm step), shown ONCE to the user,
-- and stored HASHED (SHA-256 with the app key as a pepper) so a DB leak alone
-- cannot recover them. A backup code is consumed atomically at the login-verify
-- step (after the TOTP check fails) by setting used_at; each is single-use.
-- Used rows are retained for audit. Regenerate deletes only UNUSED rows.
-- =============================================================================

CREATE TABLE user_2fa_backup_codes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  CHAR(64) NOT NULL,              -- SHA-256 hex of (pepper || normalized code)
    used_at    TIMESTAMPTZ,                     -- NULL = available, set = consumed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Find a user's available (unused) backup codes; also the consume target.
CREATE INDEX idx_2fa_backup_user_unused ON user_2fa_backup_codes (user_id) WHERE used_at IS NULL;
