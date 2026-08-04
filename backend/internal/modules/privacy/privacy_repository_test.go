package privacy_test

// Integration tests for the privacy (GDPR erasure) repository against real
// PostgreSQL (testcontainers-go). The invariant under test (TDD §3.4 +
// migration 000010): erasure is anonymize-in-place, NOT a hard delete, so
// `orders.user_id` (FK with NO ACTION) retains referential integrity — order
// history survives erasure for audit/retention.
//
// A regression here is a compliance bug:
//   - If AnonymizeUser ever CASCADE-deleted orders, the platform loses order
//     history it's legally required to retain.
//   - If the FK broke (e.g. someone "fixed" it to SET NULL), orders would
//     lose their customer link.
// This test pins the migration's NO ACTION + the code's anonymize-in-place
// behavior together, against real PG.

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/privacy"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// seedUserWithOrder inserts a user + an order owned by that user, returning
// both ids. The order is the thing that must survive erasure.
func seedUserWithOrder(t *testing.T, pool *pgxpool.Pool) (userID string, orderID int64) {
	t.Helper()
	ctx := context.Background()
	err := pool.QueryRow(ctx, `INSERT INTO users (email, nickname, is_active, auth_provider)
		VALUES ('erase@t.test', 'Erase Me', true, 'email') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `INSERT INTO orders (user_id, currency, subtotal_minor, total_minor,
		subtotal_cny, total_cny, address) VALUES ($1,'USD',10000,10000,10000,10000,'{}'::jsonb)
		RETURNING id`, userID).Scan(&orderID)
	require.NoError(t, err)
	return userID, orderID
}

// TestAnonymizeUser_PreservesOrderHistory is the GDPR invariant: after
// erasure, the user row is anonymized (email/nickname NULL, is_active false,
// deleted_at set) BUT the order still exists and still references the same
// user_id (NO ACTION FK — no cascade, no set-null). This is what
// "anonymize-in-place" buys: retention without PII.
func TestAnonymizeUser_PreservesOrderHistory(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := privacy.NewRepository(pool)
	userID, orderID := seedUserWithOrder(t, pool)

	// Erase.
	require.NoError(t, repo.AnonymizeUser(ctx, userID))

	// The order still exists and still points at the same user_id.
	var orderUserID string
	var orderCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT user_id FROM orders WHERE id=$1`, orderID).Scan(&orderUserID))
	require.Equal(t, userID, orderUserID, "order.user_id must still reference the (anonymized) user stub — NO ACTION FK")
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id=$1`, userID).Scan(&orderCount))
	require.Equal(t, 1, orderCount, "the order must not have been cascade-deleted by erasure")

	// The user stub is anonymized: PII nulled, is_active=false, deleted_at set.
	var email, nickname *string
	var isActive bool
	var deletedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT email, nickname, is_active, deleted_at FROM users WHERE id=$1`, userID).
		Scan(&email, &nickname, &isActive, &deletedAt))
	require.Nil(t, email, "email must be NULL after erasure")
	require.Nil(t, nickname, "nickname must be NULL after erasure")
	require.False(t, isActive, "is_active must be false (login rejected)")
	require.NotNil(t, deletedAt, "deleted_at must be set (the erasure timestamp)")

	// IsDeleted reports true (the login gate + handler use this).
	deleted, err := repo.IsDeleted(ctx, userID)
	require.NoError(t, err)
	require.True(t, deleted)
}

// TestAnonymizeUser_IdempotentOnReErase asserts a second erasure on an
// already-anonymized user returns ErrAccountDeleted (not ErrNotFound, not a
// no-op, not a second mutation). The handler maps this to a 409 so a client
// retry after erasure gets a clear "already gone" signal.
func TestAnonymizeUser_IdempotentOnReErase(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := privacy.NewRepository(pool)
	userID, _ := seedUserWithOrder(t, pool)

	require.NoError(t, repo.AnonymizeUser(ctx, userID))
	err := repo.AnonymizeUser(ctx, userID)
	require.ErrorIs(t, err, models.ErrAccountDeleted, "re-erasing an already-anonymized user must return ErrAccountDeleted")
}

// TestAnonymizeUser_NonexistentUser asserts erasing a user that never existed
// returns ErrNotFound (not ErrAccountDeleted — the distinction matters for
// the handler's status code: 404 vs 409).
func TestAnonymizeUser_NonexistentUser(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := privacy.NewRepository(pool)

	err := repo.AnonymizeUser(ctx, "00000000-0000-0000-0000-000000000999")
	require.ErrorIs(t, err, models.ErrNotFound)
}
