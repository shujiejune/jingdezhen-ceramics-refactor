package payment

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines payment storage.
type RepositoryInterface interface {
	// RecordIntent inserts a pending payment row at CreateIntent time.
	RecordIntent(ctx context.Context, p *models.Payment) (int64, error)
	// GetByID loads a payment by id.
	GetByID(ctx context.Context, id int64) (*models.Payment, error)
	// GetSucceededByOrderID returns the succeeded payment for an order (for
	// refunds). Returns ErrNotFound if none.
	GetSucceededByOrderID(ctx context.Context, orderID int64) (*models.Payment, error)
	// UpsertWebhook idempotently records a verified webhook event. Returns
	// inserted=true if a new row was created (first time this event is seen),
	// inserted=false if the idempotency_key already existed (a replay → no-op).
	// On a first-seen event that updates an existing intent's status, the
	// status is set + raw_webhook captured.
	UpsertWebhook(ctx context.Context, p *models.Payment) (inserted bool, err error)
	// MarkStatus atomically moves a payment from->to (CAS).
	MarkStatus(ctx context.Context, id int64, from, to models.PaymentStatus) error
	// SetRefunded marks a succeeded payment refunded (after gateway.Refund ok).
	SetRefunded(ctx context.Context, id int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

const paymentCols = `
	id, order_id, gateway, gateway_ref, status, amount_minor, currency,
	raw_webhook, idempotency_key, created_at, updated_at `

func (r *Repository) scanPayment(row pgx.Row) (*models.Payment, error) {
	var p models.Payment
	if err := row.Scan(
		&p.ID, &p.OrderID, &p.Gateway, &p.GatewayRef, &p.Status,
		&p.AmountMinor, &p.Currency, &p.RawWebhook, &p.IdempotencyKey,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) RecordIntent(ctx context.Context, p *models.Payment) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO payments (order_id, gateway, gateway_ref, status, amount_minor, currency, idempotency_key)
		VALUES ($1,$2,$3,'pending',$4,$5,$6)
		RETURNING id`,
		p.OrderID, p.Gateway, p.GatewayRef, p.AmountMinor, p.Currency, p.IdempotencyKey,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("payment.Repository.RecordIntent: %w", err)
	}
	return id, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*models.Payment, error) {
	p, err := r.scanPayment(r.db.QueryRow(ctx, `SELECT`+paymentCols+`FROM payments WHERE id=$1`, id))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payment.Repository.GetByID: %w", err)
	}
	return p, nil
}

func (r *Repository) GetSucceededByOrderID(ctx context.Context, orderID int64) (*models.Payment, error) {
	p, err := r.scanPayment(r.db.QueryRow(ctx,
		`SELECT`+paymentCols+`FROM payments WHERE order_id=$1 AND status='succeeded' ORDER BY id DESC LIMIT 1`, orderID))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("payment.Repository.GetSucceededByOrderID: %w", err)
	}
	return p, nil
}

// UpsertWebhook idempotently records a verified webhook. On a duplicate
// idempotency_key (a gateway replay) no new row is created and inserted=false.
// On the first event, the intent's payment row is found by gateway_ref and its
// status set to the event status (pending→succeeded/failed) + raw_webhook
// captured. inserted reports whether this call was the "first time seen".
func (r *Repository) UpsertWebhook(ctx context.Context, ev *models.Payment) (bool, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("payment.Repository.UpsertWebhook.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id int64
	// Insert the event record keyed by idempotency_key; ON CONFLICT → no-op.
	if err := tx.QueryRow(ctx, `
		INSERT INTO payments (order_id, gateway, gateway_ref, status, amount_minor, currency, raw_webhook, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (idempotency_key) DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
		RETURNING id`, ev.OrderID, ev.Gateway, ev.GatewayRef, string(ev.Status),
		ev.AmountMinor, ev.Currency, ev.RawWebhook, ev.IdempotencyKey,
	).Scan(&id); err != nil {
		return false, fmt.Errorf("payment.Repository.UpsertWebhook.Insert: %w", err)
	}
	// Was this row actually new? xmax = 0 means the row was just inserted
	// (not updated by the ON CONFLICT clause).
	var inserted bool
	if err := tx.QueryRow(ctx, `SELECT (xmax = 0) FROM payments WHERE id = $1`, id).Scan(&inserted); err != nil {
		return false, fmt.Errorf("payment.Repository.UpsertWebhook.Probe: %w", err)
	}
	if !inserted {
		// A replay of an already-seen event. Ack as no-op.
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	// First-seen event: update the intent row (the one with status='pending')
	// matching this gateway_ref to the event status. The event row itself is
	// the audit record; the intent row's status drives refunds.
	if ev.Status == models.PaymentSucceeded || ev.Status == models.PaymentFailed {
		if _, err := tx.Exec(ctx, `
			UPDATE payments SET status = $2, raw_webhook = $3, updated_at = NOW()
			WHERE gateway_ref = $1 AND status = 'pending' AND id <> $4`,
			ev.GatewayRef, string(ev.Status), ev.RawWebhook, id,
		); err != nil {
			return false, fmt.Errorf("payment.Repository.UpsertWebhook.UpdateIntent: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("payment.Repository.UpsertWebhook.Commit: %w", err)
	}
	return true, nil
}

func (r *Repository) MarkStatus(ctx context.Context, id int64, from, to models.PaymentStatus) error {
	cmd, err := r.db.Exec(ctx, `UPDATE payments SET status=$3, updated_at=NOW() WHERE id=$1 AND status=$2`,
		id, string(from), string(to))
	if err != nil {
		return fmt.Errorf("payment.Repository.MarkStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrConflict
	}
	return nil
}

func (r *Repository) SetRefunded(ctx context.Context, id int64) error {
	cmd, err := r.db.Exec(ctx, `UPDATE payments SET status='refunded', updated_at=NOW() WHERE id=$1 AND status='succeeded'`, id)
	if err != nil {
		return fmt.Errorf("payment.Repository.SetRefunded: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrConflict
	}
	return nil
}
