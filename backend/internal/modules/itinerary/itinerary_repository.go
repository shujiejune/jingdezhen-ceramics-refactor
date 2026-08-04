package itinerary

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
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

	// --- Requests (submitted) ---
	CreateRequest(ctx context.Context, req *models.ItineraryRequest) (*models.ItineraryRequest, error)
	GetByIDForUser(ctx context.Context, userID string, id int64) (*models.ItineraryRequest, error)
	ListForUser(ctx context.Context, userID string, page, limit int) ([]models.ItineraryRequest, int, error)
	// CancelRequest CAS-moves pending→cancelled for a user's request. Returns
	// ErrNotFound if absent/not-owned, ErrItineraryNotCancellable if not pending.
	CancelRequest(ctx context.Context, userID string, id int64, reason string) error
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
