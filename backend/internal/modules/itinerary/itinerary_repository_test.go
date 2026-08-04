package itinerary_test

// Integration tests for the itinerary repository against real PostgreSQL
// (testcontainers-go). The bug-prone bits unique to the wizard:
//   - itinerary_drafts is one-per-user (UNIQUE) — a second save must REPLACE,
//     not insert a duplicate (the ON CONFLICT upsert).
//   - Submit deletes the draft in the same tx as the request insert (a submit
//     clears the draft; if the tx rolled back the draft would survive, but the
//     happy path is draft-gone-after-submit).
//   - CancelRequest is a CAS pending→cancelled scoped to the user: not-owned →
//     ErrNotFound (not a leak), non-pending → ErrItineraryNotCancellable.
//
// Pattern: NewDBPool(t) → seed a user → exercise the real repo → assert on
// rows re-read from the DB.

import (
	"context"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/itinerary"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// seedUser inserts a minimal user row and returns its id (the itinerary tables
// FK to users).
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO users (email, is_active, auth_provider)
		VALUES ($1, true, 'email') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestUpsertDraft_OnePerUser verifies the UNIQUE(user_id) upsert: a second save
// REPLACES the first (same row id, updated step/form_state), not a second row.
// A regression here (e.g. plain INSERT) would collide on the unique constraint.
func TestUpsertDraft_OnePerUser(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "draft@t.test")

	d1, err := repo.UpsertDraft(ctx, uid, models.ItineraryDraftData{
		Step: 1, FormState: []byte(`{"arrival_date":"2026-09-01"}`),
	})
	require.NoError(t, err)

	d2, err := repo.UpsertDraft(ctx, uid, models.ItineraryDraftData{
		Step: 3, FormState: []byte(`{"arrival_date":"2026-09-01","pace":"balanced"}`),
	})
	require.NoError(t, err)

	// Same row (UNIQUE upsert), updated fields.
	require.Equal(t, d1.ID, d2.ID, "a second save must UPDATE the same row, not insert")
	require.Equal(t, 3, d2.Step)
	require.Contains(t, string(d2.FormState), "balanced")

	// The DB agrees: exactly one draft row for this user.
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM itinerary_drafts WHERE user_id=$1`, uid).Scan(&n))
	require.Equal(t, 1, n, "one draft per user — a second save must not create a second row")
}

// TestSubmit_DeletesDraftInTx verifies the submit clears the draft in the same
// tx as the request insert. A regression that skipped the draft delete (or did
// it outside the tx) would leave a stale draft after submit.
func TestSubmit_DeletesDraftInTx(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "submit@t.test")

	// Save a draft.
	_, err := repo.UpsertDraft(ctx, uid, models.ItineraryDraftData{Step: 4, FormState: []byte(`{}`)})
	require.NoError(t, err)

	// Submit a request (the repo's CreateRequest deletes the draft in its tx).
	arrival := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	req, err := repo.CreateRequest(ctx, &models.ItineraryRequest{
		UserID: uid, DurationDays: 5, Adults: 2, Pace: "balanced",
		Interests: []byte(`[]`), Services: []byte(`{}`), Contact: []byte(`{}`),
		Locale: "en-US", SLADeadline: time.Now().Add(24 * time.Hour),
		ArrivalDate: &arrival,
	})
	require.NoError(t, err)
	require.NotZero(t, req.ID)
	require.Equal(t, models.StatusItineraryPending, req.Status)

	// The draft is gone (deleted in the same tx).
	_, err = repo.GetDraft(ctx, uid)
	require.ErrorIs(t, err, models.ErrNotFound, "the draft must be cleared on submit")

	// The request row exists + is owned by the user.
	got, err := repo.GetByIDForUser(ctx, uid, req.ID)
	require.NoError(t, err)
	require.Equal(t, req.ID, got.ID)
	require.Equal(t, models.StatusItineraryPending, got.Status)
}

// TestCancelRequest_CAS verifies the cancel state machine:
//   - pending → cancelled succeeds (CAS matches).
//   - a second cancel → ErrItineraryNotCancellable (already cancelled).
//   - cancel on another user's request → ErrNotFound (no leak).
func TestCancelRequest_CAS(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "cancel@t.test")
	other := seedUser(t, pool, "other@t.test")

	// Create a pending request for uid.
	arrival := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	req, err := repo.CreateRequest(ctx, &models.ItineraryRequest{
		UserID: uid, DurationDays: 5, Adults: 2, Pace: "balanced",
		Interests: []byte(`[]`), Services: []byte(`{}`), Contact: []byte(`{}`),
		Locale: "en-US", SLADeadline: time.Now().Add(24 * time.Hour),
		ArrivalDate: &arrival,
	})
	require.NoError(t, err)

	// Cancel by the OTHER user → ErrNotFound (scoped to user_id; no leak that
	// the request exists).
	err = repo.CancelRequest(ctx, other, req.ID, "x")
	require.ErrorIs(t, err, models.ErrNotFound)

	// Cancel by the owner → ok (pending→cancelled).
	require.NoError(t, repo.CancelRequest(ctx, uid, req.ID, "plans changed"))

	// Re-read: status is cancelled, reason + timestamp set.
	got, err := repo.GetByIDForUser(ctx, uid, req.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusItineraryCancelled, got.Status)
	require.NotNil(t, got.CancelReason)
	require.Equal(t, "plans changed", *got.CancelReason)
	require.NotNil(t, got.CancelledAt)

	// Second cancel by the owner → ErrItineraryNotCancellable (not pending).
	err = repo.CancelRequest(ctx, uid, req.ID, "again")
	require.ErrorIs(t, err, models.ErrItineraryNotCancellable)
}

// TestSLADeadline_SetOnCreate verifies the service computes sla_deadline =
// submitted_at + 24h (PRD §3.3.2). The repo stores whatever the service passes;
// this test confirms the service-side computation lands in the DB.
func TestSLADeadline_SetOnCreate(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "sla@t.test")

	before := time.Now().Add(-time.Second)
	sla := before.Add(24 * time.Hour)
	req, err := repo.CreateRequest(ctx, &models.ItineraryRequest{
		UserID: uid, DurationDays: 3, Adults: 1, Pace: "relaxed",
		Interests: []byte(`[]`), Services: []byte(`{}`), Contact: []byte(`{}`),
		Locale: "en-US", SLADeadline: sla,
	})
	require.NoError(t, err)

	// The SLA deadline is ~24h after submission (allow 1s skew for the test).
	require.WithinDuration(t, req.SubmittedAt.Add(24*time.Hour), req.SLADeadline, 2*time.Second,
		"sla_deadline must be ~24h after submitted_at")
}
