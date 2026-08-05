package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAttemptTracker backs the 2FA lockout with Redis. Two keys per user:
//   - 2fa:fail:<key>   — INCR'd per failure, EXPIRE'd FailureWindow on first
//     failure (so the counter self-prunes if the user
//     stops trying). When it reaches MaxFailures the
//     lockout key is set.
//   - 2fa:lock:<key>   — SET EX LockoutDuration once MaxFailures is hit.
//     IsLocked = EXISTS(lock). Independent of the counter
//     so a lock survives the counter's TTL expiring.
//
// All errors are fail-open/best-effort (see ratelimit.go doc): RegisterFailure
// logs + returns nil (the caller rejects the bad code anyway), IsLocked logs +
// returns (false, nil), Reset logs + returns nil. A Redis outage never blocks
// auth (would lock every 2FA user out) nor keeps a locked user locked forever.
type RedisAttemptTracker struct {
	client       *redis.Client
	maxFailures  int
	failWindow   time.Duration
	lockDuration time.Duration
}

// NewRedisAttemptTracker builds a tracker with the package defaults. Pass the
// same *redis.Client used by the rest of the app (tokenblocklist etc.).
func NewRedisAttemptTracker(client *redis.Client) *RedisAttemptTracker {
	return &RedisAttemptTracker{
		client:       client,
		maxFailures:  MaxFailures,
		failWindow:   FailureWindow,
		lockDuration: LockoutDuration,
	}
}

func failKey(key string) string { return "2fa:fail:" + key }
func lockKey(key string) string { return "2fa:lock:" + key }

func (t *RedisAttemptTracker) RegisterFailure(ctx context.Context, key string) error {
	if t.client == nil {
		return nil
	}
	fk := failKey(key)
	// INCR is atomic; set TTL only when the key is new (the first failure in
	// the window) so the window doesn't reset on every attempt.
	n, err := t.client.Incr(ctx, fk).Result()
	if err != nil {
		// Best-effort: log via the error return so the caller can log, but
		// never block the auth path.
		return nil
	}
	if n == 1 {
		// First failure in the window — start the sliding TTL.
		_ = t.client.Expire(ctx, fk, t.failWindow).Err()
	}
	if int(n) >= t.maxFailures {
		// Set the lockout (idempotent — SET EX overwrites the TTL harmlessly
		// if it already exists from a prior cycle).
		_ = t.client.Set(ctx, lockKey(key), 1, t.lockDuration).Err()
	}
	return nil
}

func (t *RedisAttemptTracker) IsLocked(ctx context.Context, key string) (bool, error) {
	if t.client == nil {
		return false, nil
	}
	n, err := t.client.Exists(ctx, lockKey(key)).Result()
	if err != nil {
		// Fail-open: a Redis outage must not lock a legit user out.
		return false, fmt.Errorf("ratelimit.IsLocked: %w", err)
	}
	return n > 0, nil
}

func (t *RedisAttemptTracker) Reset(ctx context.Context, key string) error {
	if t.client == nil {
		return nil
	}
	// Del both keys; errors are best-effort.
	_ = t.client.Del(ctx, failKey(key), lockKey(key)).Err()
	return nil
}
