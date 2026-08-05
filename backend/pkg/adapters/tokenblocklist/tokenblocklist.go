// Package tokenblocklist provides a denylist of revoked access tokens, keyed by
// user_id. It is the stopgap mechanism for invalidating a deleted user's
// outstanding JWT before refresh-token rotation (TDD §5.1) lands.
//
// The blocklist is consulted by the JWT auth middleware on every authenticated
// request: if the token's user_id is revoked, the request is rejected with 401
// even though the signature + expiry are still valid.
//
// Design (see TDD §12):
//   - Fail-open on Redis outage: a deleted user's token works only during the
//     rare outage and still self-expires within MaxAccessTokenTTL; login is
//     already blocked by the is_active=false check. A DB backstop can be added
//     in a later security pass.
//   - TTL = MaxAccessTokenTTL (30d): by the time the key expires, every
//     outstanding token for that user has also expired, so the denylist can be
//     pruned without missing a revocation.
//   - Keyed by user_id (not jti): the only MVP trigger is GDPR erasure, which
//     invalidates ALL of a user's tokens at once. Per-token (jti) revocation is
//     a future extension; the Blocklist interface is shaped to accommodate it.
package tokenblocklist

import (
	"context"

	"time"
)

// MaxAccessTokenTTL is the maximum lifetime of an access token (matches the
// 30-day access token minted by user_service.generateAuthResponse). Revocation
// keys are set with this TTL so they live exactly long enough to outlive any
// outstanding token. This is the single source of truth for the access-token
// lifetime — user_service references it when minting.
const MaxAccessTokenTTL = 30 * 24 * time.Hour

// Blocklist is the interface the auth middleware + privacy service depend on.
// NoopBlocklist skips the check (tests / worker / no-Redis); RedisBlocklist is
// the production implementation.
type Blocklist interface {
	// Revoke marks the given user's outstanding tokens as invalid for up to
	// ttl (clamped to MaxAccessTokenTTL by callers). Idempotent: revoking an
	// already-revoked user refreshes the TTL.
	Revoke(ctx context.Context, userID string, ttl time.Duration) error

	// IsRevoked reports whether the user has been revoked. Returns false (never
	// an error) on a backend outage — fail-open so a Redis outage does not lock
	// out every authenticated user.
	IsRevoked(ctx context.Context, userID string) (bool, error)
}

// NoopBlocklist is a no-op Blocklist for tests, the worker process, and any
// deployment path without Redis. Every check passes (nothing is revoked).
type NoopBlocklist struct{}

// Revoke is a no-op.
func (NoopBlocklist) Revoke(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

// IsRevoked always returns false.
func (NoopBlocklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, nil
}
