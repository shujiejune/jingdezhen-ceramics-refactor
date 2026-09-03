// Package ratelimit also provides a per-email send-frequency throttle for the
// resend-activation and request-password-reset endpoints (REFACTOR-TODO C5).
//
// The throttle is per-email (not per-IP) to stop an attacker who rotates IPs
// from flooding a single mailbox. The existing 5/min/IP auth-group limiter
// (TDD §333 layer 1) is the per-IP defense; this throttle is the per-mailbox
// defense that the per-IP limiter cannot provide.
//
// Design: a Redis fixed-window counter (INCR + EXPIRE) keyed on the email
// hash. 3 sends per hour per email — generous for a legit user who retries
// after a typo, but stops a flood. Fail-open on Redis outage (the per-IP
// limiter stays active).
package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

// Email throttle defaults (REFACTOR-TODO C5). 3 sends per hour per email.
const (
	EmailMaxSends    = 3
	EmailSendWindow  = 1 * time.Hour
	EmailLockoutTime = 1 * time.Hour // locked for the rest of the window
)

// EmailThrottler checks whether an email address has exceeded its send
// frequency limit. The key is the email (hashed for privacy — the Redis
// key doesn't store the raw address). Implementations must be safe for
// concurrent use.
type EmailThrottler interface {
	// Allow checks whether the email can receive another send. Returns
	// true if allowed (and silently records the send); false if the
	// throttle limit has been reached. Fail-open on Redis error: returns
	// (true, nil) so a Redis outage never blocks a legit password reset
	// (the per-IP limiter stays active).
	Allow(ctx context.Context, email string) (bool, error)

	// Reset clears the throttle counter for the email. Called on a
	// successful activation or password reset so a user who retried
	// then succeeded doesn't carry a near-threshold counter. Best-effort.
	Reset(ctx context.Context, email string) error
}

// NoopEmailThrottler is a no-op throttler (tests, worker, no-Redis path).
type NoopEmailThrottler struct{}

func (NoopEmailThrottler) Allow(context.Context, string) (bool, error) { return true, nil }
func (NoopEmailThrottler) Reset(context.Context, string) error         { return nil }

// RedisEmailThrottler backs the per-email throttle with Redis. One key per
// email: email:throttle:<sha256(email)> — INCR'd per send, EXPIRE'd
// EmailSendWindow on first send. Allow returns false when the count exceeds
// EmailMaxSends. The key self-prunes after the window expires.
type RedisEmailThrottler struct {
	client   *redis.Client
	maxSends int
	window   time.Duration
	lockout  time.Duration
}

// NewRedisEmailThrottler builds a throttler with the package defaults.
func NewRedisEmailThrottler(client *redis.Client) *RedisEmailThrottler {
	return &RedisEmailThrottler{
		client:   client,
		maxSends: EmailMaxSends,
		window:   EmailSendWindow,
		lockout:  EmailLockoutTime,
	}
}

func emailThrottleKey(email string) string {
	h := sha256.Sum256([]byte(email))
	return "email:throttle:" + hex.EncodeToString(h[:])
}

// Allow checks whether the email can receive another send and records it.
// Returns false if the throttle limit has been reached.
func (t *RedisEmailThrottler) Allow(ctx context.Context, email string) (bool, error) {
	if t.client == nil {
		return true, nil
	}
	k := emailThrottleKey(email)
	n, err := t.client.Incr(ctx, k).Result()
	if err != nil {
		// Fail-open: Redis outage must not block a legit password reset.
		return true, nil
	}
	if n == 1 {
		// First send in the window — start the sliding TTL.
		_ = t.client.Expire(ctx, k, t.window).Err()
	}
	return int(n) <= t.maxSends, nil
}

// Reset clears the throttle counter for the email.
func (t *RedisEmailThrottler) Reset(ctx context.Context, email string) error {
	if t.client == nil {
		return nil
	}
	_ = t.client.Del(ctx, emailThrottleKey(email)).Err()
	return nil
}
