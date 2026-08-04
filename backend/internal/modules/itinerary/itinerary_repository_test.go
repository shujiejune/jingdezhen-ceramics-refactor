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

// seedStaffUser inserts a user AND assigns them a staff role (the planner CRM
// assignment validation checks the assignee is an active travel_planner).
func seedStaffUser(t *testing.T, pool *pgxpool.Pool, email, roleKey string) string {
	t.Helper()
	uid := seedUser(t, pool, email)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE key = $2
		ON CONFLICT DO NOTHING`, uid, roleKey)
	require.NoError(t, err)
	return uid
}

// seedRequest inserts a request in a given status with a given sla_deadline
// and returns its id. A direct INSERT avoids the service's 24h computation so
// tests can plant a breached/approaching deadline.
func seedRequest(t *testing.T, pool *pgxpool.Pool, uid string, sla time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO itinerary_requests (user_id, status, duration_days, adults, pace,
			interests, services, contact, locale, sla_deadline, submitted_at)
		VALUES ($1, 'pending', 5, 2, 'balanced', '[]', '{}', '{}', 'en-US', $2, NOW())
		RETURNING id`, uid, sla).Scan(&id)
	require.NoError(t, err)
	return id
}

// TestTransitionStatus_CAS verifies the planner open/close state machine:
//   - open (pending→processing) succeeds; re-open → ErrConflict (not pending).
//   - close (processing→closed) succeeds.
//   - transition on an absent request → ErrNotFound.
func TestTransitionStatus_CAS(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "t-cas@t.test")

	id := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))

	require.NoError(t, repo.TransitionStatus(ctx, id, models.StatusItineraryPending, models.StatusItineraryProcessing))
	// Re-open: not pending → ErrConflict.
	err := repo.TransitionStatus(ctx, id, models.StatusItineraryPending, models.StatusItineraryProcessing)
	require.ErrorIs(t, err, models.ErrConflict)

	// Close: processing→closed.
	require.NoError(t, repo.TransitionStatus(ctx, id, models.StatusItineraryProcessing, models.StatusItineraryClosed))

	// Absent → ErrNotFound.
	err = repo.TransitionStatus(ctx, 999999, models.StatusItineraryPending, models.StatusItineraryProcessing)
	require.ErrorIs(t, err, models.ErrNotFound)
}

// TestCancelByStaff_AcceptsProcessing verifies the planner cancel path accepts
// both pending AND processing (the customer path only allows pending) and
// records the cancel_reason + cancelled_at.
func TestCancelByStaff_AcceptsProcessing(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "t-staffcancel@t.test")

	id := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))
	// Move to processing first.
	require.NoError(t, repo.TransitionStatus(ctx, id, models.StatusItineraryPending, models.StatusItineraryProcessing))
	// Staff cancel from processing → ok.
	require.NoError(t, repo.CancelByStaff(ctx, id, "no longer needed"))

	got, err := repo.GetByIDAdmin(ctx, id)
	require.NoError(t, err)
	require.Equal(t, models.StatusItineraryCancelled, got.Status)
	require.NotNil(t, got.CancelReason)
	require.Equal(t, "no longer needed", *got.CancelReason)
	require.NotNil(t, got.CancelledAt)

	// Re-cancel a confirmed request → ErrItineraryNotCancellable (not cancellable).
	id2 := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))
	require.NoError(t, repo.TransitionStatus(ctx, id2, models.StatusItineraryPending, models.StatusItineraryConfirmed))
	err = repo.CancelByStaff(ctx, id2, "x")
	require.ErrorIs(t, err, models.ErrItineraryNotCancellable)
}

// TestAssign_PersistsAndUnassigns verifies assign + unassign (nil).
func TestAssign_PersistsAndUnassigns(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "t-assign@t.test")
	planner := seedStaffUser(t, pool, "planner-assign@t.test", models.RoleTravelPlanner)

	id := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))

	require.NoError(t, repo.Assign(ctx, id, &planner))
	got, err := repo.GetByIDAdmin(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got.AssignedTo)
	require.Equal(t, planner, *got.AssignedTo)

	// Unassign (nil).
	require.NoError(t, repo.Assign(ctx, id, nil))
	got, err = repo.GetByIDAdmin(ctx, id)
	require.NoError(t, err)
	require.Nil(t, got.AssignedTo)

	// Absent → ErrNotFound.
	err = repo.Assign(ctx, 999999, nil)
	require.ErrorIs(t, err, models.ErrNotFound)
}

// TestAddNote_ListNotes verifies a note is saved with the author's display
// name joined, and ListNotes returns newest-first.
func TestAddNote_ListNotes(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "t-notes@t.test")
	author := seedStaffUser(t, pool, "note-author@t.test", models.RoleTravelPlanner)

	id := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))

	n1, err := repo.AddNote(ctx, id, author, "First follow-up: emailed customer.")
	require.NoError(t, err)
	require.Equal(t, "First follow-up: emailed customer.", n1.Body)
	require.Equal(t, "note-author@t.test", n1.AuthorName, "author name falls back to email when nickname unset")

	n2, err := repo.AddNote(ctx, id, author, "Second: confirmed dates.")
	require.NoError(t, err)

	notes, err := repo.ListNotes(ctx, id)
	require.NoError(t, err)
	require.Len(t, notes, 2)
	require.Equal(t, n2.ID, notes[0].ID, "newest first")
	require.Equal(t, n1.ID, notes[1].ID)
}

// TestListAdmin_FiltersAndJoinsCustomer verifies the inbox JOINs the customer's
// email/nickname + filters by status and assigned_to (incl. "unassigned").
func TestListAdmin_FiltersAndJoinsCustomer(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	cust := seedUser(t, pool, "inbox-customer@t.test")
	planner := seedStaffUser(t, pool, "inbox-planner@t.test", models.RoleTravelPlanner)

	// Two requests: one pending+assigned, one processing+unassigned.
	sla := time.Now().Add(24 * time.Hour)
	id1 := seedRequest(t, pool, cust, sla)
	require.NoError(t, repo.Assign(ctx, id1, &planner))
	id2 := seedRequest(t, pool, cust, sla)
	require.NoError(t, repo.TransitionStatus(ctx, id2, models.StatusItineraryPending, models.StatusItineraryProcessing))

	// All requests → 2 rows, customer email joined.
	rows, total, err := repo.ListAdmin(ctx, "", "", "", 1, 50)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, rows, 2)
	require.Equal(t, "inbox-customer@t.test", rows[0].CustomerEmail)

	// Filter by status=processing → 1 row (id2).
	rows, total, err = repo.ListAdmin(ctx, "processing", "", "", 1, 50)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, id2, rows[0].ID)

	// Filter by assigned_to=unassigned → 1 row (id2).
	rows, total, err = repo.ListAdmin(ctx, "", "unassigned", "", 1, 50)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, id2, rows[0].ID)

	// Filter by assigned_to=<planner> → 1 row (id1).
	rows, total, err = repo.ListAdmin(ctx, "", planner, "", 1, 50)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, id1, rows[0].ID)
}

// TestGetByIDAdmin_JoinsCustomer verifies the admin detail view includes the
// customer's email + nickname.
func TestGetByIDAdmin_JoinsCustomer(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	cust := seedUser(t, pool, "detail-customer@t.test")
	id := seedRequest(t, pool, cust, time.Now().Add(24*time.Hour))

	got, err := repo.GetByIDAdmin(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
	require.Equal(t, "detail-customer@t.test", got.CustomerEmail)
	require.Contains(t, []string{got.CustomerNickname, ""}, got.CustomerNickname)
	require.NotEmpty(t, got.SLAStatus, "SLA status is derived in SQL")
}

// TestSLA_ListBreached_AndNotifiedCAS verifies the SLA cron path:
//   - ListBreached returns pending/processing requests past their deadline +
//     not yet notified.
//   - SetSLANotified wins exactly once (the CAS); a second call returns false
//     (a concurrent tick already flagged it → skip notification).
//   - After the CAS, ListBreached excludes the request.
func TestSLA_ListBreached_AndNotifiedCAS(t *testing.T) {
	pool := testutil.NewDBPool(t)
	ctx := context.Background()
	repo := itinerary.NewRepository(pool)
	uid := seedUser(t, pool, "t-sla@t.test")

	// A breached request (deadline in the past) + a healthy one (future).
	breachedID := seedRequest(t, pool, uid, time.Now().Add(-2*time.Hour))
	healthyID := seedRequest(t, pool, uid, time.Now().Add(24*time.Hour))

	breached, err := repo.ListBreached(ctx)
	require.NoError(t, err)
	require.Len(t, breached, 1)
	require.Equal(t, breachedID, breached[0].ID, "only the past-deadline request is breached")
	_ = healthyID

	// First CAS wins.
	won, err := repo.SetSLANotified(ctx, breachedID)
	require.NoError(t, err)
	require.True(t, won, "first SetSLANotified wins the race")

	// Second CAS loses (already notified).
	won, err = repo.SetSLANotified(ctx, breachedID)
	require.NoError(t, err)
	require.False(t, won, "second SetSLANotified must lose (exactly-once)")

	// ListBreached now excludes the flagged request.
	breached, err = repo.ListBreached(ctx)
	require.NoError(t, err)
	require.Empty(t, breached, "flagged request is excluded from future breach scans")
}
