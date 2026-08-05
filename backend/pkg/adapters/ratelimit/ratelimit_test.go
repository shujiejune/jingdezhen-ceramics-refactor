package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/testutil"
	"jingdezhen-ceramics-backend/pkg/adapters/ratelimit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopAttemptTracker_NeverLocks(t *testing.T) {
	n := ratelimit.NoopAttemptTracker{}
	ctx := context.Background()
	for i := 0; i < ratelimit.MaxFailures+5; i++ {
		require.NoError(t, n.RegisterFailure(ctx, "u"))
	}
	locked, err := n.IsLocked(ctx, "u")
	require.NoError(t, err)
	assert.False(t, locked)
	require.NoError(t, n.Reset(ctx, "u"))
}

func TestRedisAttemptTracker_LocksAfterMaxFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	tr := ratelimit.NewRedisAttemptTracker(client)
	ctx := context.Background()
	const key = "lock-test"

	// MaxFailures-1 failures → not locked yet.
	for i := 0; i < ratelimit.MaxFailures-1; i++ {
		require.NoError(t, tr.RegisterFailure(ctx, key))
	}
	locked, err := tr.IsLocked(ctx, key)
	require.NoError(t, err)
	assert.False(t, locked, "one short of threshold: not locked")

	// MaxFailures-th failure trips the lock.
	require.NoError(t, tr.RegisterFailure(ctx, key))
	locked, err = tr.IsLocked(ctx, key)
	require.NoError(t, err)
	assert.True(t, locked, "threshold reached: locked")
}

func TestRedisAttemptTracker_SuccessResetsCounter(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	tr := ratelimit.NewRedisAttemptTracker(client)
	ctx := context.Background()
	const key = "reset-test"

	for i := 0; i < ratelimit.MaxFailures-1; i++ {
		require.NoError(t, tr.RegisterFailure(ctx, key))
	}
	// A successful verify resets the counter + clears any (absent) lock.
	require.NoError(t, tr.Reset(ctx, key))

	// After reset, the next MaxFailures-1 attempts must NOT lock (counter
	// restarted from 0).
	for i := 0; i < ratelimit.MaxFailures-1; i++ {
		require.NoError(t, tr.RegisterFailure(ctx, key))
	}
	locked, err := tr.IsLocked(ctx, key)
	require.NoError(t, err)
	assert.False(t, locked, "reset cleared the counter; not locked")
}

func TestRedisAttemptTracker_LockSurvivesCounterTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	// Build a tracker with a tiny failure window so the counter key expires
	// almost immediately, but a normal lockout duration. The lock must still
	// hold (it has its own TTL independent of the counter).
	client := testutil.NewRedisClient(t)
	tr := ratelimit.NewRedisAttemptTracker(client)
	ctx := context.Background()
	const key = "ttl-test"

	// Reach threshold.
	for i := 0; i < ratelimit.MaxFailures; i++ {
		require.NoError(t, tr.RegisterFailure(ctx, key))
	}
	locked, err := tr.IsLocked(ctx, key)
	require.NoError(t, err)
	require.True(t, locked)

	// Manually expire the counter key (simulate window elapsing) — the lock
	// key has its own TTL and survives.
	require.NoError(t, client.Del(ctx, "2fa:fail:"+key).Err())
	locked, err = tr.IsLocked(ctx, key)
	require.NoError(t, err)
	assert.True(t, locked, "lock key independent of counter TTL")
}

func TestRedisAttemptTracker_FailOpenOnRedisOutage(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	tr := ratelimit.NewRedisAttemptTracker(client)
	ctx := context.Background()

	// Close the client → all ops fail. IsLocked must fail-open (false, err-ish
	// from the call but the contract returns an error — the SERVICE layer
	// treats it as fail-open; here we assert the RegisterFailure/Reset are
	// best-effort and IsLocked surfaces the error rather than panicking).
	require.NoError(t, client.Close())

	// RegisterFailure + Reset are best-effort (return nil even on closed Redis).
	require.NoError(t, tr.RegisterFailure(ctx, "u-outage"))
	require.NoError(t, tr.Reset(ctx, "u-outage"))

	// IsLocked returns an error on Redis outage; the caller (user service)
	// logs + treats as fail-open. Assert it doesn't panic + returns false-ish.
	_, err := tr.IsLocked(ctx, "u-outage")
	assert.Error(t, err, "IsLocked surfaces Redis errors for the caller to log")
}

func TestRedisAttemptTracker_NilClientNoop(t *testing.T) {
	tr := ratelimit.NewRedisAttemptTracker(nil)
	ctx := context.Background()
	require.NoError(t, tr.RegisterFailure(ctx, "u"))
	locked, err := tr.IsLocked(ctx, "u")
	require.NoError(t, err)
	assert.False(t, locked)
	require.NoError(t, tr.Reset(ctx, "u"))
}

// _ keeps the time import alive (LockoutDuration referenced via the consts).
var _ = time.Minute
