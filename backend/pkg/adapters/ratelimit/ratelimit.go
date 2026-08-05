// Package ratelimit provides a per-key failed-attempt lockout adapter (TDD
// §333 brute-force defense for the 2FA verify endpoint). It mirrors the
// tokenblocklist/geoip/pdf adapter pattern: an interface + a Redis-backed
// implementation + a Noop implementation, so the live service is an env flip
// away and tests/worker run without Redis.
//
// The lockout is per-userID (the account-targeted defense — a distributed
// attacker can't bypass it by rotating IPs). Layer 1 (per-IP Fiber limiter
// in router.go) stops the single-IP case; this adapter is the redundant
// distributed-IP defense.
//
// Fail-open on Redis outage (mirrors tokenblocklist): a Redis outage disables
// the lockout, but the attack window is bounded by the 2FA pending-token TTL
// (5-15 min) + the per-IP limiter stays active. Availability > strictness for
// MVP; a hard fail-closed here would lock EVERY 2FA user out during a Redis
// blip.
package ratelimit

import (
	"context"
	"time"
)

// Defaults for the 2FA verify lockout (TDD §333). 5 failures in a 15-min
// window locks the account for 15 min — generous for a legit user who mistypes
// a TOTP code across ~2-3 time windows, but stops a brute-forcer after 5
// guesses. The 15-min lockout aligns with the longer pending-token TTL so a
// locked user can retry once the token has expired anyway.
const (
	MaxFailures     = 5
	FailureWindow   = 15 * time.Minute
	LockoutDuration = 15 * time.Minute
)

// AttemptTracker tracks per-key failed attempts and exposes a lockout check.
// The key is a stable identifier (userID for 2FA verify — NOT the pending
// token, so re-login doesn't reset the counter). Implementations must be safe
// for concurrent use.
type AttemptTracker interface {
	// RegisterFailure records one failed attempt for the key. When the
	// failure count reaches MaxFailures it sets the lockout. Best-effort:
	// a Redis error is logged but never blocks the auth path (the caller
	// proceeds to reject the bad code regardless).
	RegisterFailure(ctx context.Context, key string) error

	// IsLocked reports whether the key is currently locked out. Fail-open
	// on Redis error: returns (false, nil) so a Redis outage never locks a
	// legit user out (bounded by the pending-token TTL + per-IP limiter).
	IsLocked(ctx context.Context, key string) (bool, error)

	// Reset clears the failure counter + lockout for the key. Called on a
	// successful 2FA verify so a user who mistyped then got it right doesn't
	// carry a near-threshold counter forward. Best-effort.
	Reset(ctx context.Context, key string) error
}

// NoopAttemptTracker is a no-op tracker (tests, worker, no-Redis path).
type NoopAttemptTracker struct{}

func (NoopAttemptTracker) RegisterFailure(context.Context, string) error  { return nil }
func (NoopAttemptTracker) IsLocked(context.Context, string) (bool, error) { return false, nil }
func (NoopAttemptTracker) Reset(context.Context, string) error            { return nil }
