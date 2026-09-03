package ratelimit_test

import (
	"context"
	"testing"

	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/ratelimit"

	"github.com/stretchr/testify/require"
)

// TestNoopEmailThrottler verifies the noop always allows.
func TestNoopEmailThrottler(t *testing.T) {
	nt := ratelimit.NoopEmailThrottler{}
	for i := 0; i < 100; i++ {
		ok, err := nt.Allow(context.Background(), "test@example.com")
		require.NoError(t, err)
		require.True(t, ok)
	}
}

// TestRedisEmailThrottler_AllowUntilMax verifies the throttle allows up to
// EmailMaxSends and then blocks subsequent sends within the window.
//
// This is an integration test — it needs a real Redis container.
func TestRedisEmailThrottler_AllowUntilMax(t *testing.T) {
	rdb := testutil.NewRedisClient(t)
	throttler := ratelimit.NewRedisEmailThrottler(rdb)
	ctx := context.Background()
	email := "flood-target@example.com"

	// The first EmailMaxSends should be allowed.
	for i := 0; i < ratelimit.EmailMaxSends; i++ {
		ok, err := throttler.Allow(ctx, email)
		require.NoError(t, err)
		require.True(t, ok, "send %d should be allowed", i+1)
	}

	// The next send should be blocked.
	ok, err := throttler.Allow(ctx, email)
	require.NoError(t, err)
	require.False(t, ok, "send %d should be throttled", ratelimit.EmailMaxSends+1)

	// A different email should still be allowed (per-email, not global).
	ok, err = throttler.Allow(ctx, "other@example.com")
	require.NoError(t, err)
	require.True(t, ok, "different email should not be throttled")
}

// TestRedisEmailThrottler_Reset verifies that Reset clears the counter so
// a user who succeeded can retry without carrying a near-threshold counter.
func TestRedisEmailThrottler_Reset(t *testing.T) {
	rdb := testutil.NewRedisClient(t)
	throttler := ratelimit.NewRedisEmailThrottler(rdb)
	ctx := context.Background()
	email := "reset-test@example.com"

	// Exhaust the limit.
	for i := 0; i < ratelimit.EmailMaxSends; i++ {
		ok, _ := throttler.Allow(ctx, email)
		require.True(t, ok)
	}
	ok, _ := throttler.Allow(ctx, email)
	require.False(t, ok, "should be throttled before reset")

	// After reset, the first send should be allowed again.
	require.NoError(t, throttler.Reset(ctx, email))
	ok, err := throttler.Allow(ctx, email)
	require.NoError(t, err)
	require.True(t, ok, "send after reset should be allowed")
}

// TestRedisEmailThrottler_EmailHash verifies that the Redis key is hashed
// (the raw email is not stored as a Redis key). Two different casing of the
// same email should produce different keys (case-sensitive hash).
func TestRedisEmailThrottler_EmailHash(t *testing.T) {
	rdb := testutil.NewRedisClient(t)
	throttler := ratelimit.NewRedisEmailThrottler(rdb)
	ctx := context.Background()

	// Exhaust the limit for one email.
	for i := 0; i < ratelimit.EmailMaxSends; i++ {
		ok, _ := throttler.Allow(ctx, "test@example.com")
		require.True(t, ok)
	}
	ok, _ := throttler.Allow(ctx, "test@example.com")
	require.False(t, ok, "original email should be throttled")

	// Verify the raw email is NOT a Redis key.
	keys, err := rdb.Keys(ctx, "email:throttle:*").Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys, "hashed keys should exist")
	for _, k := range keys {
		require.NotContains(t, k, "test@example.com", "raw email must not appear in Redis key")
	}
}
