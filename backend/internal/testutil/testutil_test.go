package testutil_test

import (
	"context"
	"testing"

	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/stretchr/testify/require"
)

// TestNewDBPool_AppliesAllMigrations proves the harness works: a fresh per-test
// database has every migration table present and is unseeded. Run first when
// iterating on the harness; it surfaces Docker-not-reachable, embed errors,
// migration-order bugs, and per-test-DB cleanup failures in isolation.
func TestNewDBPool_AppliesAllMigrations(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()

	// Every migration should have applied. Spot-check a late table (product_tags,
	// migration 000021 — the last landed) + an early one (orders, migration 000017).
	for _, tt := range []struct{ table, mig string }{
		{"orders", "000017"},
		{"product_tags", "000021"},
		{"tag_translations", "000021"},
		{"media_assets", "000020"},
		{"certificates", "000019"},
		{"payments", "000018"},
	} {
		var n int
		err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`,
			tt.table).Scan(&n)
		require.NoError(t, err, "querying for %s (mig %s)", tt.table, tt.mig)
		require.Equalf(t, 1, n, "table %s (mig %s) missing — migrations did not fully apply",
			tt.table, tt.mig)
	}

	// The test DB is unseeded — testutil applies migrations, not seed files.
	// Seed rows are inserted by each test via direct INSERT (hermetic, no shared
	// fixtures), so the test owns exactly the data it needs.
	var userCount int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount)
	require.NoError(t, err)
	require.Zero(t, userCount, "test DB should be unseeded (no users)")
}

// TestNewDBPool_IsolationBetweenTests asserts two tests get independent DBs —
// rows created in one do not leak into the other. This is the hermeticity
// guarantee the harness sells; if it ever regresses, every later test becomes
// order-dependent.
func TestNewDBPool_IsolationBetweenTests(t *testing.T) {
	t.Run("writes_a_user", func(t *testing.T) {
		pool := testutil.NewDBPool(t)
		ctx := context.Background()
		_, err := pool.Exec(ctx, `INSERT INTO users (id, email, is_active, auth_provider)
			VALUES ('00000000-0000-0000-0000-000000000001','a@t.test',true,'email')`)
		require.NoError(t, err)
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n))
		require.Equal(t, 1, n)
	})
	t.Run("sees_no_users", func(t *testing.T) {
		pool := testutil.NewDBPool(t)
		ctx := context.Background()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n))
		require.Zero(t, n, "subtest B saw subtest A's rows — DB isolation broke")
	})
}

func TestNewRedisClient_Pings(t *testing.T) {
	cli := testutil.NewRedisClient(t)
	require.NoError(t, cli.Ping(context.Background()).Err())
}

func TestNewRedisClient_IsolationBetweenTests(t *testing.T) {
	t.Run("writes_a_key", func(t *testing.T) {
		cli := testutil.NewRedisClient(t)
		ctx := context.Background()
		require.NoError(t, cli.Set(ctx, "k", "v", 0).Err())
		v, err := cli.Get(ctx, "k").Result()
		require.NoError(t, err)
		require.Equal(t, "v", v)
	})
	t.Run("sees_no_keys", func(t *testing.T) {
		cli := testutil.NewRedisClient(t)
		ctx := context.Background()
		n, err := cli.DBSize(ctx).Result()
		require.NoError(t, err)
		require.Zero(t, n, "subtest B saw subtest A's keys — Redis DB isolation broke")
	})
}
