package engage

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines engage (Destinations & Local Lifestyle) storage.
type RepositoryInterface interface {
	// --- Public reads ---
	// FindAllPublished returns published translations for a locale, optionally
	// filtered by the parent `type` (e.g. "Destination" vs "Local Lifestyle"),
	// paginated. Returns the page of activities + the total count (of published
	// translations matching the filter) for pagination metadata.
	FindAllPublished(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error)
	// FindPublishedBySlug returns the published translation for (slug, locale).
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Activity, error)

	// --- Admin / CMS ---
	FindAllAdmin(ctx context.Context, locale, status, typeFilter string, page, limit int) ([]models.Activity, int, error)
	FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Activity, error)
	FindAdminByID(ctx context.Context, activityID int64, locale string) (*models.Activity, error)
	CreateWithTranslation(ctx context.Context, data models.CreateActivityData) (*models.Activity, error)
	UpdateTranslation(ctx context.Context, activityID int64, locale string, data models.UpdateActivityData) (*models.Activity, error)
	GetTranslationStatus(ctx context.Context, activityID int64, locale string) (models.ContentStatus, error)
	UpdateTranslationStatus(ctx context.Context, activityID int64, locale string, status models.ContentStatus, reviewerID *string) error
	Delete(ctx context.Context, activityID int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// Columns selected from the JOIN of parent + translation, in scan order.
const activityJoinColumns = `
    a.id, a.type, a.lat, a.lng, a.address, a.created_at, a.updated_at,
    t.slug, t.title, t.brief_introduction, t.body,
    t.meta_title, t.meta_description, t.locale, t.status, t.published_at
`
const activityJoinFrom = `
    FROM activities a
    JOIN activity_translations t ON t.activity_id = a.id
`

func (r *Repository) scanActivity(row pgx.Row) (*models.Activity, error) {
	var act models.Activity
	var lat, lng *float64
	var address, brief, body, metaTitle, metaDesc *string
	err := row.Scan(
		&act.ID, &act.Type, &lat, &lng, &address, &act.CreatedAt, &act.UpdatedAt,
		&act.Slug, &act.Title, &brief, &body,
		&metaTitle, &metaDesc, &act.Locale, &act.Status, &act.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	act.Lat = lat
	act.Lng = lng
	act.Address = address
	act.BriefIntroduction = brief
	act.Body = body
	act.MetaTitle = metaTitle
	act.MetaDescription = metaDesc
	return &act, nil
}

// FindAllPublished returns published activities for a locale, optionally
// filtered by type, paginated. Ordered by published_at DESC (newest first).
func (r *Repository) FindAllPublished(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error) {
	where := "WHERE t.locale = $1 AND t.status = 'published'"
	args := []any{locale}
	argIdx := 2
	if typeFilter != "" {
		where += fmt.Sprintf(" AND a.type = $%d", argIdx)
		args = append(args, typeFilter)
		argIdx++
	}

	// Count of matching published translations (for pagination metadata).
	countQuery := `SELECT COUNT(*) ` + activityJoinFrom + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Count: %w", err)
	}
	if total == 0 {
		return []models.Activity{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + activityJoinColumns + activityJoinFrom + where +
		fmt.Sprintf(" ORDER BY t.published_at DESC NULLS LAST LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Activity{}
	for rows.Next() {
		act, err := r.scanActivity(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllPublished.Scan: %w", err)
		}
		out = append(out, *act)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.RowsErr: %w", err)
	}
	return out, total, nil
}

// FindPublishedBySlug returns the published translation for (slug, locale).
func (r *Repository) FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Activity, error) {
	query := `SELECT ` + activityJoinColumns + activityJoinFrom +
		` WHERE t.locale = $1 AND t.slug = $2 AND t.status = 'published'`
	row := r.db.QueryRow(ctx, query, locale, slug)
	act, err := r.scanActivity(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindPublishedBySlug: %w", err)
	}
	return act, nil
}

// --- Admin / CMS ---------------------------------------------------------------

func (r *Repository) FindAllAdmin(ctx context.Context, locale, status, typeFilter string, page, limit int) ([]models.Activity, int, error) {
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
	if typeFilter != "" {
		where += fmt.Sprintf(" AND a.type = $%d", idx)
		args = append(args, typeFilter)
		idx++
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+activityJoinFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.Activity{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + activityJoinColumns + activityJoinFrom + where +
		fmt.Sprintf(" ORDER BY t.updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Activity{}
	for rows.Next() {
		act, err := r.scanActivity(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllAdmin.Scan: %w", err)
		}
		out = append(out, *act)
	}
	return out, total, nil
}

func (r *Repository) FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Activity, error) {
	query := `SELECT ` + activityJoinColumns + activityJoinFrom + ` WHERE t.locale = $1 AND t.slug = $2`
	act, err := r.scanActivity(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminBySlug: %w", err)
	}
	return act, nil
}

func (r *Repository) FindAdminByID(ctx context.Context, activityID int64, locale string) (*models.Activity, error) {
	query := `SELECT ` + activityJoinColumns + activityJoinFrom + ` WHERE a.id = $1 AND t.locale = $2`
	act, err := r.scanActivity(r.db.QueryRow(ctx, query, activityID, locale))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminByID: %w", err)
	}
	return act, nil
}

func (r *Repository) CreateWithTranslation(ctx context.Context, data models.CreateActivityData) (*models.Activity, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var activityID int64
	parentQuery := `
		INSERT INTO activities (type, lat, lng, address)
		VALUES ($1, $2, $3, $4) RETURNING id`
	if err := tx.QueryRow(ctx, parentQuery, data.Type, data.Lat, data.Lng, nullableStr(data.Address)).Scan(&activityID); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Parent: %w", err)
	}

	transQuery := `
		INSERT INTO activity_translations
		    (activity_id, locale, slug, title, brief_introduction, body, meta_title, meta_description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := tx.Exec(ctx, transQuery,
		activityID, data.Locale, data.Slug, data.Title,
		nullableStr(data.BriefIntroduction), nullableStr(data.Body),
		nullableStr(data.MetaTitle), nullableStr(data.MetaDescription)); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Translation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, activityID, data.Locale)
}

func (r *Repository) UpdateTranslation(ctx context.Context, activityID int64, locale string, data models.UpdateActivityData) (*models.Activity, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Parent non-localized fields.
	parentSets := []string{}
	parentArgs := []any{}
	pidx := 1
	if data.Type != nil {
		parentSets = append(parentSets, fmt.Sprintf("type = $%d", pidx))
		parentArgs = append(parentArgs, *data.Type)
		pidx++
	}
	if data.Lat != nil {
		parentSets = append(parentSets, fmt.Sprintf("lat = $%d", pidx))
		parentArgs = append(parentArgs, *data.Lat)
		pidx++
	}
	if data.Lng != nil {
		parentSets = append(parentSets, fmt.Sprintf("lng = $%d", pidx))
		parentArgs = append(parentArgs, *data.Lng)
		pidx++
	}
	if data.Address != nil {
		parentSets = append(parentSets, fmt.Sprintf("address = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.Address))
		pidx++
	}
	if len(parentSets) > 0 {
		parentSets = append(parentSets, "updated_at = NOW()")
		parentArgs = append(parentArgs, activityID)
		pq := fmt.Sprintf(`UPDATE activities SET %s WHERE id = $%d`, strings.Join(parentSets, ", "), pidx)
		if _, err := tx.Exec(ctx, pq, parentArgs...); err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Parent: %w", err)
		}
	}

	// Translation localized fields.
	transSets := []string{}
	transArgs := []any{}
	tidx := 1
	if data.Slug != nil {
		transSets = append(transSets, fmt.Sprintf("slug = $%d", tidx))
		transArgs = append(transArgs, *data.Slug)
		tidx++
	}
	if data.Title != nil {
		transSets = append(transSets, fmt.Sprintf("title = $%d", tidx))
		transArgs = append(transArgs, *data.Title)
		tidx++
	}
	if data.BriefIntroduction != nil {
		transSets = append(transSets, fmt.Sprintf("brief_introduction = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.BriefIntroduction))
		tidx++
	}
	if data.Body != nil {
		transSets = append(transSets, fmt.Sprintf("body = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.Body))
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
		transArgs = append(transArgs, activityID, locale)
		tq := fmt.Sprintf(`UPDATE activity_translations SET %s WHERE activity_id = $%d AND locale = $%d`,
			strings.Join(transSets, ", "), tidx, tidx+1)
		cmd, err := tx.Exec(ctx, tq, transArgs...)
		if err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Translation: %w", err)
		}
		if cmd.RowsAffected() == 0 {
			return nil, models.ErrNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, activityID, locale)
}

func (r *Repository) GetTranslationStatus(ctx context.Context, activityID int64, locale string) (models.ContentStatus, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM activity_translations WHERE activity_id = $1 AND locale = $2`,
		activityID, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", models.ErrNotFound
		}
		return "", fmt.Errorf("repository.GetTranslationStatus: %w", err)
	}
	return models.ContentStatus(status), nil
}

func (r *Repository) UpdateTranslationStatus(ctx context.Context, activityID int64, locale string, status models.ContentStatus, reviewerID *string) error {
	var publishedAt any
	if status == models.StatusPublished {
		publishedAt = time.Now()
	} else {
		publishedAt = nil
	}
	cmd, err := r.db.Exec(ctx,
		`UPDATE activity_translations
		    SET status = $3, reviewed_by = $4, published_at = $5, updated_at = NOW()
		    WHERE activity_id = $1 AND locale = $2`,
		activityID, locale, string(status), reviewerID, publishedAt)
	if err != nil {
		return fmt.Errorf("repository.UpdateTranslationStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, activityID int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM activities WHERE id = $1`, activityID)
	if err != nil {
		return fmt.Errorf("repository.Delete: %w", err)
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
