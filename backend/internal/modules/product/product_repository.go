package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines product/SKU storage operations (i18n-aware).
type RepositoryInterface interface {
	// --- Public reads ---
	FindAllPublished(ctx context.Context, locale, category string, artistID int64, tags []string, page, limit int) ([]models.Product, int, error)
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Product, error)
	FindSKUsByProductID(ctx context.Context, productID int64) ([]models.SKU, error)
	FindLowStock(ctx context.Context, skuIDs []int64) ([]models.SKU, error) // stock <= low_stock_threshold

	// --- Admin / CMS (products) ---
	FindAllAdmin(ctx context.Context, locale, status string, tags []string, page, limit int) ([]models.Product, int, error)
	FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Product, error)
	FindAdminByID(ctx context.Context, productID int64, locale string) (*models.Product, error)
	CreateWithTranslation(ctx context.Context, data models.CreateProductData) (*models.Product, error)
	UpdateTranslation(ctx context.Context, productID int64, locale string, data models.UpdateProductData) (*models.Product, error)
	GetTranslationStatus(ctx context.Context, productID int64, locale string) (models.ContentStatus, error)
	UpdateTranslationStatus(ctx context.Context, productID int64, locale string, status models.ContentStatus, reviewerID *string) error
	Delete(ctx context.Context, productID int64) error

	// --- Admin / CMS (SKUs) ---
	CreateSKU(ctx context.Context, productID int64, data models.CreateSKUData) (*models.SKU, error)
	UpdateSKU(ctx context.Context, skuID int64, data models.UpdateSKUData) (*models.SKU, error)
	DeleteSKU(ctx context.Context, skuID int64) error
	FindSKUByID(ctx context.Context, skuID int64) (*models.SKU, error)

	// --- Tags (PRD §3.2.1, TDD §3.2) ---
	// SetProductTags replaces a product's full tag set (absolute). Unknown keys
	// are created inline with an en-US name defaulting to the key. Empty keys =
	// clear all. Runs in the caller's tx when passed one.
	SetProductTags(ctx context.Context, exec pgx.Tx, productID int64, keys []string) error
	// FindTagsByProductIDs batch-loads tags (locale-resolved name) for a set of
	// products — returns a map[productID][]Tag for attachment without N+1.
	FindTagsByProductIDs(ctx context.Context, productIDs []int64, locale string) (map[int64][]models.Tag, error)
	// FindAllTagsInUse lists tags attached to ≥1 published product, with the
	// locale-resolved name + a product count (for the public facet list).
	FindAllTagsInUse(ctx context.Context, locale string) ([]models.TagWithCount, error)

	// --- Catalog helpers ---
	FindAllCategories(ctx context.Context) ([]string, error)
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// Columns selected from the JOIN of parent + translation, in scan order.
const productJoinColumns = `
    p.id, p.artist_id, p.category, p.thumbnail_url, p.display_order,
    p.created_at, p.updated_at,
    t.title, t.slug, t.description, t.meta_title, t.meta_description,
    t.locale, t.status, t.published_at
`

const productJoinFrom = `
    FROM products p
    JOIN product_translations t ON t.product_id = p.id
`

func (r *Repository) scanProduct(row pgx.Row) (*models.Product, error) {
	var pr models.Product
	var artistID sql.NullInt64
	var category, thumbnailURL, description, metaTitle, metaDesc *string
	err := row.Scan(
		&pr.ID, &artistID, &category, &thumbnailURL, &pr.DisplayOrder,
		&pr.CreatedAt, &pr.UpdatedAt,
		&pr.Title, &pr.Slug, &description, &metaTitle, &metaDesc,
		&pr.Locale, &pr.Status, &pr.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	if artistID.Valid {
		id := artistID.Int64
		pr.ArtistID = &id
	}
	pr.Category = category
	pr.ThumbnailURL = thumbnailURL
	pr.Description = description
	pr.MetaTitle = metaTitle
	pr.MetaDescription = metaDesc
	return &pr, nil
}

// --- Public reads ---

func (r *Repository) FindAllPublished(ctx context.Context, locale, category string, artistID int64, tags []string, page, limit int) ([]models.Product, int, error) {
	where := "WHERE t.locale = $1 AND t.status = 'published'"
	args := []any{locale}
	idx := 2
	if category != "" {
		where += fmt.Sprintf(" AND p.category = $%d", idx)
		args = append(args, category)
		idx++
	}
	if artistID > 0 {
		where += fmt.Sprintf(" AND p.artist_id = $%d", idx)
		args = append(args, artistID)
		idx++
	}
	if len(tags) > 0 {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM product_tags pt JOIN tags tg ON tg.id=pt.tag_id WHERE pt.product_id=p.id AND tg.key = ANY($%d))", idx)
		args = append(args, tags)
		idx++
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+productJoinFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Count: %w", err)
	}
	if total == 0 {
		return []models.Product{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + productJoinColumns + productJoinFrom + where +
		fmt.Sprintf(" ORDER BY p.display_order ASC, p.created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Product{}
	for rows.Next() {
		pr, err := r.scanProduct(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllPublished.Scan: %w", err)
		}
		out = append(out, *pr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.RowsErr: %w", err)
	}
	return out, total, nil
}

func (r *Repository) FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Product, error) {
	query := `SELECT ` + productJoinColumns + productJoinFrom + `
		WHERE t.locale = $1 AND t.slug = $2 AND t.status = 'published'`
	pr, err := r.scanProduct(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindPublishedBySlug: %w", err)
	}
	return pr, nil
}

func (r *Repository) FindSKUsByProductID(ctx context.Context, productID int64) ([]models.SKU, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, product_id, sku_code, price_cny, stock, weight_grams,
		       low_stock_threshold, attributes, is_active, created_at, updated_at
		FROM skus WHERE product_id = $1 AND is_active = TRUE
		ORDER BY id ASC`, productID)
	if err != nil {
		return nil, fmt.Errorf("repository.FindSKUsByProductID: %w", err)
	}
	defer rows.Close()

	out := []models.SKU{}
	for rows.Next() {
		var s models.SKU
		if err := rows.Scan(&s.ID, &s.ProductID, &s.SKUCode, &s.PriceCNY, &s.Stock,
			&s.WeightGrams, &s.LowStockThreshold, &s.Attributes, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repository.FindSKUsByProductID.Scan: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}

// FindLowStock returns the subset of the given SKUs whose stock is at or below
// their low_stock_threshold (PRD §3.4.1: default 2). Called by the stock:check
// worker after an order is paid (stock already decremented at checkout).
func (r *Repository) FindLowStock(ctx context.Context, skuIDs []int64) ([]models.SKU, error) {
	if len(skuIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, product_id, sku_code, price_cny, stock, weight_grams,
		       low_stock_threshold, attributes, is_active, created_at, updated_at
		FROM skus
		WHERE id = ANY($1) AND stock <= low_stock_threshold
		ORDER BY id ASC`, skuIDs)
	if err != nil {
		return nil, fmt.Errorf("repository.FindLowStock: %w", err)
	}
	defer rows.Close()
	out := []models.SKU{}
	for rows.Next() {
		var s models.SKU
		if err := rows.Scan(&s.ID, &s.ProductID, &s.SKUCode, &s.PriceCNY, &s.Stock,
			&s.WeightGrams, &s.LowStockThreshold, &s.Attributes, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repository.FindLowStock.Scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// --- Admin / CMS (products) ---

func (r *Repository) FindAllAdmin(ctx context.Context, locale, status string, tags []string, page, limit int) ([]models.Product, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	idx := 1
	if locale != "" {
		where += fmt.Sprintf(" AND t.locale = $%d", idx)
		args = append(args, locale)
		idx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND t.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if len(tags) > 0 {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM product_tags pt JOIN tags tg ON tg.id=pt.tag_id WHERE pt.product_id=p.id AND tg.key = ANY($%d))", idx)
		args = append(args, tags)
		idx++
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+productJoinFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.Product{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + productJoinColumns + productJoinFrom + where +
		fmt.Sprintf(" ORDER BY t.updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Product{}
	for rows.Next() {
		pr, err := r.scanProduct(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllAdmin.Scan: %w", err)
		}
		out = append(out, *pr)
	}
	return out, total, nil
}

func (r *Repository) FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Product, error) {
	query := `SELECT ` + productJoinColumns + productJoinFrom + ` WHERE t.locale = $1 AND t.slug = $2`
	pr, err := r.scanProduct(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminBySlug: %w", err)
	}
	return pr, nil
}

func (r *Repository) FindAdminByID(ctx context.Context, productID int64, locale string) (*models.Product, error) {
	query := `SELECT ` + productJoinColumns + productJoinFrom + ` WHERE p.id = $1 AND t.locale = $2`
	pr, err := r.scanProduct(r.db.QueryRow(ctx, query, productID, locale))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminByID: %w", err)
	}
	return pr, nil
}

func (r *Repository) CreateWithTranslation(ctx context.Context, data models.CreateProductData) (*models.Product, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var productID int64
	parentQuery := `
		INSERT INTO products (artist_id, category, thumbnail_url, display_order)
		VALUES ($1, $2, $3, $4) RETURNING id`
	var artistID any
	if data.ArtistID != nil {
		artistID = *data.ArtistID
	}
	if err := tx.QueryRow(ctx, parentQuery,
		artistID, nullableStr(data.Category), nullableStr(data.ThumbnailURL), data.DisplayOrder).Scan(&productID); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Parent: %w", err)
	}

	transQuery := `
		INSERT INTO product_translations
		    (product_id, locale, title, slug, description, meta_title, meta_description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.Exec(ctx, transQuery,
		productID, data.Locale, data.Title, data.Slug,
		nullableStr(data.Description), nullableStr(data.MetaTitle), nullableStr(data.MetaDescription)); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Translation: %w", err)
	}

	// Tag assignment (absolute set). Unknown keys are created inline.
	if len(data.Tags) > 0 {
		if err := r.SetProductTags(ctx, tx, productID, data.Tags); err != nil {
			return nil, fmt.Errorf("repository.CreateWithTranslation.Tags: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, productID, data.Locale)
}

func (r *Repository) UpdateTranslation(ctx context.Context, productID int64, locale string, data models.UpdateProductData) (*models.Product, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Parent non-localized fields.
	parentSets := []string{}
	parentArgs := []any{}
	pidx := 1
	if data.ArtistID != nil {
		parentSets = append(parentSets, fmt.Sprintf("artist_id = $%d", pidx))
		parentArgs = append(parentArgs, *data.ArtistID)
		pidx++
	}
	if data.Category != nil {
		parentSets = append(parentSets, fmt.Sprintf("category = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.Category))
		pidx++
	}
	if data.ThumbnailURL != nil {
		parentSets = append(parentSets, fmt.Sprintf("thumbnail_url = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.ThumbnailURL))
		pidx++
	}
	if data.DisplayOrder != nil {
		parentSets = append(parentSets, fmt.Sprintf("display_order = $%d", pidx))
		parentArgs = append(parentArgs, *data.DisplayOrder)
		pidx++
	}
	if len(parentSets) > 0 {
		parentSets = append(parentSets, "updated_at = NOW()")
		parentArgs = append(parentArgs, productID)
		pq := fmt.Sprintf(`UPDATE products SET %s WHERE id = $%d`, strings.Join(parentSets, ", "), pidx)
		if _, err := tx.Exec(ctx, pq, parentArgs...); err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Parent: %w", err)
		}
	}

	// Translation localized fields.
	transSets := []string{}
	transArgs := []any{}
	tidx := 1
	if data.Title != nil {
		transSets = append(transSets, fmt.Sprintf("title = $%d", tidx))
		transArgs = append(transArgs, *data.Title)
		tidx++
	}
	if data.Slug != nil {
		transSets = append(transSets, fmt.Sprintf("slug = $%d", tidx))
		transArgs = append(transArgs, *data.Slug)
		tidx++
	}
	if data.Description != nil {
		transSets = append(transSets, fmt.Sprintf("description = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.Description))
		tidx++
	}
	if data.MetaTitle != nil {
		transSets = append(transSets, fmt.Sprintf("meta_title = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.MetaTitle))
		tidx++
	}
	if data.MetaDescription != nil {
		transSets = append(transSets, fmt.Sprintf("meta_description = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.MetaDescription))
		tidx++
	}
	if len(transSets) > 0 {
		transSets = append(transSets, "updated_at = NOW()")
		transArgs = append(transArgs, productID, locale)
		tq := fmt.Sprintf(`UPDATE product_translations SET %s WHERE product_id = $%d AND locale = $%d`,
			strings.Join(transSets, ", "), tidx, tidx+1)
		cmd, err := tx.Exec(ctx, tq, transArgs...)
		if err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Translation: %w", err)
		}
		if cmd.RowsAffected() == 0 {
			return nil, models.ErrNotFound
		}
	}

	// Tag assignment (absolute set: nil = unchanged, empty = clear all).
	if data.Tags != nil {
		if err := r.SetProductTags(ctx, tx, productID, *data.Tags); err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Tags: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, productID, locale)
}

func (r *Repository) GetTranslationStatus(ctx context.Context, productID int64, locale string) (models.ContentStatus, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM product_translations WHERE product_id = $1 AND locale = $2`,
		productID, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", models.ErrNotFound
		}
		return "", fmt.Errorf("repository.GetTranslationStatus: %w", err)
	}
	return models.ContentStatus(status), nil
}

func (r *Repository) UpdateTranslationStatus(ctx context.Context, productID int64, locale string, status models.ContentStatus, reviewerID *string) error {
	var publishedAt any
	if status == models.StatusPublished {
		publishedAt = time.Now()
	} else {
		publishedAt = nil
	}
	cmd, err := r.db.Exec(ctx,
		`UPDATE product_translations
		    SET status = $3, reviewed_by = $4, published_at = $5, updated_at = NOW()
		    WHERE product_id = $1 AND locale = $2`,
		productID, locale, string(status), reviewerID, publishedAt)
	if err != nil {
		return fmt.Errorf("repository.UpdateTranslationStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, productID int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
	if err != nil {
		return fmt.Errorf("repository.Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// --- Admin / CMS (SKUs) ---

func (r *Repository) FindSKUByID(ctx context.Context, skuID int64) (*models.SKU, error) {
	var s models.SKU
	err := r.db.QueryRow(ctx, `
		SELECT id, product_id, sku_code, price_cny, stock, weight_grams,
		       low_stock_threshold, attributes, is_active, created_at, updated_at
		FROM skus WHERE id = $1`, skuID).Scan(
		&s.ID, &s.ProductID, &s.SKUCode, &s.PriceCNY, &s.Stock, &s.WeightGrams,
		&s.LowStockThreshold, &s.Attributes, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindSKUByID: %w", err)
	}
	return &s, nil
}

func (r *Repository) CreateSKU(ctx context.Context, productID int64, data models.CreateSKUData) (*models.SKU, error) {
	threshold := 2 // PRD §3.2.1 default
	if data.LowStockThreshold != nil {
		threshold = *data.LowStockThreshold
	}
	isActive := true
	if data.IsActive != nil {
		isActive = *data.IsActive
	}
	attrs := data.Attributes
	if len(attrs) == 0 {
		attrs = []byte("{}")
	}

	var s models.SKU
	err := r.db.QueryRow(ctx, `
		INSERT INTO skus (product_id, sku_code, price_cny, stock, weight_grams,
		                 low_stock_threshold, attributes, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, product_id, sku_code, price_cny, stock, weight_grams,
		          low_stock_threshold, attributes, is_active, created_at, updated_at`,
		productID, data.SKUCode, data.PriceCNY, data.Stock, data.WeightGrams,
		threshold, attrs, isActive).Scan(
		&s.ID, &s.ProductID, &s.SKUCode, &s.PriceCNY, &s.Stock, &s.WeightGrams,
		&s.LowStockThreshold, &s.Attributes, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateSKU: %w", err)
	}
	return &s, nil
}

func (r *Repository) UpdateSKU(ctx context.Context, skuID int64, data models.UpdateSKUData) (*models.SKU, error) {
	sets := []string{}
	args := []any{}
	idx := 1
	if data.SKUCode != nil {
		sets = append(sets, fmt.Sprintf("sku_code = $%d", idx))
		args = append(args, *data.SKUCode)
		idx++
	}
	if data.PriceCNY != nil {
		sets = append(sets, fmt.Sprintf("price_cny = $%d", idx))
		args = append(args, *data.PriceCNY)
		idx++
	}
	if data.Stock != nil {
		sets = append(sets, fmt.Sprintf("stock = $%d", idx))
		args = append(args, *data.Stock)
		idx++
	}
	if data.WeightGrams != nil {
		sets = append(sets, fmt.Sprintf("weight_grams = $%d", idx))
		args = append(args, *data.WeightGrams)
		idx++
	}
	if data.LowStockThreshold != nil {
		sets = append(sets, fmt.Sprintf("low_stock_threshold = $%d", idx))
		args = append(args, *data.LowStockThreshold)
		idx++
	}
	if len(data.Attributes) > 0 {
		sets = append(sets, fmt.Sprintf("attributes = $%d", idx))
		args = append(args, data.Attributes)
		idx++
	}
	if data.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, *data.IsActive)
		idx++
	}
	if len(sets) == 0 {
		// Nothing to update — return current state.
		return r.FindSKUByID(ctx, skuID)
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, skuID)
	query := fmt.Sprintf(`UPDATE skus SET %s WHERE id = $%d`,
		strings.Join(sets, ", "), idx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateSKU: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return nil, models.ErrNotFound
	}
	return r.FindSKUByID(ctx, skuID)
}

func (r *Repository) DeleteSKU(ctx context.Context, skuID int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM skus WHERE id = $1`, skuID)
	if err != nil {
		return fmt.Errorf("repository.DeleteSKU: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// nullableStr returns nil for an empty string so pgx writes SQL NULL.
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// --- Tags (PRD §3.2.1, TDD §3.2) ---

// SetProductTags replaces a product's full tag set (absolute assignment).
// Unknown keys are created inline with an en-US name defaulting to the key
// itself (the operator edits the localized name later from the CMS). Empty
// keys clears all tags. The caller's tx is used so tag assignment commits or
// rolls back with the product write.
func (r *Repository) SetProductTags(ctx context.Context, exec pgx.Tx, productID int64, keys []string) error {
	// Normalize + dedupe (case-insensitive, trim). The tags.key CHECK enforces
	// lowercase kebab-case; reject malformed keys early.
	seen := map[string]struct{}{}
	norm := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		norm = append(norm, k)
	}

	// Clear the existing set, then (re)attach. Always run the DELETE so an empty
	// `keys` slice clears all tags (the absolute-set contract).
	if _, err := exec.Exec(ctx, `DELETE FROM product_tags WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("SetProductTags.Delete: %w", err)
	}
	if len(norm) == 0 {
		return nil
	}

	// Resolve-or-create tag ids. ON CONFLICT (key) DO UPDATE is a no-op on the
	// row but lets RETURNING id fire for both insert + conflict paths.
	rows, err := exec.Query(ctx,
		`INSERT INTO tags (key) SELECT UNNEST($1::text[]) ON CONFLICT (key) DO UPDATE SET key = EXCLUDED.key RETURNING id`,
		norm)
	if err != nil {
		return fmt.Errorf("SetProductTags.ResolveTags: %w", err)
	}
	tagIDs := make([]int64, 0, len(norm))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("SetProductTags.ScanTagID: %w", err)
		}
		tagIDs = append(tagIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("SetProductTags.ResolveTagsRows: %w", err)
	}

	// Seed an en-US display name for newly-created tags (name = key). Existing
	// translations are left untouched (ON CONFLICT DO NOTHING). This makes the
	// tag immediately displayable in any locale via the en-US → key fallback.
	if _, err := exec.Exec(ctx, `
		INSERT INTO tag_translations (tag_id, locale, name)
		SELECT id, 'en-US', key FROM tags WHERE id = ANY($1)
		ON CONFLICT (tag_id, locale) DO NOTHING`, tagIDs); err != nil {
		return fmt.Errorf("SetProductTags.SeedEnUS: %w", err)
	}

	// Attach the tags to the product. ON CONFLICT guards a re-attach after a
	// partial set (defensive; the DELETE above already cleared the set).
	if _, err := exec.Exec(ctx, `
		INSERT INTO product_tags (product_id, tag_id)
		SELECT $1, UNNEST($2::bigint[])
		ON CONFLICT DO NOTHING`, productID, tagIDs); err != nil {
		return fmt.Errorf("SetProductTags.Attach: %w", err)
	}
	return nil
}

// FindTagsByProductIDs batch-loads tags (locale-resolved name) for a set of
// products. Returns a map[productID][]Tag for attachment without N+1. The name
// resolves via the requested locale, falling back to en-US, then to the raw
// key (never blank).
func (r *Repository) FindTagsByProductIDs(ctx context.Context, productIDs []int64, locale string) (map[int64][]models.Tag, error) {
	out := map[int64][]models.Tag{}
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT pt.product_id, t.id, t.key,
		       COALESCE(tt.name, tt_en.name, t.key) AS name
		FROM product_tags pt
		JOIN tags t ON t.id = pt.tag_id
		LEFT JOIN tag_translations tt      ON tt.tag_id = t.id      AND tt.locale = $2
		LEFT JOIN tag_translations tt_en   ON tt_en.tag_id = t.id   AND tt_en.locale = 'en-US'
		WHERE pt.product_id = ANY($1)
		ORDER BY t.key`, productIDs, locale)
	if err != nil {
		return nil, fmt.Errorf("repository.FindTagsByProductIDs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid int64
		var tg models.Tag
		if err := rows.Scan(&pid, &tg.ID, &tg.Key, &tg.Name); err != nil {
			return nil, fmt.Errorf("repository.FindTagsByProductIDs.Scan: %w", err)
		}
		out[pid] = append(out[pid], tg)
	}
	return out, rows.Err()
}

// FindAllTagsInUse lists tags attached to ≥1 published product, with the
// locale-resolved name + a product count (for the public facet list,
// GET /catalog/tags).
func (r *Repository) FindAllTagsInUse(ctx context.Context, locale string) ([]models.TagWithCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.key,
		       COALESCE(tt.name, tt_en.name, t.key) AS name,
		       COUNT(DISTINCT pt.product_id) AS product_count
		FROM tags t
		JOIN product_tags pt ON pt.tag_id = t.id
		JOIN product_translations tr ON tr.product_id = pt.product_id
		                       AND tr.status = 'published'
		LEFT JOIN tag_translations tt      ON tt.tag_id = t.id      AND tt.locale = $1
		LEFT JOIN tag_translations tt_en   ON tt_en.tag_id = t.id   AND tt_en.locale = 'en-US'
		GROUP BY t.id, t.key, tt.name, tt_en.name
		ORDER BY name ASC`, locale)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAllTagsInUse: %w", err)
	}
	defer rows.Close()
	out := []models.TagWithCount{}
	for rows.Next() {
		var twc models.TagWithCount
		if err := rows.Scan(&twc.ID, &twc.Key, &twc.Name, &twc.ProductCount); err != nil {
			return nil, fmt.Errorf("repository.FindAllTagsInUse.Scan: %w", err)
		}
		out = append(out, twc)
	}
	return out, rows.Err()
}

// --- Catalog helpers ---

func (r *Repository) FindAllCategories(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT category FROM products WHERE category IS NOT NULL AND category != '' ORDER BY category ASC`)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAllCategories: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("repository.FindAllCategories.Scan: %w", err)
		}
		out = append(out, cat)
	}
	return out, nil
}
