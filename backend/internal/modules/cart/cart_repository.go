package cart

import (
	"context"
	"database/sql"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines cart storage operations.
type RepositoryInterface interface {
	// GetOrCreateCart returns the user's cart ID, lazily creating the cart row.
	// carts.user_id UNIQUE guarantees the singleton.
	GetOrCreateCart(ctx context.Context, userID string) (int64, error)
	// ListItems returns the cart's items enriched with the product display info
	// for the given locale (published translations only), mirroring the
	// wishlist read path.
	ListItems(ctx context.Context, cartID int64, locale string) ([]models.CartItem, error)
	// AddItem inserts a (cart_id, sku_id) row or, if it exists, increments qty
	// by the given amount (additive — POST add-to-cart semantics).
	AddItem(ctx context.Context, cartID, skuID int64, qty int) error
	// SetItemQty sets the qty for (cart_id, sku_id) to exactly qty (absolute —
	// PATCH change-quantity semantics).
	SetItemQty(ctx context.Context, cartID, skuID int64, qty int) error
	// RemoveItem deletes a (cart_id, sku_id) row. Returns ErrNotFound if absent.
	RemoveItem(ctx context.Context, cartID, skuID int64) error
	// BulkRemove deletes all of the given sku_ids from the cart.
	BulkRemove(ctx context.Context, cartID int64, skuIDs []int64) (int, error)
	// MergeItems upserts each guest item into the cart (additive). Unknown
	// SKUs are skipped by the caller (service pre-validates). Runs in a tx so a
	// partially-merged cart never persists.
	MergeItems(ctx context.Context, cartID int64, items []models.MergeCartItem) error
	// GetItemQty returns the qty of an SKU in a cart (0 if absent). Reads
	// cart_items directly — no product JOIN, so it works without a locale.
	GetItemQty(ctx context.Context, cartID, skuID int64) (int, error)
	// SKUExists checks the SKU exists (so Add returns ErrNotFound, not a FK
	// violation, for a bad sku_id).
	SKUExists(ctx context.Context, skuID int64) (bool, error)
	// SKUStock returns the current stock for an SKU. exists=false if the SKU
	// is absent. Used by the service's qty>stock advisory guard.
	SKUStock(ctx context.Context, skuID int64) (stock int, exists bool, err error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) GetOrCreateCart(ctx context.Context, userID string) (int64, error) {
	// INSERT ... ON CONFLICT (user_id) DO UPDATE SET updated_at=NOW() ensures
	// the row exists and returns its id in a single statement (no separate
	// SELECT-then-INSERT race).
	var cartID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO carts (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = NOW()
		RETURNING id`, userID).Scan(&cartID)
	if err != nil {
		return 0, fmt.Errorf("repository.GetOrCreateCart: %w", err)
	}
	return cartID, nil
}

func (r *Repository) ListItems(ctx context.Context, cartID int64, locale string) ([]models.CartItem, error) {
	// JOIN: cart_items → skus → products → product_translations (published, locale)
	//         → artists (name via artists.name; additive fallback). Mirrors the
	//         wishlist read path exactly.
	query := `
		SELECT
		    ci.sku_id, s.sku_code, ci.qty, s.price_cny, s.price_cny * ci.qty,
		    s.stock, s.weight_grams,
		    s.product_id, pt.slug, pt.title,
		    p.thumbnail_url, a.name,
		    s.attributes, ci.created_at
		FROM cart_items ci
		JOIN skus s ON s.id = ci.sku_id
		JOIN products p ON p.id = s.product_id
		JOIN product_translations pt ON pt.product_id = p.id
		     AND pt.locale = $2 AND pt.status = 'published'
		LEFT JOIN artists a ON a.id = p.artist_id
		WHERE ci.cart_id = $1
		ORDER BY ci.created_at ASC`
	rows, err := r.db.Query(ctx, query, cartID, locale)
	if err != nil {
		return nil, fmt.Errorf("repository.ListItems.Query: %w", err)
	}
	defer rows.Close()

	out := []models.CartItem{}
	for rows.Next() {
		var item models.CartItem
		var thumbnailURL, artistName sql.NullString
		if err := rows.Scan(
			&item.SkuID, &item.SKUCode, &item.Qty, &item.UnitPriceCNY, &item.LineTotalCNY,
			&item.Stock, &item.WeightGrams,
			&item.ProductID, &item.ProductSlug, &item.ProductTitle,
			&thumbnailURL, &artistName,
			&item.Attributes, &item.AddedAt); err != nil {
			return nil, fmt.Errorf("repository.ListItems.Scan: %w", err)
		}
		if thumbnailURL.Valid {
			item.ThumbnailURL = &thumbnailURL.String
		}
		if artistName.Valid {
			item.ArtistName = &artistName.String
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.ListItems.RowsErr: %w", err)
	}
	return out, nil
}

func (r *Repository) AddItem(ctx context.Context, cartID, skuID int64, qty int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO cart_items (cart_id, sku_id, qty)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, sku_id) DO UPDATE
			SET qty = cart_items.qty + EXCLUDED.qty,
			    updated_at = NOW()`,
		cartID, skuID, qty)
	if err != nil {
		return fmt.Errorf("repository.AddItem: %w", err)
	}
	return nil
}

func (r *Repository) SetItemQty(ctx context.Context, cartID, skuID int64, qty int) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE cart_items SET qty = $3, updated_at = NOW()
		WHERE cart_id = $1 AND sku_id = $2`,
		cartID, skuID, qty)
	if err != nil {
		return fmt.Errorf("repository.SetItemQty: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) RemoveItem(ctx context.Context, cartID, skuID int64) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND sku_id = $2`,
		cartID, skuID)
	if err != nil {
		return fmt.Errorf("repository.RemoveItem: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) BulkRemove(ctx context.Context, cartID int64, skuIDs []int64) (int, error) {
	if len(skuIDs) == 0 {
		return 0, nil
	}
	// ANY($2) matches against an int8[] array parameter.
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM cart_items WHERE cart_id = $1 AND sku_id = ANY($2)`,
		cartID, skuIDs)
	if err != nil {
		return 0, fmt.Errorf("repository.BulkRemove: %w", err)
	}
	return int(cmd.RowsAffected()), nil
}

func (r *Repository) MergeItems(ctx context.Context, cartID int64, items []models.MergeCartItem) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("repository.MergeItems.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // committed on success

	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cart_items (cart_id, sku_id, qty)
			VALUES ($1, $2, $3)
			ON CONFLICT (cart_id, sku_id) DO UPDATE
				SET qty = cart_items.qty + EXCLUDED.qty,
				    updated_at = NOW()`,
			cartID, it.SkuID, it.Qty); err != nil {
			return fmt.Errorf("repository.MergeItems.Upsert: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository.MergeItems.Commit: %w", err)
	}
	return nil
}

func (r *Repository) SKUExists(ctx context.Context, skuID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM skus WHERE id = $1)`, skuID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("repository.SKUExists: %w", err)
	}
	return exists, nil
}

func (r *Repository) SKUStock(ctx context.Context, skuID int64) (int, bool, error) {
	var stock int
	err := r.db.QueryRow(ctx, `SELECT stock FROM skus WHERE id = $1`, skuID).Scan(&stock)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("repository.SKUStock: %w", err)
	}
	return stock, true, nil
}

func (r *Repository) GetItemQty(ctx context.Context, cartID, skuID int64) (int, error) {
	// No product JOIN — reads cart_items directly so it works without a locale
	// (unlike ListItems, which requires a valid locale for the translation JOIN).
	var qty int
	err := r.db.QueryRow(ctx,
		`SELECT qty FROM cart_items WHERE cart_id = $1 AND sku_id = $2`,
		cartID, skuID).Scan(&qty)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("repository.GetItemQty: %w", err)
	}
	return qty, nil
}
