-- GDPR erasure support (PRD §4.3, TDD §3.1).
--
-- The user row is retained as an anonymized stub on erasure (email/nickname/
-- password_hash/avatar/tokens/auth_provider_id all NULLed, is_active=false,
-- deleted_at set) so that future order records keep referential integrity while
-- all PII is purged. Truly personal ancillary data (addresses, 2FA secrets,
-- backup codes, favorites, notifications) is CASCADE-deleted via existing FKs.
-- consent_records.user_id is SET NULL (audit ledger retained, anonymized).
-- articles.author_id / artists.user_id are SET NULL (content outlives the author).
--
-- "Hard deletes only for GDPR erasure" (TDD §3.1) — this is the only path that
-- purges a user; normal lifecycle uses status columns.

ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;

-- Housekeeping index for deleted-account sweeps (e.g. JWT blocklist cleanup).
CREATE INDEX idx_users_deleted_at ON users (deleted_at) WHERE deleted_at IS NOT NULL;
