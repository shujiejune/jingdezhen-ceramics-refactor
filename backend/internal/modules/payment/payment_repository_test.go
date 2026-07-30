package payment_test

// Integration tests for the payment repository against real PostgreSQL
// (testcontainers-go). These are the TDD §11 priority #2: the DB side of
// webhook idempotency. The unit test in payment_service_test.go proves the
// service no-ops on a replayed event (control flow); THIS test proves the
// database actually enforces it — the `payments.idempotency_key UNIQUE`
// constraint + the `xmax = 0` probe that distinguishes "just inserted" from
// "ON CONFLICT updated an existing row".
//
// Why both layers? A replayed webhook is a double-charge / double-transition
// risk. The control flow says "if UpsertWebhook returns inserted=false, don't
// enqueue finalize again"; the DB says "a second insert with the same key is
// not a second row". Either layer alone is insufficient: the service logic
// could be right while the DB silently allows a duplicate, or the DB could be
// right while the service ignores the inserted flag. Testing both is what
// makes the idempotency guarantee trustworthy.
//
// Pattern matches order_service_test.go: NewDBPool(t) → seed minimal rows →
// build the real repo → exercise + assert on rows re-read from the DB.

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/payment"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// seedOrderForPayment inserts a minimal user + order row so the payments FK
// (order_id → orders.id → users.id) is satisfiable. Returns the order id.
// Payments exist in the context of an order; we don't care about the order's
// contents here, only that the row exists.
func seedOrderForPayment(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var userID string
	err := pool.QueryRow(ctx, `INSERT INTO users (email, is_active, auth_provider)
		VALUES ('pay@t.test', true, 'email') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	var orderID int64
	err = pool.QueryRow(ctx, `INSERT INTO orders (user_id, currency, subtotal_minor, total_minor,
		subtotal_cny, total_cny, address) VALUES ($1,'USD',10000,10000,10000,10000,'{}'::jsonb)
		RETURNING id`, userID).Scan(&orderID)
	require.NoError(t, err)
	return orderID
}

// TestUpsertWebhook_IdempotentReplay is the core TDD §11 invariant: a second
// UpsertWebhook with the same idempotency_key must return inserted=false AND
// create no second row. The xmax=0 probe is what distinguishes a fresh insert
// from an ON CONFLICT update; this test would catch a regression where the
// probe is removed or the UNIQUE constraint is dropped.
func TestUpsertWebhook_IdempotentReplay(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := payment.NewRepository(pool)
	orderID := seedOrderForPayment(t, pool)

	ev := &models.Payment{
		OrderID: orderID, Gateway: models.GatewayAirwallex, GatewayRef: "int_42",
		Status: models.PaymentSucceeded, AmountMinor: 10000, Currency: "USD",
		RawWebhook:     json.RawMessage(`{"event_id":"evt_1"}`),
		IdempotencyKey: "int_42_evt_1",
	}

	// First insert: a brand-new event → inserted=true.
	inserted, err := repo.UpsertWebhook(ctx, ev)
	require.NoError(t, err)
	require.True(t, inserted, "first webhook for a key must report inserted=true")

	// Replay: same idempotency_key → inserted=false, no error.
	inserted, err = repo.UpsertWebhook(ctx, ev)
	require.NoError(t, err)
	require.False(t, inserted, "a replayed webhook must report inserted=false (no-op)")

	// The DB is the source of truth: exactly ONE row for this key, regardless
	// of how many replays arrived. This is the double-charge guard.
	var rowCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE idempotency_key = $1`, ev.IdempotencyKey).Scan(&rowCount))
	require.Equal(t, 1, rowCount, "a replayed webhook must not create a second row")

	// A third replay too — the probe must be stable, not flaky.
	_, err = repo.UpsertWebhook(ctx, ev)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE idempotency_key = $1`, ev.IdempotencyKey).Scan(&rowCount))
	require.Equal(t, 1, rowCount)
}

// TestUpsertWebhook_DistinctKeysAreDistinctRows confirms the idempotency
// guard is keyed on idempotency_key specifically — two DIFFERENT events (e.g.
// `evt_1` then `evt_2`) both insert, because they're genuinely different
// gateway events, not a replay. This is the complement to the replay test: it
// guards against an over-broad dedupe that would swallow real distinct events.
func TestUpsertWebhook_DistinctKeysAreDistinctRows(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := payment.NewRepository(pool)
	orderID := seedOrderForPayment(t, pool)

	for i, key := range []string{"int_42_evt_A", "int_42_evt_B", "int_42_evt_C"} {
		ev := &models.Payment{
			OrderID: orderID, Gateway: models.GatewayAirwallex, GatewayRef: "int_42",
			Status: models.PaymentSucceeded, AmountMinor: 10000, Currency: "USD",
			RawWebhook:     json.RawMessage(`{"event_id":"` + key + `"}`),
			IdempotencyKey: key,
		}
		inserted, err := repo.UpsertWebhook(ctx, ev)
		require.NoError(t, err)
		require.Truef(t, inserted, "event %d (%s) is a distinct event, must insert", i, key)
	}
	var rowCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE idempotency_key LIKE 'int_42_evt_%'`).Scan(&rowCount))
	require.Equal(t, 3, rowCount, "three distinct idempotency keys → three rows")
}

// TestUpsertWebhook_ConcurrentReplaysOneWins is the hardest invariant: two
// goroutines insert the SAME idempotency_key at the same time. Exactly one
// must win (inserted=true); the other must get inserted=false (ON CONFLICT),
// NOT an error and NOT a second row. This is the concurrency case the
// UNIQUE constraint + ON CONFLICT DO UPDATE exists for — a webhook gateway
// under load will genuinely send the same event twice in parallel.
//
// Without the UNIQUE constraint, both inserts would succeed and the finalize
// job would enqueue twice (double-transition risk). Without the xmax probe,
// the service couldn't tell which one won. This test exercises both together.
func TestUpsertWebhook_ConcurrentReplaysOneWins(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := payment.NewRepository(pool)
	orderID := seedOrderForPayment(t, pool)

	key := "int_42_concurrent_evt"
	ev := &models.Payment{
		OrderID: orderID, Gateway: models.GatewayAirwallex, GatewayRef: "int_42",
		Status: models.PaymentSucceeded, AmountMinor: 10000, Currency: "USD",
		RawWebhook:     json.RawMessage(`{"event_id":"concurrent"}`),
		IdempotencyKey: key,
	}

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	results := make([]bool, goroutines)
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			// Each goroutine races to upsert the SAME event. Postgres serializes
			// via the UNIQUE index; exactly one ON CONFLICT path wins the insert.
			inserted, err := repo.UpsertWebhook(ctx, ev)
			results[i] = inserted
			errs[i] = err
		}()
	}
	wg.Wait()

	// None of the goroutines should error — ON CONFLICT must absorb the losers,
	// not surface a unique-violation.
	for i, err := range errs {
		require.NoErrorf(t, err, "goroutine %d errored on a concurrent replay", i)
	}

	// Exactly ONE goroutine reports inserted=true; the rest report false.
	winners := 0
	for _, inserted := range results {
		if inserted {
			winners++
		}
	}
	require.Equal(t, 1, winners, "exactly one of %d concurrent replays must win", goroutines)

	// The DB agrees: one row, not goroutines rows.
	var rowCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM payments WHERE idempotency_key = $1`, key).Scan(&rowCount))
	require.Equal(t, 1, rowCount, "concurrent replays of one event → one row (the double-charge guard)")
}

// TestUpsertWebhook_UpdatesIntentStatus confirms the upsert's other job: on a
// first-seen succeeded event, the matching pending INTENT row (created by
// RecordIntent at checkout) is transitioned to succeeded. A replay must NOT
// re-update it (the intent is already succeeded; idempotency). This pairs the
// event-row audit trail with the intent-row status that drives refunds.
func TestUpsertWebhook_UpdatesIntentStatus(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := payment.NewRepository(pool)
	orderID := seedOrderForPayment(t, pool)

	// Checkout creates a pending intent row.
	intent := &models.Payment{
		OrderID: orderID, Gateway: models.GatewayAirwallex, GatewayRef: "int_99",
		AmountMinor: 10000, Currency: "USD", IdempotencyKey: "intent_99",
	}
	intentID, err := repo.RecordIntent(ctx, intent)
	require.NoError(t, err)

	// First webhook: event has the same gateway_ref + a succeeded status + a
	// DISTINCT idempotency_key (the event key, not the intent key). The upsert
	// inserts the event row AND updates the pending intent → succeeded.
	ev := &models.Payment{
		OrderID: orderID, Gateway: models.GatewayAirwallex, GatewayRef: "int_99",
		Status: models.PaymentSucceeded, AmountMinor: 10000, Currency: "USD",
		RawWebhook:     json.RawMessage(`{"event_id":"evt_99"}`),
		IdempotencyKey: "int_99_evt_99",
	}
	inserted, err := repo.UpsertWebhook(ctx, ev)
	require.NoError(t, err)
	require.True(t, inserted)

	// The intent row (by id) should now be succeeded.
	got, err := repo.GetByID(ctx, intentID)
	require.NoError(t, err)
	require.Equal(t, models.PaymentSucceeded, got.Status, "the pending intent must be advanced to succeeded")

	// GetSucceededByOrderID now finds it (this is the lookup Refund uses).
	succ, err := repo.GetSucceededByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, intentID, succ.ID)

	// Replay the same event: inserted=false AND the intent status is untouched
	// (still succeeded — not re-processed, not errored).
	inserted, err = repo.UpsertWebhook(ctx, ev)
	require.NoError(t, err)
	require.False(t, inserted)
	got, err = repo.GetByID(ctx, intentID)
	require.NoError(t, err)
	require.Equal(t, models.PaymentSucceeded, got.Status, "a replay must not re-advance the intent")
}

// TestSetRefunded_CAS confirms refund is a CAS on status='succeeded': succeeds
// once, then a second refund (or refund on a non-succeeded row) is ErrConflict.
// This is the fail-closed guard (TDD §4.3) — a double-refund would refund the
// customer twice for one payment.
func TestSetRefunded_CAS(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := payment.NewRepository(pool)
	orderID := seedOrderForPayment(t, pool)

	intent := &models.Payment{
		OrderID: orderID, Gateway: models.GatewayMock, GatewayRef: "int_77",
		AmountMinor: 10000, Currency: "USD", IdempotencyKey: "intent_77",
	}
	intentID, err := repo.RecordIntent(ctx, intent)
	require.NoError(t, err)

	// Refund before succeeded → ErrConflict (can't refund a pending payment).
	err = repo.SetRefunded(ctx, intentID)
	require.ErrorIs(t, err, models.ErrConflict)

	// Advance to succeeded (via the webhook upsert, as in the test above).
	_, err = repo.UpsertWebhook(ctx, &models.Payment{
		OrderID: orderID, Gateway: models.GatewayMock, GatewayRef: "int_77",
		Status: models.PaymentSucceeded, AmountMinor: 10000, Currency: "USD",
		RawWebhook:     json.RawMessage(`{}`),
		IdempotencyKey: "int_77_evt",
	})
	require.NoError(t, err)

	// First refund on a succeeded row → ok.
	require.NoError(t, repo.SetRefunded(ctx, intentID))

	// Second refund → ErrConflict (status is now 'refunded', not 'succeeded').
	// This is the double-refund guard.
	err = repo.SetRefunded(ctx, intentID)
	require.ErrorIs(t, err, models.ErrConflict)
}
