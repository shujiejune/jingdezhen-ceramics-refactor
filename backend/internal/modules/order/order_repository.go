package order

import (
	"context"
	"encoding/json"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines order storage.
type RepositoryInterface interface {
	// CreateOrder inserts an order + its items in one transaction and atomically
	// decrements stock for each item. Returns ErrConflict if any SKU has
	// insufficient stock (the whole transaction rolls back — no partial order).
	CreateOrder(ctx context.Context, o *models.Order, items []models.OrderItem) (int64, error)
	// GetByID loads an order + its items, scoped to a specific user (customer
	// view). Returns ErrNotFound if absent or not owned.
	GetByIDForUser(ctx context.Context, userID string, id int64) (*models.Order, error)
	// GetByID loads an order + its items (admin view, any user).
	GetByID(ctx context.Context, id int64) (*models.Order, error)
	// ListForUser paginates a user's orders (newest first).
	ListForUser(ctx context.Context, userID string, page, limit int) ([]models.Order, int, error)
	// ListAdmin paginates all orders, optionally filtered by status.
	ListAdmin(ctx context.Context, status string, page, limit int) ([]models.Order, int, error)
	// TransitionStatus atomically moves an order from->to via a CAS UPDATE,
	// setting the transition timestamp (paidAt/shippedAt/etc.). Returns
	// ErrNotFound if absent, ErrConflict if the current status doesn't match
	// `from` (concurrent transition or invalid).
	TransitionStatus(ctx context.Context, id int64, from, to models.OrderStatus, tsCol string) error
	// SetShipped records carrier + tracking + status=paid→shipped atomically.
	SetShipped(ctx context.Context, id int64, carrier, tracking string) error
	// SetCancelled marks created→cancelled and restores stock for the items.
	SetCancelled(ctx context.Context, id int64, reason string, items []models.OrderItem) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// CreateOrder runs the authoritative stock decrement (TDD §4.3):
//
//	UPDATE skus SET stock = stock - $1 WHERE id = $2 AND stock >= $1
//
// Zero rows affected → ErrConflict → rollback the whole order.
func (r *Repository) CreateOrder(ctx context.Context, o *models.Order, items []models.OrderItem) (int64, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("order.Repository.CreateOrder.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert the order header; RETURNING id.
	var orderID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO orders (
			user_id, status, currency,
			subtotal_minor, shipping_minor, total_minor,
			subtotal_cny, shipping_cny, total_cny, fx_rate_used,
			address, locale, placed_at
		) VALUES ($1,'created',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
		RETURNING id`,
		o.UserID, o.Currency,
		o.SubtotalMinor, o.ShippingMinor, o.TotalMinor,
		o.SubtotalCNY, o.ShippingCNY, o.TotalCNY, o.FxRateUsedRaw,
		o.Address, o.Locale,
	).Scan(&orderID); err != nil {
		return 0, fmt.Errorf("order.Repository.CreateOrder.InsertOrder: %w", err)
	}

	// Insert items + decrement stock atomically.
	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (
				order_id, sku_id, qty, unit_price_minor, unit_price_cny,
				title_snapshot, attributes_snapshot
			) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			orderID, it.SkuID, it.Qty, it.UnitPriceMinor, it.UnitPriceCNY,
			it.TitleSnapshot, it.AttributesSnapshot,
		); err != nil {
			return 0, fmt.Errorf("order.Repository.CreateOrder.InsertItem(sku=%d): %w", it.SkuID, err)
		}
		cmd, err := tx.Exec(ctx, `
			UPDATE skus SET stock = stock - $1 WHERE id = $2 AND stock >= $1`,
			it.Qty, it.SkuID)
		if err != nil {
			return 0, fmt.Errorf("order.Repository.CreateOrder.DecrementStock(sku=%d): %w", it.SkuID, err)
		}
		if cmd.RowsAffected() == 0 {
			// Insufficient stock — the whole order rolls back. TDD §4.3.
			return 0, fmt.Errorf("%w: insufficient stock for sku %d", models.ErrConflict, it.SkuID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("order.Repository.CreateOrder.Commit: %w", err)
	}
	return orderID, nil
}

const orderSelectCols = `
	id, user_id, status, currency,
	subtotal_minor, shipping_minor, total_minor,
	subtotal_cny, shipping_cny, total_cny, fx_rate_used,
	address, locale, carrier_name, tracking_number,
	placed_at, paid_at, shipped_at, completed_at, cancelled_at, refunded_at,
	cancel_reason, created_at, updated_at `

func (r *Repository) scanOrder(row pgx.Row) (*models.Order, error) {
	var o models.Order
	if err := row.Scan(
		&o.ID, &o.UserID, &o.Status, &o.Currency,
		&o.SubtotalMinor, &o.ShippingMinor, &o.TotalMinor,
		&o.SubtotalCNY, &o.ShippingCNY, &o.TotalCNY, &o.FxRateUsedRaw,
		&o.Address, &o.Locale, &o.CarrierName, &o.TrackingNumber,
		&o.PlacedAt, &o.PaidAt, &o.ShippedAt, &o.CompletedAt, &o.CancelledAt, &o.RefundedAt,
		&o.CancelReason, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if o.FxRateUsedRaw != nil {
		// Render the NUMERIC snapshot as a float for the API response.
		var f float64
		if _, err := fmt.Sscanf(*o.FxRateUsedRaw, "%g", &f); err == nil {
			o.FxRateUsed = &f
		}
	}
	return &o, nil
}

func (r *Repository) loadItems(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, orderID int64) ([]models.OrderItem, error) {
	rows, err := q.Query(ctx, `
		SELECT id, order_id, sku_id, qty, unit_price_minor, unit_price_cny,
		       title_snapshot, attributes_snapshot, created_at
		FROM order_items WHERE order_id = $1 ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OrderItem{}
	for rows.Next() {
		var it models.OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.SkuID, &it.Qty,
			&it.UnitPriceMinor, &it.UnitPriceCNY,
			&it.TitleSnapshot, &it.AttributesSnapshot, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repository) GetByIDForUser(ctx context.Context, userID string, id int64) (*models.Order, error) {
	o, err := r.scanOrder(r.db.QueryRow(ctx, `SELECT`+orderSelectCols+`FROM orders WHERE id = $1 AND user_id = $2`, id, userID))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("order.Repository.GetByIDForUser: %w", err)
	}
	o.Items, err = r.loadItems(ctx, r.db, o.ID)
	if err != nil {
		return nil, fmt.Errorf("order.Repository.GetByIDForUser.loadItems: %w", err)
	}
	if o.Items == nil {
		o.Items = []models.OrderItem{}
	}
	return o, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*models.Order, error) {
	o, err := r.scanOrder(r.db.QueryRow(ctx, `SELECT`+orderSelectCols+`FROM orders WHERE id = $1`, id))
	if err == pgx.ErrNoRows {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("order.Repository.GetByID: %w", err)
	}
	o.Items, err = r.loadItems(ctx, r.db, o.ID)
	if err != nil {
		return nil, fmt.Errorf("order.Repository.GetByID.loadItems: %w", err)
	}
	if o.Items == nil {
		o.Items = []models.OrderItem{}
	}
	return o, nil
}

func (r *Repository) ListForUser(ctx context.Context, userID string, page, limit int) ([]models.Order, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("order.Repository.ListForUser.Count: %w", err)
	}
	if total == 0 {
		return []models.Order{}, 0, nil
	}
	offset := (page - 1) * limit
	rows, err := r.db.Query(ctx, `SELECT`+orderSelectCols+`FROM orders WHERE user_id = $1 ORDER BY placed_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("order.Repository.ListForUser.Query: %w", err)
	}
	defer rows.Close()
	out := []models.Order{}
	for rows.Next() {
		o, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("order.Repository.ListForUser.Scan: %w", err)
		}
		out = append(out, *o)
	}
	return out, total, rows.Err()
}

func (r *Repository) ListAdmin(ctx context.Context, status string, page, limit int) ([]models.Order, int, error) {
	var (
		total int
		rows  pgx.Rows
		err   error
	)
	if status != "" {
		if err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status = $1`, status).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("order.Repository.ListAdmin.Count: %w", err)
		}
		offset := (page - 1) * limit
		rows, err = r.db.Query(ctx, `SELECT`+orderSelectCols+`FROM orders WHERE status = $1 ORDER BY placed_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	} else {
		if err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("order.Repository.ListAdmin.Count: %w", err)
		}
		offset := (page - 1) * limit
		rows, err = r.db.Query(ctx, `SELECT`+orderSelectCols+`FROM orders ORDER BY placed_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("order.Repository.ListAdmin.Query: %w", err)
	}
	defer rows.Close()
	out := []models.Order{}
	for rows.Next() {
		o, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("order.Repository.ListAdmin.Scan: %w", err)
		}
		out = append(out, *o)
	}
	return out, total, rows.Err()
}

// tsCol maps a target status to the timestamp column to set on transition.
func tsCol(to models.OrderStatus) string {
	switch to {
	case models.StatusPaid:
		return "paid_at"
	case models.StatusShipped:
		return "shipped_at"
	case models.StatusCompleted:
		return "completed_at"
	case models.StatusCancelled:
		return "cancelled_at"
	case models.StatusRefunded:
		return "refunded_at"
	}
	return ""
}

// TransitionStatus atomically moves an order from->to (CAS). Returns ErrNotFound
// if the order is absent, ErrConflict if the current status != from (concurrent
// or invalid transition). Sets the transition timestamp.
func (r *Repository) TransitionStatus(ctx context.Context, id int64, from, to models.OrderStatus, _ string) error {
	col := tsCol(to)
	if col == "" {
		return fmt.Errorf("order.Repository.TransitionStatus: no timestamp column for status %q", to)
	}
	cmd, err := r.db.Exec(ctx, `
		UPDATE orders SET status = $2, `+col+` = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = $3`,
		id, string(to), string(from))
	if err != nil {
		return fmt.Errorf("order.Repository.TransitionStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		// Distinguish "absent" from "wrong status": probe existence.
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1)`, id).Scan(&exists); err != nil {
			return fmt.Errorf("order.Repository.TransitionStatus.probe: %w", err)
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrConflict
	}
	return nil
}

func (r *Repository) SetShipped(ctx context.Context, id int64, carrier, tracking string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE orders
		SET status = 'shipped', carrier_name = $2, tracking_number = $3,
		    shipped_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'paid'`,
		id, carrier, tracking)
	if err != nil {
		return fmt.Errorf("order.Repository.SetShipped: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrConflict
	}
	return nil
}

// SetCancelled moves created→cancelled and restores stock for each item.
func (r *Repository) SetCancelled(ctx context.Context, id int64, reason string, items []models.OrderItem) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("order.Repository.SetCancelled.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	cmd, err := tx.Exec(ctx, `
		UPDATE orders SET status = 'cancelled', cancelled_at = NOW(),
		                  cancel_reason = $2, updated_at = NOW()
		WHERE id = $1 AND status = 'created'`,
		id, reason)
	if err != nil {
		return fmt.Errorf("order.Repository.SetCancelled.Update: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id=$1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return models.ErrNotFound
		}
		return models.ErrConflict
	}
	// Restore stock per item.
	for _, it := range items {
		if _, err := tx.Exec(ctx, `UPDATE skus SET stock = stock + $1 WHERE id = $2`, it.Qty, it.SkuID); err != nil {
			return fmt.Errorf("order.Repository.SetCancelled.RestoreStock(sku=%d): %w", it.SkuID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("order.Repository.SetCancelled.Commit: %w", err)
	}
	return nil
}

// (compile-time: keep json imported for snapshot marshalling in the service)
var _ = json.RawMessage(nil)
