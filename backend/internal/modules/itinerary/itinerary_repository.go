package itinerary

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines itinerary storage (PRD §3.3.2, TDD §3.4 M3).
type RepositoryInterface interface {
	// --- Draft (one per user, save & resume) ---
	UpsertDraft(ctx context.Context, userID string, data models.ItineraryDraftData) (*models.ItineraryDraft, error)
	GetDraft(ctx context.Context, userID string) (*models.ItineraryDraft, error)
	DeleteDraft(ctx context.Context, userID string) error

	// --- Requests (customer-facing) ---
	CreateRequest(ctx context.Context, req *models.ItineraryRequest) (*models.ItineraryRequest, error)
	GetByIDForUser(ctx context.Context, userID string, id int64) (*models.ItineraryRequest, error)
	ListForUser(ctx context.Context, userID string, page, limit int) ([]models.ItineraryRequest, int, error)
	// CancelRequest CAS-moves pending→cancelled for a user's request. Returns
	// ErrNotFound if absent/not-owned, ErrItineraryNotCancellable if not pending.
	CancelRequest(ctx context.Context, userID string, id int64, reason string) error

	// --- Planner CRM (PRD §3.3.2 "Backend/CRM") ---
	// ListAdmin paginates all requests for the planner inbox, enriched with
	// the customer's email/nickname + a derived SLA status. Filters: status
	// ("" = all), assignedTo (UUID, or "unassigned" for the null case),
	// sla ("breached"|"approaching"|"" = all).
	ListAdmin(ctx context.Context, status, assignedTo, sla string, page, limit int) ([]models.ItineraryAdminRow, int, error)
	GetByIDAdmin(ctx context.Context, id int64) (*models.ItineraryAdminRow, error)
	// TransitionStatus atomically moves a request from→to (CAS). Returns
	// ErrNotFound if absent, ErrConflict if the current status != from.
	TransitionStatus(ctx context.Context, id int64, from, to models.ItineraryStatus) error
	// Assign sets assigned_to (nil = unassign). Returns ErrNotFound if absent.
	Assign(ctx context.Context, id int64, assigneeID *string) error
	// CancelByStaff CAS-moves {pending,processing}→cancelled for any user's
	// request (planner-initiated; the customer path only allows pending).
	// Sets cancel_reason + cancelled_at. Returns ErrNotFound if absent,
	// ErrItineraryNotCancellable if not in a cancellable state.
	CancelByStaff(ctx context.Context, id int64, reason string) error

	// CRM notes (contact history).
	AddNote(ctx context.Context, requestID int64, authorID, body string) (*models.CRMNote, error)
	ListNotes(ctx context.Context, requestID int64) ([]models.CRMNote, error)

	// SLA-breach tracking for the sla:check cron (PRD §3.3.2).
	// ListBreached returns pending/processing requests past their sla_deadline
	// and not yet notified.
	ListBreached(ctx context.Context) ([]models.ItineraryRequest, error)
	// SetSLANotified CAS-sets sla_notified_at=NOW() iff currently NULL. Returns
	// true iff this call won the race (the caller should notify); false means a
	// concurrent cron already flagged it (skip the notification).
	SetSLANotified(ctx context.Context, id int64) (bool, error)

	// --- Quote builder + deposit (PRD §3.3.2, TDD §3.4 M3 #3) ---
	ListOptionRates(ctx context.Context) ([]models.OptionRate, error)
	GetOptionRatesByKeys(ctx context.Context, keys []string) (map[string]models.OptionRate, error)
	// CreateQuote inserts OR replaces the active quote for a request (UNIQUE
	// request_id; re-quote replaces). Returns the new row.
	CreateQuote(ctx context.Context, q *models.ItineraryQuote) (*models.ItineraryQuote, error)
	GetQuoteByRequestID(ctx context.Context, requestID int64) (*models.ItineraryQuote, error)
	GetQuoteByID(ctx context.Context, quoteID int64) (*models.ItineraryQuote, error)
	// MarkQuoteDepositPaid CAS-moves the quote sent→deposit_paid|fully_paid +
	// sets paid_at. Returns true iff this call won (idempotent on replay).
	MarkQuoteDepositPaid(ctx context.Context, quoteID int64, payFull bool) (bool, error)
	// CancelQuote CAS-moves {sent,deposit_paid,fully_paid}→cancelled.
	CancelQuote(ctx context.Context, quoteID int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// --- Draft ---

func (r *Repository) UpsertDraft(ctx context.Context, userID string, data models.ItineraryDraftData) (*models.ItineraryDraft, error) {
	d := &models.ItineraryDraft{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO itinerary_drafts (user_id, form_state, step)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
			SET form_state = EXCLUDED.form_state,
			    step = EXCLUDED.step,
			    updated_at = NOW()
		RETURNING id, user_id, form_state, step, updated_at`,
		userID, data.FormState, data.Step,
	).Scan(&d.ID, &d.UserID, &d.FormState, &d.Step, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("itinerary.UpsertDraft: %w", err)
	}
	return d, nil
}

func (r *Repository) GetDraft(ctx context.Context, userID string) (*models.ItineraryDraft, error) {
	d := &models.ItineraryDraft{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, form_state, step, updated_at
		FROM itinerary_drafts WHERE user_id = $1`, userID,
	).Scan(&d.ID, &d.UserID, &d.FormState, &d.Step, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("itinerary.GetDraft: %w", err)
	}
	return d, nil
}

func (r *Repository) DeleteDraft(ctx context.Context, userID string) error {
	// Best-effort: a submit-without-draft flow is valid, so zero-rows-deleted is
	// not an error. The tx in CreateRequest calls this after the insert commits.
	_, err := r.db.Exec(ctx, `DELETE FROM itinerary_drafts WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("itinerary.DeleteDraft: %w", err)
	}
	return nil
}

// --- Requests ---

const requestCols = `
	id, user_id, status, arrival_date, duration_days, flexible, adults, children,
	interests, budget, pace, services, contact, notes, locale, sla_deadline,
	assigned_to, submitted_at, cancel_reason, cancelled_at, created_at, updated_at `

func (r *Repository) scanRequest(row pgx.Row) (*models.ItineraryRequest, error) {
	var req models.ItineraryRequest
	if err := row.Scan(
		&req.ID, &req.UserID, &req.Status, &req.ArrivalDate, &req.DurationDays,
		&req.Flexible, &req.Adults, &req.Children, &req.Interests, &req.Budget,
		&req.Pace, &req.Services, &req.Contact, &req.Notes, &req.Locale,
		&req.SLADeadline, &req.AssignedTo, &req.SubmittedAt, &req.CancelReason,
		&req.CancelledAt, &req.CreatedAt, &req.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *Repository) CreateRequest(ctx context.Context, req *models.ItineraryRequest) (*models.ItineraryRequest, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("itinerary.CreateRequest.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	out, err := r.scanRequest(tx.QueryRow(ctx, `
		INSERT INTO itinerary_requests (
			user_id, status, arrival_date, duration_days, flexible, adults, children,
			interests, budget, pace, services, contact, notes, locale, sla_deadline, submitted_at
		) VALUES (
			$1, 'pending', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW()
		)
		RETURNING `+requestCols,
		req.UserID, req.ArrivalDate, req.DurationDays, req.Flexible, req.Adults,
		req.Children, req.Interests, req.Budget, req.Pace, req.Services, req.Contact,
		req.Notes, req.Locale, req.SLADeadline,
	))
	if err != nil {
		return nil, fmt.Errorf("itinerary.CreateRequest.Insert: %w", err)
	}

	// Delete the user's draft in the same tx (a submit clears the draft). Zero
	// rows deleted is fine (the user may have skipped saving a draft).
	if _, err := tx.Exec(ctx, `DELETE FROM itinerary_drafts WHERE user_id = $1`, req.UserID); err != nil {
		return nil, fmt.Errorf("itinerary.CreateRequest.DeleteDraft: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("itinerary.CreateRequest.Commit: %w", err)
	}
	return out, nil
}

func (r *Repository) GetByIDForUser(ctx context.Context, userID string, id int64) (*models.ItineraryRequest, error) {
	req, err := r.scanRequest(r.db.QueryRow(ctx,
		`SELECT`+requestCols+`FROM itinerary_requests WHERE id = $1 AND user_id = $2`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("itinerary.GetByIDForUser: %w", err)
	}
	return req, nil
}

func (r *Repository) ListForUser(ctx context.Context, userID string, page, limit int) ([]models.ItineraryRequest, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM itinerary_requests WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("itinerary.ListForUser.Count: %w", err)
	}
	if total == 0 {
		return []models.ItineraryRequest{}, 0, nil
	}
	offset := (page - 1) * limit
	rows, err := r.db.Query(ctx,
		`SELECT`+requestCols+`FROM itinerary_requests WHERE user_id = $1 ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("itinerary.ListForUser.Query: %w", err)
	}
	defer rows.Close()
	out := []models.ItineraryRequest{}
	for rows.Next() {
		req, err := r.scanRequest(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("itinerary.ListForUser.Scan: %w", err)
		}
		out = append(out, *req)
	}
	return out, total, rows.Err()
}

func (r *Repository) CancelRequest(ctx context.Context, userID string, id int64, reason string) error {
	// CAS pending→cancelled, scoped to the user (customer cancel only their own).
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_requests
		SET status = 'cancelled', cancel_reason = $3, cancelled_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND status = 'pending'`,
		id, userID, reason)
	if err != nil {
		return fmt.Errorf("itinerary.CancelRequest: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		// Distinguish "absent/not-owned" from "wrong status".
		var exists bool
		if err := r.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM itinerary_requests WHERE id = $1 AND user_id = $2)`, id, userID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("itinerary.CancelRequest.probe: %w", err)
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrItineraryNotCancellable
	}
	return nil
}

// slaDuration is the SLA window (PRD §3.3.2: 24 hours). Hardcoded for MVP;
// CMS-configurable once a settings table exists (TDD §3.4 settings follow-up).
const slaDuration = 24 * time.Hour

// =============================================================================
// Planner CRM repository methods (PRD §3.3.2 "Backend/CRM")
// =============================================================================

// adminCols is requestCols + the joined customer columns + the derived SLA
// status. The SLA status is computed in SQL (against NOW()) so the inbox badge
// is always current without a client clock. It mirrors ComputeSLAStatus but
// in-pipeline; confirmed/cancelled/closed → 'sla_met'.
const adminCols = `
	r.id, r.user_id, r.status, r.arrival_date, r.duration_days, r.flexible,
	r.adults, r.children, r.interests, r.budget, r.pace, r.services, r.contact,
	r.notes, r.locale, r.sla_deadline, r.assigned_to, r.submitted_at,
	r.cancel_reason, r.cancelled_at, r.created_at, r.updated_at,
	u.email AS customer_email, COALESCE(u.nickname, u.email) AS customer_nickname, (
		CASE WHEN r.status IN ('confirmed','cancelled','closed') THEN 'sla_met'
		     WHEN r.sla_deadline < NOW() THEN 'sla_breached'
		     WHEN r.sla_deadline <= NOW() + ($1 || ' hours')::INTERVAL THEN 'sla_approaching'
		     ELSE 'sla_on_time' END
	) AS sla_status `

// slaApproachingHoursParam is the 1-based bind position for the approaching
// window in adminCols (kept here so ListAdmin/GetByIDAdmin pass it correctly).
const slaApproachingHours = "4"

// scanAdminRow scans an ItineraryAdminRow from a pgx.Row. The adminCols CASE
// binds the approaching-hours window at query time ($1), so the scan itself
// needs no extra param — it reads the 25 columns adminCols selects.
func (r *Repository) scanAdminRow(row pgx.Row) (*models.ItineraryAdminRow, error) {
	var row_ models.ItineraryAdminRow
	if err := row.Scan(
		&row_.ID, &row_.UserID, &row_.Status, &row_.ArrivalDate, &row_.DurationDays,
		&row_.Flexible, &row_.Adults, &row_.Children, &row_.Interests, &row_.Budget,
		&row_.Pace, &row_.Services, &row_.Contact, &row_.Notes, &row_.Locale,
		&row_.SLADeadline, &row_.AssignedTo, &row_.SubmittedAt, &row_.CancelReason,
		&row_.CancelledAt, &row_.CreatedAt, &row_.UpdatedAt,
		&row_.CustomerEmail, &row_.CustomerNickname, &row_.SLAStatus,
	); err != nil {
		return nil, err
	}
	return &row_, nil
}

// ListAdmin paginates all requests for the planner inbox (PRD §3.3.2). Filters
// are optional (empty = ignore). assignedTo="unassigned" selects NULL assignees.
func (r *Repository) ListAdmin(ctx context.Context, status, assignedTo, sla string, page, limit int) ([]models.ItineraryAdminRow, int, error) {
	// Build filter fragments. The adminCols CASE references $1 (approachHours)
// so the ROWS query always binds approachHours as $1 + filters from $2.
// The COUNT query does NOT include adminCols, so it only binds the params its
// WHERE actually references — built separately to avoid a placeholder/arg
// count mismatch (pgx errors when args exceed referenced $N).
	filterFragments := []string{}
	filterArgs := []interface{}{} // bound AFTER approachHours in the rows query
	if status != "" {
		filterFragments = append(filterFragments, fmt.Sprintf("r.status = $%d", len(filterArgs)+2))
		filterArgs = append(filterArgs, status)
	}
	if assignedTo != "" {
		if assignedTo == "unassigned" {
			filterFragments = append(filterFragments, "r.assigned_to IS NULL")
		} else {
			filterFragments = append(filterFragments, fmt.Sprintf("r.assigned_to = $%d::uuid", len(filterArgs)+2))
			filterArgs = append(filterArgs, assignedTo)
		}
	}
	if sla != "" {
		// Re-derive the SLA status in the WHERE to match the badge. The CASE in
		// adminCols uses $1 (approachHours); the approaching filter replicates it
		// against the same $1 bind so the filter matches what the badge shows.
		switch sla {
		case models.SLABreached:
			filterFragments = append(filterFragments, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline < NOW()")
		case models.SLAApproaching:
			filterFragments = append(filterFragments, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline >= NOW() AND r.sla_deadline <= NOW() + ($1 || ' hours')::INTERVAL")
		case models.SLAOnTime:
			filterFragments = append(filterFragments, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline > NOW() + ($1 || ' hours')::INTERVAL")
		}
	}
	whereClause := ""
	if len(filterFragments) > 0 {
		whereClause = "WHERE " + joinStrings(filterFragments, " AND ")
	}

	// COUNT: bind only the params its WHERE references. approachHours ($1) is
	// referenced only by the approaching/on-time SLA filter; status + assignee
	// follow. buildCountWhere returns the WHERE + its args in lockstep so the
	// placeholder count never diverges from the arg count.
	countWhere, countArgs := buildCountWhere(status, assignedTo, sla)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM itinerary_requests r `+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("itinerary.ListAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.ItineraryAdminRow{}, 0, nil
	}

	// ROWS: approachHours is $1 (adminCols CASE), filters start at $2.
	offset := (page - 1) * limit
	limitIdx := len(filterArgs) + 2
	offsetIdx := limitIdx + 1
	rowsArgs := append([]interface{}{slaApproachingHours}, filterArgs...)
	rowsArgs = append(rowsArgs, limit, offset)
	rows, err := r.db.Query(ctx,
		`SELECT`+adminCols+`FROM itinerary_requests r
		 JOIN users u ON u.id = r.user_id
		 `+whereClause+`
		 ORDER BY r.sla_deadline ASC, r.submitted_at DESC
		 LIMIT $`+strconv.Itoa(limitIdx)+` OFFSET $`+strconv.Itoa(offsetIdx),
		rowsArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("itinerary.ListAdmin.Query: %w", err)
	}
	defer rows.Close()
	out := []models.ItineraryAdminRow{}
	for rows.Next() {
		row_, err := r.scanAdminRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("itinerary.ListAdmin.Scan: %w", err)
		}
		out = append(out, *row_)
	}
	return out, total, rows.Err()
}

// buildCountWhere constructs the WHERE clause + args for the ListAdmin COUNT
// query with a fresh placeholder counter (approachHours is $1 iff the
// approaching/on-time SLA filter references it; status/assignee follow). The
// args slice is returned in lockstep with the placeholders so they can never
// diverge (which would surface as a pgx arg-count mismatch).
func buildCountWhere(status, assignedTo, sla string) (string, []interface{}) {
	frags := []string{}
	args := []interface{}{}
	n := 1
	needsApproach := sla == models.SLAApproaching || sla == models.SLAOnTime
	if needsApproach {
		args = append(args, slaApproachingHours) // $1
		n = 2 // status/assignee start at $2
	}
	if status != "" {
		frags = append(frags, fmt.Sprintf("r.status = $%d", n))
		args = append(args, status)
		n++
	}
	if assignedTo != "" && assignedTo != "unassigned" {
		frags = append(frags, fmt.Sprintf("r.assigned_to = $%d::uuid", n))
		args = append(args, assignedTo)
		n++
	}
	if assignedTo == "unassigned" {
		frags = append(frags, "r.assigned_to IS NULL")
	}
	switch sla {
	case models.SLABreached:
		frags = append(frags, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline < NOW()")
	case models.SLAApproaching:
		frags = append(frags, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline >= NOW() AND r.sla_deadline <= NOW() + ($1 || ' hours')::INTERVAL")
	case models.SLAOnTime:
		frags = append(frags, "r.status NOT IN ('confirmed','cancelled','closed') AND r.sla_deadline > NOW() + ($1 || ' hours')::INTERVAL")
	}
	if len(frags) == 0 {
		return "", nil
	}
	return "WHERE " + joinStrings(frags, " AND "), args
}

func (r *Repository) GetByIDAdmin(ctx context.Context, id int64) (*models.ItineraryAdminRow, error) {
	row_, err := r.scanAdminRow(r.db.QueryRow(ctx,
		`SELECT`+adminCols+`FROM itinerary_requests r
		 JOIN users u ON u.id = r.user_id
		 WHERE r.id = $2`, slaApproachingHours, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("itinerary.GetByIDAdmin: %w", err)
	}
	return row_, nil
}

// TransitionStatus atomically moves a request from→to (CAS, mirrors the order
// module's TransitionStatus). No per-state timestamp columns exist yet (the
// quote/deposit sub-track will add quoted_at/deposit_paid_at); this relies on
// updated_at + crm_notes for the narrative trail. Returns ErrNotFound if the
// request is absent, ErrConflict if the current status != from.
func (r *Repository) TransitionStatus(ctx context.Context, id int64, from, to models.ItineraryStatus) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_requests SET status = $3, updated_at = NOW()
		WHERE id = $1 AND status = $2`,
		id, string(from), string(to))
	if err != nil {
		return fmt.Errorf("itinerary.TransitionStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM itinerary_requests WHERE id=$1)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("itinerary.TransitionStatus.probe: %w", err)
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrConflict
	}
	return nil
}

// Assign sets assigned_to (nil = unassign). Returns ErrNotFound if absent.
func (r *Repository) Assign(ctx context.Context, id int64, assigneeID *string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_requests SET assigned_to = $2::uuid, updated_at = NOW()
		WHERE id = $1`, id, assigneeID) // $2 nil → NULL
	if err != nil {
		return fmt.Errorf("itinerary.Assign: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// CancelByStaff CAS-moves {pending,processing}→cancelled (planner-initiated).
// Mirrors the customer CancelRequest (sets cancel_reason + cancelled_at) but
// is not user-scoped and accepts processing (not just pending).
func (r *Repository) CancelByStaff(ctx context.Context, id int64, reason string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_requests
		SET status = 'cancelled', cancel_reason = $2, cancelled_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('pending','processing')`, id, reason)
	if err != nil {
		return fmt.Errorf("itinerary.CancelByStaff: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM itinerary_requests WHERE id=$1)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("itinerary.CancelByStaff.probe: %w", err)
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrItineraryNotCancellable
	}
	return nil
}

// --- CRM notes ---

func (r *Repository) AddNote(ctx context.Context, requestID int64, authorID, body string) (*models.CRMNote, error) {
	n := &models.CRMNote{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO crm_notes (request_id, author_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, request_id, author_id, body, created_at`,
		requestID, authorID, body,
	).Scan(&n.ID, &n.RequestID, &n.AuthorID, &n.Body, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("itinerary.AddNote: %w", err)
	}
	// Resolve the author's display name (nickname, fall back to email).
	if err := r.db.QueryRow(ctx,
		`SELECT COALESCE(nickname, email) FROM users WHERE id = $1`, authorID,
	).Scan(&n.AuthorName); err != nil {
		n.AuthorName = "" // best-effort; the note is still saved
	}
	return n, nil
}

func (r *Repository) ListNotes(ctx context.Context, requestID int64) ([]models.CRMNote, error) {
	rows, err := r.db.Query(ctx, `
		SELECT n.id, n.request_id, n.author_id, COALESCE(u.nickname, u.email), n.body, n.created_at
		FROM crm_notes n
		JOIN users u ON u.id = n.author_id
		WHERE n.request_id = $1
		ORDER BY n.created_at DESC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("itinerary.ListNotes: %w", err)
	}
	defer rows.Close()
	out := []models.CRMNote{}
	for rows.Next() {
		var note models.CRMNote
		if err := rows.Scan(&note.ID, &note.RequestID, &note.AuthorID, &note.AuthorName, &note.Body, &note.CreatedAt); err != nil {
			return nil, fmt.Errorf("itinerary.ListNotes.Scan: %w", err)
		}
		out = append(out, note)
	}
	return out, rows.Err()
}

// --- SLA-breach tracking (sla:check cron) ---

// ListBreached returns pending/processing requests past their sla_deadline and
// not yet notified. The partial index idx_itin_req_sla_breach serves this.
func (r *Repository) ListBreached(ctx context.Context) ([]models.ItineraryRequest, error) {
	rows, err := r.db.Query(ctx, `SELECT`+requestCols+`FROM itinerary_requests
		WHERE status IN ('pending','processing') AND sla_deadline < NOW() AND sla_notified_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("itinerary.ListBreached: %w", err)
	}
	defer rows.Close()
	out := []models.ItineraryRequest{}
	for rows.Next() {
		req, err := r.scanRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("itinerary.ListBreached.Scan: %w", err)
		}
		out = append(out, *req)
	}
	return out, rows.Err()
}

// SetSLANotified CAS-sets sla_notified_at=NOW() iff currently NULL. Returns
// true iff this call won the race (exactly-once notification).
func (r *Repository) SetSLANotified(ctx context.Context, id int64) (bool, error) {
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_requests SET sla_notified_at = NOW()
		WHERE id = $1 AND sla_notified_at IS NULL`, id)
	if err != nil {
		return false, fmt.Errorf("itinerary.SetSLANotified: %w", err)
	}
	return cmd.RowsAffected() == 1, nil
}

// joinStrings joins ss with sep (a tiny strings.Join avoids importing strings
// just for one call; kept local to avoid surprising the package's import list).
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	s := ss[0]
	for _, x := range ss[1:] {
		s += sep + x
	}
	return s
}

// =============================================================================
// Quote builder + deposit repository methods (PRD §3.3.2, TDD §3.4 M3 #3)
// =============================================================================

// quoteCols selects all itinerary_quotes columns. fx_rate_used is NUMERIC;
// scanned into a string to preserve precision (matches orders' fx_rate_used
// handling).
const quoteCols = `
	id, request_id, line_items, total_cny, currency, total_minor, deposit_minor,
	fx_rate_used, status, sent_at, paid_at, created_at, updated_at `

func (r *Repository) scanQuote(row pgx.Row) (*models.ItineraryQuote, error) {
	var q models.ItineraryQuote
	if err := row.Scan(
		&q.ID, &q.RequestID, &q.LineItems, &q.TotalCNY, &q.Currency,
		&q.TotalMinor, &q.DepositMinor, &q.FxRateUsed, &q.Status,
		&q.SentAt, &q.PaidAt, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *Repository) ListOptionRates(ctx context.Context) ([]models.OptionRate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, option_key, rate_cny, unit, display_label
		FROM option_rates ORDER BY option_key`)
	if err != nil {
		return nil, fmt.Errorf("itinerary.ListOptionRates: %w", err)
	}
	defer rows.Close()
	out := []models.OptionRate{}
	for rows.Next() {
		var o models.OptionRate
		if err := rows.Scan(&o.ID, &o.OptionKey, &o.RateCNY, &o.Unit, &o.DisplayLabel); err != nil {
			return nil, fmt.Errorf("itinerary.ListOptionRates.Scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repository) GetOptionRatesByKeys(ctx context.Context, keys []string) (map[string]models.OptionRate, error) {
	if len(keys) == 0 {
		return map[string]models.OptionRate{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, option_key, rate_cny, unit, display_label
		FROM option_rates WHERE option_key = ANY($1)`, keys)
	if err != nil {
		return nil, fmt.Errorf("itinerary.GetOptionRatesByKeys: %w", err)
	}
	defer rows.Close()
	out := map[string]models.OptionRate{}
	for rows.Next() {
		var o models.OptionRate
		if err := rows.Scan(&o.ID, &o.OptionKey, &o.RateCNY, &o.Unit, &o.DisplayLabel); err != nil {
			return nil, fmt.Errorf("itinerary.GetOptionRatesByKeys.Scan: %w", err)
		}
		out[o.OptionKey] = o
	}
	return out, rows.Err()
}

// CreateQuote inserts OR replaces the active quote (UNIQUE request_id; ON
// CONFLICT DO UPDATE so a re-quote replaces the prior quote's totals). The
// status resets to 'sent' + sent_at refreshed (a re-quote voids any prior
// 'sent'-stage payment intent, which the gateway side handles via the
// idempotency_key; the payment row from a prior sent quote is left in place —
// a succeeded deposit blocks the re-quote in practice).
func (r *Repository) CreateQuote(ctx context.Context, q *models.ItineraryQuote) (*models.ItineraryQuote, error) {
	out, err := r.scanQuote(r.db.QueryRow(ctx, `
		INSERT INTO itinerary_quotes (request_id, line_items, total_cny, currency,
		                              total_minor, deposit_minor, fx_rate_used, status, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'sent', NOW())
		ON CONFLICT (request_id) DO UPDATE SET
			line_items    = EXCLUDED.line_items,
			total_cny     = EXCLUDED.total_cny,
			currency      = EXCLUDED.currency,
			total_minor   = EXCLUDED.total_minor,
			deposit_minor = EXCLUDED.deposit_minor,
			fx_rate_used  = EXCLUDED.fx_rate_used,
			status        = 'sent',
			sent_at       = NOW(),
			paid_at       = NULL,
			updated_at    = NOW()
		RETURNING `+quoteCols,
		q.RequestID, q.LineItems, q.TotalCNY, q.Currency,
		q.TotalMinor, q.DepositMinor, q.FxRateUsed, // *string → numeric NULL-safe? see note
	))
	if err != nil {
		return nil, fmt.Errorf("itinerary.CreateQuote: %w", err)
	}
	return out, nil
}

func (r *Repository) GetQuoteByRequestID(ctx context.Context, requestID int64) (*models.ItineraryQuote, error) {
	q, err := r.scanQuote(r.db.QueryRow(ctx,
		`SELECT`+quoteCols+`FROM itinerary_quotes WHERE request_id = $1`, requestID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("itinerary.GetQuoteByRequestID: %w", err)
	}
	return q, nil
}

func (r *Repository) GetQuoteByID(ctx context.Context, quoteID int64) (*models.ItineraryQuote, error) {
	q, err := r.scanQuote(r.db.QueryRow(ctx,
		`SELECT`+quoteCols+`FROM itinerary_quotes WHERE id = $1`, quoteID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("itinerary.GetQuoteByID: %w", err)
	}
	return q, nil
}

// MarkQuoteDepositPaid CAS-moves the quote sent→deposit_paid (or sent→
// fully_paid if payFull) and sets paid_at. Returns true iff this call won the
// CAS (a replayed webhook returns false → idempotent no-op for the caller).
func (r *Repository) MarkQuoteDepositPaid(ctx context.Context, quoteID int64, payFull bool) (bool, error) {
	to := "deposit_paid"
	if payFull {
		to = "fully_paid"
	}
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_quotes
		SET status = $2, paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'sent'`, quoteID, to)
	if err != nil {
		return false, fmt.Errorf("itinerary.MarkQuoteDepositPaid: %w", err)
	}
	return cmd.RowsAffected() == 1, nil
}

// CancelQuote CAS-moves {sent,deposit_paid,fully_paid}→cancelled (planner-
// initiated refund flow).
func (r *Repository) CancelQuote(ctx context.Context, quoteID int64) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE itinerary_quotes SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status IN ('sent','deposit_paid','fully_paid')`, quoteID)
	if err != nil {
		return fmt.Errorf("itinerary.CancelQuote: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM itinerary_quotes WHERE id=$1)`, quoteID).Scan(&exists); err != nil {
			return fmt.Errorf("itinerary.CancelQuote.probe: %w", err)
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrConflict
	}
	return nil
}
