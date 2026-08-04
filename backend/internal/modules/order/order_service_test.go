package order_test

// Integration tests for the order repository against real PostgreSQL
// (testcontainers-go). These are the TDD §11 priority tests: the DB-touching
// paths where a schema drift or a transaction bug would silently corrupt
// money/stock. The unit-test layer (handler/service with mocked repos) can't
// catch these — only real PG can.
//
// Pattern (copy this for every integration test):
//  1. pool := testutil.NewDBPool(t)   — fresh, migrated, unseeded DB.
//  2. Seed the minimal rows the test needs via direct INSERT (no shared fixture
//     files — each test owns exactly its data, so failures are local).
//  3. Build the real repository with the pool: order.NewRepository(pool).
//  4. Exercise the path under test + assert on rows re-read from the DB (not
//     on the in-memory return value alone — the DB is the source of truth).
//  5. t.Cleanup (registered inside NewDBPool) drops the per-test DB.

import (
	"context"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/order"
	"jingdezhen-ceramics-backend/internal/testutil"

	"strconv"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// uniqueSlug returns a short unique suffix so seeded slugs / sku codes don't
// collide on the UNIQUE constraints when a test creates multiple rows.
var slugCounter uint64

func uniqueSlug() string {
	return strconv.FormatUint(atomic.AddUint64(&slugCounter, 1), 36)
}

// seedProductAndSKU inserts one product (en-US published) + one SKU with the
// given stock and returns the sku id. Tests call this so the order's FK
// targets exist; the exact ids don't matter (BIGSERIAL), we RETURNING them.
func seedProductAndSKU(t *testing.T, pool *pgxpool.Pool, stock int) (productID, skuID int64) {
	t.Helper()
	ctx := context.Background()
	// Unique slug per call — product_translations has UNIQUE(locale, slug).
	// Use a counter so two products in the same test don't collide.
	slug := "test-product-" + uniqueSlug()
	err := pool.QueryRow(ctx, `
		INSERT INTO products (category, display_order)
		VALUES ('test', 0) RETURNING id`).Scan(&productID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO product_translations (product_id, locale, title, slug, status, published_at)
		VALUES ($1, 'en-US', 'Test Product', $2, 'published', NOW())`, productID, slug)
	require.NoError(t, err)
	// Unique SKU code too (skus.sku_code is UNIQUE).
	skuCode := "SKU-T-" + uniqueSlug()
	err = pool.QueryRow(ctx, `
		INSERT INTO skus (product_id, sku_code, price_cny, stock, weight_grams, attributes, is_active)
		VALUES ($1, $2, 10000, $3, 500, '{}'::jsonb, true) RETURNING id`,
		productID, skuCode, stock).Scan(&skuID)
	require.NoError(t, err)
	return productID, skuID
}

// seedUser inserts a minimal user row (orders.user_id is a FK to users) and
// returns its id. Tests need this because the order row carries user_id.
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, is_active, auth_provider)
		VALUES ($1, true, 'email') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestCreateOrder_AtomicStockDecrement verifies the TDD §4.3 invariant:
// CreateOrder decrements stock inside the order-creation tx, and an
// insufficient-stock SKU rolls back the WHOLE order (no partial order, no
// partial stock change). This is the bug class that manual testing can't
// catch deterministically — a tx bug here means oversold inventory.
func TestCreateOrder_AtomicStockDecrement(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := order.NewRepository(pool)

	_, skuID := seedProductAndSKU(t, pool, 2) // stock = 2
	userID := seedUser(t, pool, "buyer@t.test")

	// --- Happy path: buy 2 (exact stock) → succeeds, stock → 0. ---
	o := &models.Order{
		UserID: userID, Currency: "USD",
		SubtotalMinor: 20000, ShippingMinor: 1000, TotalMinor: 21000,
		SubtotalCNY: 20000, ShippingCNY: 1000, TotalCNY: 21000,
		Address: []byte(`{}`),
	}
	items := []models.OrderItem{{SkuID: skuID, Qty: 2, UnitPriceMinor: 10000, UnitPriceCNY: 10000}}
	orderID, err := repo.CreateOrder(ctx, o, items)
	require.NoError(t, err)
	require.NotZero(t, orderID)

	var stockAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM skus WHERE id=$1`, skuID).Scan(&stockAfter))
	require.Equal(t, 0, stockAfter, "stock should be decremented to 0 after buying all 2")

	// --- Failure path: buy 1 more → ErrConflict, stock unchanged, NO new order. ---
	o2 := &models.Order{
		UserID: userID, Currency: "USD",
		SubtotalMinor: 10000, ShippingMinor: 1000, TotalMinor: 11000,
		SubtotalCNY: 10000, ShippingCNY: 1000, TotalCNY: 11000,
		Address: []byte(`{}`),
	}
	_, err = repo.CreateOrder(ctx, o2, []models.OrderItem{{SkuID: skuID, Qty: 1, UnitPriceMinor: 10000, UnitPriceCNY: 10000}})
	require.ErrorIs(t, err, models.ErrConflict, "oversell attempt must return ErrConflict")

	// The DB is the source of truth: stock unchanged, only one order exists.
	require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM skus WHERE id=$1`, skuID).Scan(&stockAfter))
	require.Equal(t, 0, stockAfter, "stock must NOT change on a failed (rolled-back) order")

	var orderCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orderCount))
	require.Equal(t, 1, orderCount, "the failed order must not have been inserted (tx rolled back)")
}

// TestCreateOrder_RollsBackOnAnyItemInsufficient extends the invariant to a
// multi-item order: if the SECOND item is out of stock, the FIRST item's stock
// decrement must also roll back (the whole tx, not just the failing line).
// This is the partial-order bug the tx is specifically designed to prevent.
func TestCreateOrder_RollsBackOnAnyItemInsufficient(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := order.NewRepository(pool)

	_, skuA := seedProductAndSKU(t, pool, 5) // plenty
	_, skuB := seedProductAndSKU(t, pool, 1) // only 1 in stock
	userID := seedUser(t, pool, "buyer2@t.test")

	o := &models.Order{
		UserID: userID, Currency: "USD",
		SubtotalMinor: 30000, ShippingMinor: 1000, TotalMinor: 31000,
		SubtotalCNY: 30000, ShippingCNY: 1000, TotalCNY: 31000,
		Address: []byte(`{}`),
	}
	// Item 1 (qty 3 of A, stock 5 — fine) + Item 2 (qty 2 of B, stock 1 — fails).
	items := []models.OrderItem{
		{SkuID: skuA, Qty: 3, UnitPriceMinor: 10000, UnitPriceCNY: 10000},
		{SkuID: skuB, Qty: 2, UnitPriceMinor: 10000, UnitPriceCNY: 10000},
	}
	_, err := repo.CreateOrder(ctx, o, items)
	require.ErrorIs(t, err, models.ErrConflict)

	// Both stock decrements must roll back: A still 5, B still 1.
	var stockA, stockB int
	require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM skus WHERE id=$1`, skuA).Scan(&stockA))
	require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM skus WHERE id=$1`, skuB).Scan(&stockB))
	require.Equal(t, 5, stockA, "item A's decrement must roll back when item B fails (whole-tx rollback)")
	require.Equal(t, 1, stockB, "item B's stock must be unchanged")

	var orderCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orderCount))
	require.Zero(t, orderCount, "no order should exist after a rolled-back multi-item checkout")
}

// TestSetCancelled_RestoresStock verifies the cancel path restores stock
// atomically (created → cancelled, stock back). Pairs with the decrement test
// to cover the full stock lifecycle: decrement at checkout, restore on cancel.
func TestSetCancelled_RestoresStock(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := order.NewRepository(pool)

	_, skuID := seedProductAndSKU(t, pool, 3) // stock = 3
	userID := seedUser(t, pool, "buyer3@t.test")

	// Buy 2 → stock 1.
	o := &models.Order{
		UserID: userID, Currency: "USD",
		SubtotalMinor: 20000, ShippingMinor: 1000, TotalMinor: 21000,
		SubtotalCNY: 20000, ShippingCNY: 1000, TotalCNY: 21000,
		Address: []byte(`{}`),
	}
	items := []models.OrderItem{{SkuID: skuID, Qty: 2, UnitPriceMinor: 10000, UnitPriceCNY: 10000}}
	orderID, err := repo.CreateOrder(ctx, o, items)
	require.NoError(t, err)

	// Cancel → stock restored to 3.
	err = repo.SetCancelled(ctx, orderID, "changed mind", items)
	require.NoError(t, err)

	var stock int
	require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM skus WHERE id=$1`, skuID).Scan(&stock))
	require.Equal(t, 3, stock, "cancel must restore the decremented stock")

	// The order status reflects the transition (re-read, don't trust the return).
	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, orderID).Scan(&status))
	require.Equal(t, string(models.StatusCancelled), status)
}

// TestTransitionStatus_IdempotentMarkPaid verifies the webhook-idempotency
// invariant (TDD §11): MarkPaid via TransitionStatus(created→paid) returns
// ErrConflict on a second call (replayed webhook), and the service layer
// treats that as a no-op success. The repo itself surfaces ErrConflict so the
// service can distinguish "already paid" from "absent".
func TestTransitionStatus_IdempotentMarkPaid(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := order.NewRepository(pool)

	_, skuID := seedProductAndSKU(t, pool, 1)
	userID := seedUser(t, pool, "buyer4@t.test")
	o := &models.Order{UserID: userID, Currency: "USD", SubtotalMinor: 10000, TotalMinor: 10000,
		SubtotalCNY: 10000, TotalCNY: 10000, Address: []byte(`{}`)}
	orderID, err := repo.CreateOrder(ctx, o, []models.OrderItem{{SkuID: skuID, Qty: 1, UnitPriceMinor: 10000, UnitPriceCNY: 10000}})
	require.NoError(t, err)

	// First created→paid: succeeds.
	require.NoError(t, repo.TransitionStatus(ctx, orderID, models.StatusCreated, models.StatusPaid, ""))

	// Replay (replayed webhook): ErrConflict, NOT a second transition.
	err = repo.TransitionStatus(ctx, orderID, models.StatusCreated, models.StatusPaid, "")
	require.ErrorIs(t, err, models.ErrConflict, "a replayed transition must surface ErrConflict so the service can no-op it")

	// paid_at was set once and is not advanced by the replay.
	var paidCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE id=$1 AND status='paid' AND paid_at IS NOT NULL`, orderID).Scan(&paidCount))
	require.Equal(t, 1, paidCount)

	// Absent order → ErrNotFound (not ErrConflict) so the service can 404.
	err = repo.TransitionStatus(ctx, orderID+99999, models.StatusCreated, models.StatusPaid, "")
	require.ErrorIs(t, err, models.ErrNotFound)
}
