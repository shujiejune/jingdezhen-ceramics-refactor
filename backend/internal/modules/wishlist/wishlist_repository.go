package wishlist

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines wishlist storage operations.
type RepositoryInterface interface {
	// List returns the user's wishlist items enriched with the product display
	// info for the given locale (published translations only). Paginated.
	List(ctx context.Context, userID string, locale string, page, limit int) ([]models.WishlistItem, int, error)
	// Add inserts a (user_id, sku_id) row. Idempotent (ON CONFLICT DO NOTHING).
	Add(ctx context.Context, userID string, skuID int64) error
	// Remove deletes a (user_id, sku_id) row.
	Remove(ctx context.Context, userID string, skuID int64) error
	// SKUExists checks the SKU exists (so Add returns ErrNotFound, not a FK
	// violation, for a bad sku_id).
	SKUExists(ctx context.Context, skuID int64) (bool, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, userID string, locale string, page, limit int) ([]models.WishlistItem, int, error) {
	// Total count for pagination.
	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM wishlists WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.List.Count: %w", err)
	}
	if total == 0 {
		return []models.WishlistItem{}, 0, nil
	}

	offset := (page - 1) * limit
	// JOIN: wishlists → skus → products → product_translations (published, locale)
	//         → artists (name via artists.name; additive fallback until the artist
	//         module's artist_translations is joined by the service if needed).
	// For now the artist name comes from the artists.name column (retained,
	// additive). A future join to artist_translations can replace this.
	query := `
		SELECT
		    w.sku_id, s.sku_code, s.price_cny, s.stock,
		    s.product_id, pt.slug, pt.title,
		    p.thumbnail_url, a.name,
		    s.attributes, w.created_at
		FROM wishlists w
		JOIN skus s ON s.id = w.sku_id
		JOIN products p ON p.id = s.product_id
		JOIN product_translations pt ON pt.product_id = p.id
		     AND pt.locale = $2 AND pt.status = 'published'
		LEFT JOIN artists a ON a.id = p.artist_id
		WHERE w.user_id = $1
		ORDER BY w.created_at DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, query, userID, locale, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.List.Query: %w", err)
	}
	defer rows.Close()

	out := []models.WishlistItem{}
	for rows.Next() {
		var item models.WishlistItem
		var thumbnailURL, artistName sql.NullString
		if err := rows.Scan(
			&item.SkuID, &item.SKUCode, &item.PriceCNY, &item.Stock,
			&item.ProductID, &item.ProductSlug, &item.ProductTitle,
			&thumbnailURL, &artistName,
			&item.Attributes, &item.FavoritedAt); err != nil {
			return nil, 0, fmt.Errorf("repository.List.Scan: %w", err)
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
		return nil, 0, fmt.Errorf("repository.List.RowsErr: %w", err)
	}
	return out, total, nil
}

func (r *Repository) Add(ctx context.Context, userID string, skuID int64) error {
	cmd, err := r.db.Exec(ctx,
		`INSERT INTO wishlists (user_id, sku_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, skuID)
	if err != nil {
		// A FK violation means the sku_id doesn't exist — but we pre-check
		// with SKUExists, so this shouldn't normally surface.
		return fmt.Errorf("repository.Add: %w", err)
	}
	_ = cmd // ON CONFLICT DO NOTHING means RowsAffected may be 0 (already favorited)
	return nil
}

func (r *Repository) Remove(ctx context.Context, userID string, skuID int64) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM wishlists WHERE user_id = $1 AND sku_id = $2`,
		userID, skuID)
	if err != nil {
		return fmt.Errorf("repository.Remove: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
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

// (unused but kept for potential future use)
var _ = errors.Is
var _ = pgx.ErrNoRows
