package media

import (
	"context"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// mediaCols is the canonical column list for scanning a media_assets row.
const mediaCols = `id, kind, oss_key, mime, width, height, duration, hls_key, uploaded_by, created_at, updated_at`

func scanAsset(row pgx.Row) (*models.MediaAsset, error) {
	var a models.MediaAsset
	if err := row.Scan(&a.ID, &a.Kind, &a.OSSKey, &a.MIME, &a.Width, &a.Height,
		&a.Duration, &a.HLSKey, &a.UploadedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// RegisterAsset inserts a media_assets row + populates a.ID/CreatedAt/UpdatedAt.
func (r *Repository) RegisterAsset(ctx context.Context, a *models.MediaAsset) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO media_assets (kind, oss_key, mime, width, height, duration, hls_key, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		a.Kind, a.OSSKey, a.MIME, a.Width, a.Height, a.Duration, a.HLSKey, a.UploadedBy,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *Repository) GetAsset(ctx context.Context, id int64) (*models.MediaAsset, error) {
	a, err := scanAsset(r.db.QueryRow(ctx, `SELECT `+mediaCols+` FROM media_assets WHERE id=$1`, id))
	if err != nil {
		return nil, fmt.Errorf("media.GetAsset: %w", err)
	}
	return a, nil
}

func (r *Repository) ListAssets(ctx context.Context, kind string, page, limit int) ([]models.MediaAsset, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	var (
		rows pgx.Rows
		err  error
	)
	if kind != "" {
		rows, err = r.db.Query(ctx,
			`SELECT `+mediaCols+` FROM media_assets WHERE kind=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			kind, limit, offset)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+mediaCols+` FROM media_assets ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("media.ListAssets: %w", err)
	}
	defer rows.Close()
	out := []models.MediaAsset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// Total count.
	var total int
	countQ := `SELECT count(*) FROM media_assets`
	if kind != "" {
		countQ += ` WHERE kind=$1`
	}
	if kind != "" {
		if err := r.db.QueryRow(ctx, countQ, kind).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		if err := r.db.QueryRow(ctx, countQ).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *Repository) DeleteAsset(ctx context.Context, id int64) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM media_assets WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("media.DeleteAsset: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// AttachToProduct inserts a product_media row. If sortOrder is nil, the media
// is appended after the current max sort_order.
func (r *Repository) AttachToProduct(ctx context.Context, productID, mediaID int64, sortOrder *int, caption *string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media.AttachToProduct.Begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	so := 0
	if sortOrder != nil {
		so = *sortOrder
	} else {
		// Append-last: current max + 1.
		var maxOrder int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(sort_order), -1) FROM product_media WHERE product_id=$1`, productID,
		).Scan(&maxOrder); err != nil {
			return fmt.Errorf("media.AttachToProduct.Max: %w", err)
		}
		so = maxOrder + 1
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO product_media (product_id, media_id, sort_order, caption)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id, media_id) DO UPDATE SET sort_order = EXCLUDED.sort_order, caption = EXCLUDED.caption`,
		productID, mediaID, so, nullableStr(caption)); err != nil {
		return fmt.Errorf("media.AttachToProduct.Insert: %w", err)
	}
	return tx.Commit(ctx)
}

// productMediaCols selects product_media.* + the joined media_assets columns
// (aliased m.*). The scan order must match the SELECT order below.
const productMediaCols = `pm.id, pm.product_id, pm.media_id, pm.sort_order, pm.caption, pm.created_at,
    m.id, m.kind, m.oss_key, m.mime, m.width, m.height, m.duration, m.hls_key, m.uploaded_by, m.created_at, m.updated_at`

func scanProductMediaItem(row pgx.Row) (*models.ProductMediaItem, error) {
	var pm models.ProductMediaItem
	if err := row.Scan(&pm.ID, &pm.ProductID, &pm.MediaID, &pm.SortOrder, &pm.Caption, &pm.CreatedAt,
		&pm.MediaAsset.ID, &pm.MediaAsset.Kind, &pm.MediaAsset.OSSKey, &pm.MediaAsset.MIME,
		&pm.MediaAsset.Width, &pm.MediaAsset.Height, &pm.MediaAsset.Duration,
		&pm.MediaAsset.HLSKey, &pm.MediaAsset.UploadedBy,
		&pm.MediaAsset.CreatedAt, &pm.MediaAsset.UpdatedAt); err != nil {
		return nil, err
	}
	return &pm, nil
}

func (r *Repository) ListProductMedia(ctx context.Context, productID int64) ([]models.ProductMediaItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+productMediaCols+`
		FROM product_media pm
		JOIN media_assets m ON m.id = pm.media_id
		WHERE pm.product_id = $1
		ORDER BY pm.sort_order ASC, pm.id ASC`, productID)
	if err != nil {
		return nil, fmt.Errorf("media.ListProductMedia: %w", err)
	}
	defer rows.Close()
	out := []models.ProductMediaItem{}
	for rows.Next() {
		pm, err := scanProductMediaItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pm)
	}
	return out, rows.Err()
}

func (r *Repository) DetachFromProduct(ctx context.Context, productID, mediaID int64) error {
	ct, err := r.db.Exec(ctx,
		`DELETE FROM product_media WHERE product_id=$1 AND media_id=$2`, productID, mediaID)
	if err != nil {
		return fmt.Errorf("media.DetachFromProduct: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// ReorderProductMedia sets sort_order for a batch in one tx.
func (r *Repository) ReorderProductMedia(ctx context.Context, productID int64, items []models.ReorderMediaItem) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media.ReorderProductMedia.Begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	for _, it := range items {
		if _, err := tx.Exec(ctx,
			`UPDATE product_media SET sort_order=$1 WHERE product_id=$2 AND media_id=$3`,
			it.SortOrder, productID, it.MediaID); err != nil {
			return fmt.Errorf("media.ReorderProductMedia.Update: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// nullableStr returns a ptr-or-NULL for pgx.
func nullableStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
