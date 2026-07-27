// Package testutil provides testcontainers-go harnesses for integration tests.
//
// Integration tests run against real PostgreSQL + Redis inside ephemeral
// Docker containers (testcontainers-go). This is the TDD §11 priority: the
// DB-touching paths (order/stock, webhook idempotency, RBAC, FX) need real
// PG/Redis to catch schema drift + transaction bugs that hand-written mocks
// cannot surface.
//
// Design:
//   - One shared Postgres container per `go test` run (sync.Once). A per-test
//     database is CREATEd, all migrations applied, and DROPped on cleanup —
//     fast (~50ms/test after the first container start) and hermetic.
//   - One shared Redis container; per-test DB index (0..15) + FLUSHDB on
//     cleanup.
//   - Migrations are embedded into the binary (internal/migrations/embed.go)
//     and applied via golang-migrate's Go API — no `migrate` CLI dependency.
//
// Tests opt out with testing.Short() (e.g. `go test -short` runs only unit
// tests). Docker must be reachable on the host; if it isn't, the first call
// t.Fatalfs with a clear message.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx5 source driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"jingdezhen-ceramics-backend/internal/migrations"
)

// Reuse across packages: testcontainers-go labels containers and, when Ryuk is
// enabled (the dev default), reaps them after the run. In CI set
// TESTCONTAINERS_RYUK_DISABLED=true on an ephemeral runner.

var (
	pgOnce     sync.Once
	sharedPG   *tcpg.PostgresContainer
	pgErr      error
	pgAdminCfg *pgx.ConnConfig // cached admin conn config (maintenance DB)

	redisOnce   sync.Once
	sharedRedis *tcredis.RedisContainer
	redisErr    error
	redisNextDB int // round-robins DB indexes 0..15 across tests
	redisMu     sync.Mutex
)

// -----------------------------------------------------------------------------
// PostgreSQL
// -----------------------------------------------------------------------------

// startSharedPG lazily starts one Postgres container for the whole test binary.
// The container image matches the dev docker-compose (postgis/postgis) so
// tests cannot drift from dev even though no current migration uses PostGIS
// yet (TDD §3.4 reserves destinations lat/lng + OSM maps).
func startSharedPG(ctx context.Context) (*tcpg.PostgresContainer, error) {
	pgOnce.Do(func() {
		image := "postgis/postgis:16-3.5-alpine"
		// Allow overriding the image for a faster pull on bare CI runners that
		// don't need PostGIS: TESTUTIL_PG_IMAGE=postgres:16-alpine.
		if v := os.Getenv("TESTUTIL_PG_IMAGE"); v != "" {
			image = v
		}
		pg, err := tcpg.Run(ctx, image,
			tcpg.WithDatabase("testserver"),
			tcpg.WithUsername("test"),
			tcpg.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(60*time.Second)),
		)
		if err != nil {
			pgErr = fmt.Errorf("testutil: start postgres container: %w", err)
			return
		}
		sharedPG = pg
	})
	return sharedPG, pgErr
}

// NewDBPool returns an isolated, fully-migrated *pgxpool.Pool for one test.
// A fresh database is created, all up-migrations applied, and the DB dropped
// on t.Cleanup. Tests are hermetic: no shared rows, no ordering dependency.
//
// Use this as the single entry point for any integration test that needs PG.
// Repositories take a *pgxpool.Pool, so the returned pool drops straight into
// `product.NewRepository(pool)`, `order.NewRepository(pool)`, etc.
func NewDBPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}

	pg, err := startSharedPG(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Admin conn (to the maintenance DB) for CREATE/DROP of per-test DBs.
	adminStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("testutil: postgres connection string: %v", err)
	}
	if pgAdminCfg == nil {
		cfg, err := pgx.ParseConfig(adminStr)
		if err != nil {
			t.Fatalf("testutil: parse admin config: %v", err)
		}
		pgAdminCfg = cfg
	}
	adminConn, err := pgx.ConnectConfig(ctx, pgAdminCfg)
	if err != nil {
		t.Fatalf("testutil: connect admin: %v", err)
	}
	t.Cleanup(func() { _ = adminConn.Close(ctx) })

	// Per-test DB name (Postgres identifiers are [a-z0-9_]; sanitize t.Name()
	// which contains "/" for subtests + "/" prefix for parallel).
	dbName := "t_" + sanitizeIdent(t.Name())
	execOrFatal(t, adminConn, ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q`, dbName))
	execOrFatal(t, adminConn, ctx, fmt.Sprintf(`CREATE DATABASE %q`, dbName))

	// Per-test conn string = admin URL with the DB path swapped.
	testStr, err := withDatabase(adminStr, dbName)
	if err != nil {
		t.Fatalf("testutil: rewrite conn string: %v", err)
	}

	// golang-migrate's pgx5 driver registers under the "pgx5" scheme (not
	// "postgres"), so the URL migrate sees must use pgx5://. pgxpool accepts
	// both, so only the migrate URL is rewritten.
	migrateStr := withScheme(testStr, "pgx5")

	// Apply migrations via golang-migrate driven by the embedded FS. The pgx5
	// source driver parses the URL + creates the schema_migrations table.
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("testutil: iofs source: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateStr)
	if err != nil {
		t.Fatalf("testutil: migrate.New: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("testutil: migrate.Up: %v", err)
	}
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("testutil: migrate.Close: src=%v db=%v", srcErr, dbErr)
	}

	// The pool the repositories expect.
	pool, err := pgxpool.New(ctx, testStr)
	if err != nil {
		t.Fatalf("testutil: pgxpool.New: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		// Force-close any stragglers on the per-test DB before dropping it,
		// else DROP DATABASE fails with "database is being accessed".
		adminConn2, _ := pgx.ConnectConfig(ctx, pgAdminCfg)
		if adminConn2 != nil {
			defer adminConn2.Close(ctx)
			_, _ = adminConn2.Exec(ctx, fmt.Sprintf(
				`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='%s'`, dbName))
			_, _ = adminConn2.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q`, dbName))
		}
	})
	return pool
}

// -----------------------------------------------------------------------------
// Redis
// -----------------------------------------------------------------------------

func startSharedRedis(ctx context.Context) (*tcredis.RedisContainer, error) {
	redisOnce.Do(func() {
		r, err := tcredis.Run(ctx, "redis:7-alpine",
			testcontainers.WithWaitStrategy(
				wait.ForLog("* Ready to accept connections").
					WithStartupTimeout(30*time.Second)),
		)
		if err != nil {
			redisErr = fmt.Errorf("testutil: start redis container: %w", err)
			return
		}
		sharedRedis = r
	})
	return sharedRedis, redisErr
}

// NewRedisClient returns an isolated Redis client. Each test gets a unique DB
// index (0..15) and the DB is FLUSHed on cleanup. Sufficient for Asynq + cache
// tests; switch to per-test containers if a test asserts cross-DB pub/sub.
func NewRedisClient(t testing.TB) *redis.Client {
	t.Helper()
	ctx := context.Background()
	if testing.Short() {
		t.Skip("integration test skipped under -short")
	}
	r, err := startSharedRedis(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	endpoint, err := r.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("testutil: redis endpoint: %v", err)
	}
	redisMu.Lock()
	dbIndex := redisNextDB % 16
	redisNextDB++
	redisMu.Unlock()

	cli := redis.NewClient(&redis.Options{Addr: endpoint, DB: dbIndex})
	if err := cli.Ping(ctx).Err(); err != nil {
		t.Fatalf("testutil: redis ping: %v", err)
	}
	t.Cleanup(func() {
		_ = cli.FlushDB(ctx).Err()
		_ = cli.Close()
	})
	return cli
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// withScheme returns u.String() with the URL scheme replaced by `scheme`
// (used to swap postgres:// → pgx5:// for golang-migrate).
func withScheme(connStr, scheme string) string {
	u, err := url.Parse(connStr)
	if err != nil {
		// Should not happen — the string already parsed once upstream.
		return connStr
	}
	u.Scheme = scheme
	return u.String()
}

// withDatabase returns connStr with the URL path (database) replaced by dbName.
// connStr looks like `postgres://test:test@localhost:5432/testserver?sslmode=disable`.
func withDatabase(connStr, dbName string) (string, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", connStr, err)
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

// sanitizeIdent maps a test name to a valid Postgres identifier (a-z0-9_, ≤63).
// Subtest names arrive as "TestX/sub"; parallel as "TestX#01". Collapse both.
func sanitizeIdent(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, s)
	if len(s) > 60 {
		// Keep uniqueness on truncation by hashing the tail.
		s = s[:60]
	}
	return s
}

func execOrFatal(t testing.TB, conn *pgx.Conn, ctx context.Context, sql string) {
	t.Helper()
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("testutil: exec %q: %v", sql, err)
	}
}
