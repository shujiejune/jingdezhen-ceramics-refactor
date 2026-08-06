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

// =============================================================================
// Entity galleries (artist / ceramic-story / activity) — PRD §3.1.2/§3.1.3.
//
// These mirror product_media exactly; only the join table + FK column differ.
// The shared helpers below (attachGallery / listGallery / detachGallery /
// reorderGallery) take the table + FK-column names so the three entities reuse
// one code path. GalleryItem drops the per-entity FK (the caller already knows
// which entity it asked for); product_media keeps its own ProductMediaItem
// (with ProductID) for API stability.
// =============================================================================

// galleryMediaCols selects the *_media columns + the joined media_assets cols.
// The scan order must match the SELECT order in listGallery.
const galleryMediaCols = `g.id, g.media_id, g.sort_order, g.caption, g.created_at,
    m.id, m.kind, m.oss_key, m.mime, m.width, m.height, m.duration, m.hls_key, m.uploaded_by, m.created_at, m.updated_at`

func scanGalleryItem(row pgx.Row) (*models.GalleryItem, error) {
	var g models.GalleryItem
	if err := row.Scan(&g.ID, &g.MediaID, &g.SortOrder, &g.Caption, &g.CreatedAt,
		&g.MediaAsset.ID, &g.MediaAsset.Kind, &g.MediaAsset.OSSKey, &g.MediaAsset.MIME,
		&g.MediaAsset.Width, &g.MediaAsset.Height, &g.MediaAsset.Duration,
		&g.MediaAsset.HLSKey, &g.MediaAsset.UploadedBy,
		&g.MediaAsset.CreatedAt, &g.MediaAsset.UpdatedAt); err != nil {
		return nil, err
	}
	return &g, nil
}

// attachGallery inserts a *_media row; sortOrder nil = append-last.
func (r *Repository) attachGallery(ctx context.Context, table, fkCol string, entityID, mediaID int64, sortOrder *int, caption *string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media.attachGallery.Begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	so := 0
	if sortOrder != nil {
		so = *sortOrder
	} else {
		// Append-last: current max + 1.
		q := fmt.Sprintf(`SELECT COALESCE(MAX(sort_order), -1) FROM %s WHERE %s = $1`, table, fkCol)
		var maxOrder int
		if err := tx.QueryRow(ctx, q, entityID).Scan(&maxOrder); err != nil {
			return fmt.Errorf("media.attachGallery.Max: %w", err)
		}
		so = maxOrder + 1
	}
	// table + fkCol are internal constants (never user input) → safe to interpolate.
	q := fmt.Sprintf(`INSERT INTO %s (%s, media_id, sort_order, caption)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (%s, media_id) DO UPDATE SET sort_order = EXCLUDED.sort_order, caption = EXCLUDED.caption`,
		table, fkCol, fkCol)
	if _, err := tx.Exec(ctx, q, entityID, mediaID, so, nullableStr(caption)); err != nil {
		return fmt.Errorf("media.attachGallery.Insert: %w", err)
	}
	return tx.Commit(ctx)
}

// listGallery loads an entity's ordered gallery (media joined in).
func (r *Repository) listGallery(ctx context.Context, table, fkCol string, entityID int64) ([]models.GalleryItem, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s g JOIN media_assets m ON m.id = g.media_id
		WHERE g.%s = $1 ORDER BY g.sort_order ASC, g.id ASC`, galleryMediaCols, table, fkCol)
	rows, err := r.db.Query(ctx, q, entityID)
	if err != nil {
		return nil, fmt.Errorf("media.listGallery: %w", err)
	}
	defer rows.Close()
	out := []models.GalleryItem{}
	for rows.Next() {
		g, err := scanGalleryItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

// detachGallery deletes a *_media row.
func (r *Repository) detachGallery(ctx context.Context, table, fkCol string, entityID, mediaID int64) error {
	q := fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND media_id = $2`, table, fkCol)
	ct, err := r.db.Exec(ctx, q, entityID, mediaID)
	if err != nil {
		return fmt.Errorf("media.detachGallery: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// reorderGallery sets sort_order for a batch in one tx.
func (r *Repository) reorderGallery(ctx context.Context, table, fkCol string, entityID int64, items []models.ReorderMediaItem) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media.reorderGallery.Begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	q := fmt.Sprintf(`UPDATE %s SET sort_order = $1 WHERE %s = $2 AND media_id = $3`, table, fkCol)
	for _, it := range items {
		if _, err := tx.Exec(ctx, q, it.SortOrder, entityID, it.MediaID); err != nil {
			return fmt.Errorf("media.reorderGallery.Update: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// --- Artist gallery ---

func (r *Repository) AttachToArtist(ctx context.Context, artistID, mediaID int64, sortOrder *int, caption *string) error {
	return r.attachGallery(ctx, "artist_media", "artist_id", artistID, mediaID, sortOrder, caption)
}
func (r *Repository) ListArtistMedia(ctx context.Context, artistID int64) ([]models.GalleryItem, error) {
	return r.listGallery(ctx, "artist_media", "artist_id", artistID)
}
func (r *Repository) DetachFromArtist(ctx context.Context, artistID, mediaID int64) error {
	return r.detachGallery(ctx, "artist_media", "artist_id", artistID, mediaID)
}
func (r *Repository) ReorderArtistMedia(ctx context.Context, artistID int64, items []models.ReorderMediaItem) error {
	return r.reorderGallery(ctx, "artist_media", "artist_id", artistID, items)
}

// --- Ceramic story gallery ---

func (r *Repository) AttachToStory(ctx context.Context, storyID, mediaID int64, sortOrder *int, caption *string) error {
	return r.attachGallery(ctx, "ceramic_story_media", "story_id", storyID, mediaID, sortOrder, caption)
}
func (r *Repository) ListStoryMedia(ctx context.Context, storyID int64) ([]models.GalleryItem, error) {
	return r.listGallery(ctx, "ceramic_story_media", "story_id", storyID)
}
func (r *Repository) DetachFromStory(ctx context.Context, storyID, mediaID int64) error {
	return r.detachGallery(ctx, "ceramic_story_media", "story_id", storyID, mediaID)
}
func (r *Repository) ReorderStoryMedia(ctx context.Context, storyID int64, items []models.ReorderMediaItem) error {
	return r.reorderGallery(ctx, "ceramic_story_media", "story_id", storyID, items)
}

// --- Activity gallery ---

func (r *Repository) AttachToActivity(ctx context.Context, activityID, mediaID int64, sortOrder *int, caption *string) error {
	return r.attachGallery(ctx, "activity_media", "activity_id", activityID, mediaID, sortOrder, caption)
}
func (r *Repository) ListActivityMedia(ctx context.Context, activityID int64) ([]models.GalleryItem, error) {
	return r.listGallery(ctx, "activity_media", "activity_id", activityID)
}
func (r *Repository) DetachFromActivity(ctx context.Context, activityID, mediaID int64) error {
	return r.detachGallery(ctx, "activity_media", "activity_id", activityID, mediaID)
}
func (r *Repository) ReorderActivityMedia(ctx context.Context, activityID int64, items []models.ReorderMediaItem) error {
	return r.reorderGallery(ctx, "activity_media", "activity_id", activityID, items)
}
