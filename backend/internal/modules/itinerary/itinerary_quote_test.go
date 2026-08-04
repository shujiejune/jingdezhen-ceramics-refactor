package itinerary_test

// Integration tests for the quote builder + deposit payment (PRD §3.3.2, TDD
// §3.4 M3 #3). The bug-prone bits unique to this sub-track:
//   - SendQuote prices lines from option_rates (CNY) + FX-converts the total +
//     snapshots fx_rate_used (like orders); the request moves processing→quoted.
//   - CreateQuote replaces on re-send (UNIQUE request_id ON CONFLICT).
//   - PayDeposit (mock) auto-finalizes → worker MarkDepositPaid CAS-moves the
//     quote sent→deposit_paid + the request quoted→deposit_paid (idempotent).
//   - RefundDeposit is fail-closed (gateway first); full-amount only.

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/itinerary"
	"jingdezhen-ceramics-backend/internal/platform/fx"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// seedOptionRate inserts one option_rate row (the quote builder's price book).
func seedOptionRate(t *testing.T, pool *pgxpool.Pool, key string, rateCNY int64, unit, label string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO option_rates (option_key, rate_cny, unit, display_label)
		VALUES ($1,$2,$3,$4) ON CONFLICT (option_key) DO UPDATE SET rate_cny=EXCLUDED.rate_cny`, key, rateCNY, unit, label)
	require.NoError(t, err)
}

// seedFXRate inserts an fx_rates row so Convert/Rate work (the quote FX path).
func seedFXRate(t *testing.T, pool *pgxpool.Pool, currency string, rateToCNY string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO fx_rates (currency, rate_to_cny) VALUES ($1,$2) ON CONFLICT (currency) DO UPDATE SET rate_to_cny=EXCLUDED.rate_to_cny`,
		currency, rateToCNY)
	require.NoError(t, err)
}

// newFXService builds a real fx.Service against the test DB (for Convert+Rate).
func newFXService(t *testing.T, pool *pgxpool.Pool) fx.ServiceInterface {
	t.Helper()
	repo := fx.NewRepository(pool)
	return fx.NewService(repo, nil, 200) // source nil (no refresh); markup unused for Convert
}

// seedProcessingRequest inserts a `processing` request ready to be quoted.
func seedProcessingRequest(t *testing.T, pool *pgxpool.Pool, uid string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO itinerary_requests (user_id, status, duration_days, adults, pace,
			interests, services, contact, locale, sla_deadline, submitted_at)
		VALUES ($1,'processing',5,2,'balanced','[]','{}','{}','en-US', NOW()+INTERVAL '24 hours', NOW())
		RETURNING id`, uid).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestSendQuote_PricesFromOptionRatesAndConverts verifies the quote is priced
// from option_rates (CNY), FX-converted to presentment, deposit = 30%, and the
// request moves processing→quoted.
func TestSendQuote_PricesFromOptionRatesAndConverts(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-send@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "hotel-comfort", 40000, "per_person", "Comfort hotel")
	seedOptionRate(t, pool, "base-itinerary", 1000, "per_day", "Base itinerary")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{
			{OptionKey: "hotel-comfort", Qty: 2},
			{OptionKey: "base-itinerary", Qty: 5},
		},
		Currency: "USD",
	})
	require.NoError(t, err)

	// total_cny = 40000×2 + 1000×5 = 85000 fen.
	require.Equal(t, int64(85000), q.TotalCNY)
	// total_minor = fx.Convert(85000, "USD") (~$119.74 → 11974 minor per the rounding rule).
	totalMinor, err := fxs.Convert(ctx, 85000, "USD")
	require.NoError(t, err)
	require.Equal(t, totalMinor, q.TotalMinor)
	// deposit = round(totalMinor × 0.30).
	require.Equal(t, (totalMinor*30+50)/100, q.DepositMinor)
	require.Equal(t, "USD", q.Currency)
	require.Equal(t, models.QuoteSent, q.Status)
	require.NotNil(t, q.FxRateUsed)

	// The request moved processing→quoted.
	r, err := repo.GetByIDAdmin(ctx, reqID)
	require.NoError(t, err)
	require.Equal(t, models.StatusItineraryQuoted, r.Status)
}

// TestCreateQuote_ReplacesOnResend verifies UNIQUE(request_id) ON CONFLICT: a
// second SendQuote replaces the first (one row, latest totals win).
func TestCreateQuote_ReplacesOnResend(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-resend@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "hotel-budget", 15000, "per_person", "Budget hotel")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	// First quote: 2× hotel-budget.
	_, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "hotel-budget", Qty: 2}},
		Currency:  "USD",
	})
	require.NoError(t, err)
	// Second quote: 1× pickup (different totals).
	q2, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "pickup", Qty: 1}},
		Currency:  "USD",
	})
	require.NoError(t, err)
	require.Equal(t, int64(5000), q2.TotalCNY, "second quote's totals replace the first")

	// Exactly one quote row for the request.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM itinerary_quotes WHERE request_id=$1`, reqID).Scan(&n))
	require.Equal(t, 1, n, "re-send replaces (one active quote per request)")
}

// TestSendQuote_RejectsNonProcessing verifies only a processing request can be
// quoted (a pending request → ErrInvalidOperation).
func TestSendQuote_RejectsNonProcessing(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-state@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	// A pending (not processing) request.
	reqID := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))
	_, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "pickup", Qty: 1}},
		Currency:  "USD",
	})
	require.ErrorIs(t, err, models.ErrInvalidOperation)
}

// TestSendQuote_UnknownOptionKey verifies an unknown option_key → ErrInvalidQuote.
func TestSendQuote_UnknownOptionKey(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-badkey@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	_, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "nonexistent", Qty: 1}},
		Currency:  "USD",
	})
	require.ErrorIs(t, err, models.ErrInvalidQuote)
}

// TestMarkDepositPaid_Idempotent verifies the worker finalize is idempotent:
// a replayed webhook (second MarkDepositPaid) is a no-op (CAS already-paid).
func TestMarkDepositPaid_Idempotent(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-pay@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "pickup", Qty: 1}},
		Currency:  "USD",
	})
	require.NoError(t, err)

	// First finalize → quote deposit_paid, request deposit_paid.
	require.NoError(t, svc.MarkDepositPaid(ctx, q.ID))
	gq, _ := repo.GetQuoteByRequestID(ctx, reqID)
	require.Equal(t, models.QuoteDepositPaid, gq.Status)
	require.NotNil(t, gq.PaidAt)
	gr, _ := repo.GetByIDAdmin(ctx, reqID)
	require.Equal(t, models.StatusItineraryDepositPaid, gr.Status)

	// Replay → idempotent no-op (quote stays deposit_paid, no error).
	require.NoError(t, svc.MarkDepositPaid(ctx, q.ID))
}

// TestConfirm_DepositPaidToConfirmed verifies confirm moves deposit_paid→confirmed
// and rejects a non-deposit_paid request.
func TestConfirm_DepositPaidToConfirmed(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-confirm@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "pickup", Qty: 1}},
		Currency:  "USD",
	})
	require.NoError(t, err)

	// Confirm before deposit → ErrConflict (not deposit_paid).
	err = svc.Confirm(ctx, reqID)
	require.ErrorIs(t, err, models.ErrConflict)

	// Pay deposit → then confirm.
	require.NoError(t, svc.MarkDepositPaid(ctx, q.ID))
	require.NoError(t, svc.Confirm(ctx, reqID))
	gr, _ := repo.GetByIDAdmin(ctx, reqID)
	require.Equal(t, models.StatusItineraryConfirmed, gr.Status)
}

// TestRefundDeposit_CancelQuoteAndRequest verifies the planner refund path:
// CancelQuote + request deposit_paid→cancelled.
func TestRefundDeposit_CancelQuoteAndRequest(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-refund@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "pickup", Qty: 1}},
		Currency:  "USD",
	})
	require.NoError(t, err)
	require.NoError(t, svc.MarkDepositPaid(ctx, q.ID))

	// Refund without a QuoteRefunder wired (nil) → CancelQuote + request cancelled
	// (the gateway refund is the fail-closed seam; a nil QuoteRefunder skips it,
	// so this tests the post-gateway state transition).
	err = svc.RefundDeposit(ctx, reqID, models.RefundDepositRequest{Reason: "customer cancelled"})
	require.NoError(t, err)

	gq, _ := repo.GetQuoteByRequestID(ctx, reqID)
	require.Equal(t, models.QuoteCancelled, gq.Status)
	gr, _ := repo.GetByIDAdmin(ctx, reqID)
	require.Equal(t, models.StatusItineraryCancelled, gr.Status)
}

// TestPayDeposit_OwnershipAndStatus verifies a non-owner cannot pay, and a
// non-quoted request → ErrRequestNotQuoted.
func TestPayDeposit_OwnershipAndStatus(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-pay-owner@t.test")
	other := seedUser(t, pool, "q-other@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "pickup", 5000, "flat", "Pickup")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)

	// Non-owner → ErrNotFound (scoped to user).
	_, err := svc.PayDeposit(ctx, other, reqID, models.PayDepositRequest{Gateway: "mock"})
	require.ErrorIs(t, err, models.ErrNotFound)

	// Owner but not quoted (still processing) → ErrRequestNotQuoted.
	_, err = svc.PayDeposit(ctx, uid, reqID, models.PayDepositRequest{Gateway: "mock"})
	require.ErrorIs(t, err, models.ErrRequestNotQuoted)
}

// keep decimal referenced (the fx service uses it; this guards against an
// unused-import false positive if the helper signatures change).
var _ = decimal.Zero

// pdfEnqueuerCapture records the last EnqueuePDFGenerate call (kind, entityID,
// locale) so a test can assert the quote-send path enqueues the right job.
type pdfEnqueuerCapture struct {
	called   bool
	kind     string
	entityID int64
	locale   string
}

func (m *pdfEnqueuerCapture) EnqueuePDFGenerate(ctx context.Context, kind string, entityID int64, locale string) error {
	m.called = true
	m.kind = kind
	m.entityID = entityID
	m.locale = locale
	return nil
}

// TestSendQuote_EnqueuesPDF verifies SendQuote enqueues a pdf:generate job
// for the newly created quote with kind="itinerary_quote" + the request's
// locale (TDD §12, M3 #4). Best-effort: a nil enqueuer must not panic.
func TestSendQuote_EnqueuesPDF(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-pdf@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "hotel-comfort", 40000, "per_person", "Comfort hotel")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")
	capture := &pdfEnqueuerCapture{}
	svc.SetPDFEnqueuer(capture)

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "hotel-comfort", Qty: 2}},
		Currency:  "USD",
	})
	require.NoError(t, err)

	require.True(t, capture.called, "SendQuote should enqueue a pdf:generate job")
	require.Equal(t, "itinerary_quote", capture.kind)
	require.Equal(t, q.ID, capture.entityID, "entityID should be the new quote ID")
	require.Equal(t, "en-US", capture.locale, "locale should come from the request")
}

// TestSendQuote_NilPDFEnqueuerNoPanic verifies a nil enqueuer (the worker-side
// service, or a test that doesn't wire one) doesn't panic SendQuote.
func TestSendQuote_NilPDFEnqueuerNoPanic(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-pdf-nil@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "hotel-comfort", 40000, "per_person", "Comfort hotel")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock") // no SetPDFEnqueuer
	reqID := seedProcessingRequest(t, pool, uid)
	_, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "hotel-comfort", Qty: 2}},
		Currency:  "USD",
	})
	require.NoError(t, err)
}

// TestSetQuotePDFKey_Persists verifies the repo round-trips pdf_key: NULL on
// create, set by SetQuotePDFKey, cleared on re-quote (ON CONFLICT).
func TestSetQuotePDFKey_Persists(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "q-pdfkey@t.test")
	seedFXRate(t, pool, "USD", "7.09832850")
	seedOptionRate(t, pool, "hotel-comfort", 40000, "per_person", "Comfort hotel")
	fxs := newFXService(t, pool)
	svc := itinerary.NewService(repo, nil, nil, nil, fxs, "mock")

	reqID := seedProcessingRequest(t, pool, uid)
	q, err := svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "hotel-comfort", Qty: 2}},
		Currency:  "USD",
	})
	require.NoError(t, err)

	// pdf_key starts NULL.
	reloaded, err := repo.GetQuoteByID(ctx, q.ID)
	require.NoError(t, err)
	require.Nil(t, reloaded.PDFKey)

	// Set it (worker render path).
	require.NoError(t, repo.SetQuotePDFKey(ctx, q.ID, "pdf/itineraries/42.pdf"))
	reloaded, err = repo.GetQuoteByID(ctx, q.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.PDFKey)
	require.Equal(t, "pdf/itineraries/42.pdf", *reloaded.PDFKey)

	// Re-quote clears pdf_key (ON CONFLICT ... pdf_key = NULL).
	_, err = svc.SendQuote(ctx, reqID, models.SendQuoteRequest{
		LineItems: []models.QuoteLineItemInput{{OptionKey: "hotel-comfort", Qty: 3}},
		Currency:  "USD",
	})
	require.NoError(t, err)
	reloaded2, err := repo.GetQuoteByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Nil(t, reloaded2.PDFKey, "re-quote must clear the stale pdf_key")
}

// TestSetQuotePDFKey_NotFound verifies SetQuotePDFKey on a missing quote →
// ErrNotFound (mirrors the cert repo's SetPDFKey contract).
func TestSetQuotePDFKey_NotFound(t *testing.T) {
	pool := testutil.NewDBPool(t)
	repo := itinerary.NewRepository(pool)
	err := repo.SetQuotePDFKey(context.Background(), 999999, "pdf/itineraries/x.pdf")
	require.ErrorIs(t, err, models.ErrNotFound)
}
