package engage

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines engage (Destinations & Local Lifestyle) storage.
type RepositoryInterface interface {
	// FindAllPublished returns published translations for a locale, optionally
	// filtered by the parent `type` (e.g. "Destination" vs "Local Lifestyle"),
	// paginated. Returns the page of activities + the total count (of published
	// translations matching the filter) for pagination metadata.
	FindAllPublished(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error)
	// FindPublishedBySlug returns the published translation for (slug, locale).
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Activity, error)
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
