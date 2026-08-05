package tokenblocklist

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopBlocklist(t *testing.T) {
	bl := NoopBlocklist{}
	ctx := context.Background()
	require.NoError(t, bl.Revoke(ctx, "user-1", MaxAccessTokenTTL))
	revoked, err := bl.IsRevoked(ctx, "user-1")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestRedisBlocklist_RevokeAndCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	bl := NewRedisBlocklist(client)
	ctx := context.Background()

	// Unknown user → not revoked.
	revoked, err := bl.IsRevoked(ctx, "u-unknown")
	require.NoError(t, err)
	assert.False(t, revoked)

	// Revoke → revoked.
	require.NoError(t, bl.Revoke(ctx, "u-deleted", MaxAccessTokenTTL))
	revoked, err = bl.IsRevoked(ctx, "u-deleted")
	require.NoError(t, err)
	assert.True(t, revoked)

	// Idempotent: re-revoke refreshes TTL, still revoked.
	require.NoError(t, bl.Revoke(ctx, "u-deleted", MaxAccessTokenTTL))
	revoked, err = bl.IsRevoked(ctx, "u-deleted")
	require.NoError(t, err)
	assert.True(t, revoked)

	// Different user unaffected.
	revoked, err = bl.IsRevoked(ctx, "u-other")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestRedisBlocklist_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	client := testutil.NewRedisClient(t)
	bl := NewRedisBlocklist(client)
	ctx := context.Background()

	// Revoke with a very short TTL; after expiry the key is gone.
	require.NoError(t, bl.Revoke(ctx, "u-short", 1500*time.Millisecond))
	revoked, err := bl.IsRevoked(ctx, "u-short")
	require.NoError(t, err)
	assert.True(t, revoked)

	// Poll until the key expires (ttl + slack).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		revoked, err = bl.IsRevoked(ctx, "u-short")
		require.NoError(t, err)
		if !revoked {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	assert.False(t, revoked, "revocation key should expire after TTL")
}

func TestRedisBlocklist_NilClientIsNoop(t *testing.T) {
	bl := NewRedisBlocklist(nil)
	ctx := context.Background()
	require.NoError(t, bl.Revoke(ctx, "u-1", MaxAccessTokenTTL))
	revoked, err := bl.IsRevoked(ctx, "u-1")
	require.NoError(t, err)
	assert.False(t, revoked, "nil client behaves as Noop")
}

func TestRedisBlocklist_FailOpenOnUnreachableRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	// Point at a closed client to simulate a Redis outage.
	client := testutil.NewRedisClient(t)
	require.NoError(t, client.Close()) // close immediately → commands error

	bl := NewRedisBlocklist(client)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Revoke surfaces the error (caller decides); IsRevoked fail-opens.
	err := bl.Revoke(ctx, "u-outage", MaxAccessTokenTTL)
	assert.Error(t, err)

	revoked, err := bl.IsRevoked(ctx, "u-outage")
	require.NoError(t, err, "fail-open: never return an error on read path")
	assert.False(t, revoked)
}
