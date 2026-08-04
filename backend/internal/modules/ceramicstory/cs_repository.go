package ceramicstory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines ceramic-story storage operations (i18n-aware).
type RepositoryInterface interface {
	// --- Public reads ---
	// FindAllPublished returns all stories that have a published translation for
	// the given locale, ordered by display_order (parent) then start_year.
	FindAllPublished(ctx context.Context, locale string) ([]models.CeramicStory, error)
	// FindPublishedBySlug returns the published translation for (slug, locale),
	// or ErrNotFound if no published translation exists in that locale.
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.CeramicStory, error)

	// --- Admin / CMS ---
	// FindAllAdmin returns translations of ALL statuses, optionally filtered by
	// locale and/or status, paginated. Ordered by updated_at DESC.
	FindAllAdmin(ctx context.Context, locale, status string, page, limit int) ([]models.CeramicStory, int, error)
	// FindAdminBySlug returns a translation by (slug, locale) regardless of
	// status (editor preview of drafts).
	FindAdminBySlug(ctx context.Context, locale, slug string) (*models.CeramicStory, error)
	// FindAdminByID returns a translation by (story_id, locale).
	FindAdminByID(ctx context.Context, storyID int64, locale string) (*models.CeramicStory, error)
	// CreateWithTranslation inserts the parent + first translation in one tx.
	CreateWithTranslation(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error)
	// UpdateTranslation updates the translation + parent non-localized fields.
	UpdateTranslation(ctx context.Context, storyID int64, locale string, data models.UpdateCeramicStoryData) (*models.CeramicStory, error)
	// GetTranslationStatus returns the current workflow status of (story_id, locale).
	GetTranslationStatus(ctx context.Context, storyID int64, locale string) (models.ContentStatus, error)
	// UpdateTranslationStatus sets the workflow status (+ reviewer + published_at).
	UpdateTranslationStatus(ctx context.Context, storyID int64, locale string, status models.ContentStatus, reviewerID *string) error
	// Delete removes the parent (cascades translations).
	Delete(ctx context.Context, storyID int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// Columns selected from the JOIN of parent + translation, in scan order.
// The parent contributes: id, start_year, end_year, image_url, display_order,
// created_at, updated_at. The translation contributes: dynasty_name, slug,
// period, description, characteristics_craft, characteristics_art, takeaways,
// meta_title, meta_description, locale, status, published_at.
const storyJoinColumns = `
    cs.id, cs.start_year, cs.end_year, cs.image_url, cs.display_order,
    cs.created_at, cs.updated_at,
    t.dynasty_name, t.slug, t.period, t.description,
    t.characteristics_craft, t.characteristics_art, t.takeaways,
    t.meta_title, t.meta_description, t.locale, t.status, t.published_at
`

const storyJoinFrom = `
    FROM ceramic_stories cs
    JOIN ceramic_story_translations t ON t.story_id = cs.id
`

func (r *Repository) scanStory(row pgx.Row) (*models.CeramicStory, error) {
	var s models.CeramicStory
	var period, craft, art, takeaways, metaTitle, metaDesc, imageURL *string
	// pgx scans NULL into nil pointer automatically for *string targets.
	err := row.Scan(
		&s.ID, &s.StartYear, &s.EndYear, &imageURL, &s.DisplayOrder,
		&s.CreatedAt, &s.UpdatedAt,
		&s.DynastyName, &s.Slug, &period, &s.Description,
		&craft, &art, &takeaways, &metaTitle, &metaDesc,
		&s.Locale, &s.Status, &s.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	s.Period = period
	s.CharacteristicsCraft = craft
	s.CharacteristicsArt = art
	s.Takeaways = takeaways
	s.MetaTitle = metaTitle
	s.MetaDescription = metaDesc
	s.ImageURL = imageURL
	return &s, nil
}

// FindAllPublished returns all published translations for a locale.
func (r *Repository) FindAllPublished(ctx context.Context, locale string) ([]models.CeramicStory, error) {
	query := `
		SELECT ` + storyJoinColumns + storyJoinFrom + `
		WHERE t.locale = $1 AND t.status = 'published'
		ORDER BY cs.display_order ASC, cs.start_year ASC NULLS LAST
	`
	rows, err := r.db.Query(ctx, query, locale)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAllPublished: %w", err)
	}
	defer rows.Close()

	out := []models.CeramicStory{}
	for rows.Next() {
		s, err := r.scanStory(rows)
		if err != nil {
			return nil, fmt.Errorf("repository.FindAllPublished.Scan: %w", err)
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.FindAllPublished.RowsErr: %w", err)
	}
	return out, nil
}

// FindPublishedBySlug returns the published translation for (slug, locale).
// Slugs are unique per locale, so a hit is authoritative.
func (r *Repository) FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.CeramicStory, error) {
	query := `
		SELECT ` + storyJoinColumns + storyJoinFrom + `
		WHERE t.locale = $1 AND t.slug = $2 AND t.status = 'published'
	`
	row := r.db.QueryRow(ctx, query, locale, slug)
	s, err := r.scanStory(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindPublishedBySlug: %w", err)
	}
	return s, nil
}

// --- Admin / CMS ---------------------------------------------------------------

func (r *Repository) FindAllAdmin(ctx context.Context, locale, status string, page, limit int) ([]models.CeramicStory, int, error) {
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

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+storyJoinFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.CeramicStory{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + storyJoinColumns + storyJoinFrom + where +
		fmt.Sprintf(" ORDER BY t.updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Query: %w", err)
	}
	defer rows.Close()

	out := []models.CeramicStory{}
	for rows.Next() {
		s, err := r.scanStory(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllAdmin.Scan: %w", err)
		}
		out = append(out, *s)
	}
	return out, total, nil
}

func (r *Repository) FindAdminBySlug(ctx context.Context, locale, slug string) (*models.CeramicStory, error) {
	query := `SELECT ` + storyJoinColumns + storyJoinFrom + ` WHERE t.locale = $1 AND t.slug = $2`
	s, err := r.scanStory(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminBySlug: %w", err)
	}
	return s, nil
}

func (r *Repository) FindAdminByID(ctx context.Context, storyID int64, locale string) (*models.CeramicStory, error) {
	query := `SELECT ` + storyJoinColumns + storyJoinFrom + ` WHERE cs.id = $1 AND t.locale = $2`
	s, err := r.scanStory(r.db.QueryRow(ctx, query, storyID, locale))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminByID: %w", err)
	}
	return s, nil
}

func (r *Repository) CreateWithTranslation(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var storyID int64
	parentQuery := `
		INSERT INTO ceramic_stories (start_year, end_year, image_url, display_order)
		VALUES ($1, $2, $3, $4) RETURNING id`
	imgURL := nullableStr(data.ImageURL)
	if err := tx.QueryRow(ctx, parentQuery, data.StartYear, data.EndYear, imgURL, data.DisplayOrder).Scan(&storyID); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Parent: %w", err)
	}

	transQuery := `
		INSERT INTO ceramic_story_translations
		    (story_id, locale, dynasty_name, slug, period, description,
		     characteristics_craft, characteristics_art, takeaways)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := tx.Exec(ctx, transQuery,
		storyID, data.Locale, data.DynastyName, data.Slug, nullableStr(data.Period),
		data.Description, nullableStr(data.CharacteristicsCraft),
		nullableStr(data.CharacteristicsArt), nullableStr(data.Takeaways)); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Translation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, storyID, data.Locale)
}

func (r *Repository) UpdateTranslation(ctx context.Context, storyID int64, locale string, data models.UpdateCeramicStoryData) (*models.CeramicStory, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Parent non-localized fields.
	parentSets := []string{}
	parentArgs := []any{}
	pidx := 1
	if data.StartYear != nil {
		parentSets = append(parentSets, fmt.Sprintf("start_year = $%d", pidx))
		parentArgs = append(parentArgs, *data.StartYear)
		pidx++
	}
	if data.EndYear != nil {
		parentSets = append(parentSets, fmt.Sprintf("end_year = $%d", pidx))
		parentArgs = append(parentArgs, *data.EndYear)
		pidx++
	}
	if data.ImageURL != nil {
		parentSets = append(parentSets, fmt.Sprintf("image_url = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.ImageURL))
		pidx++
	}
	if data.DisplayOrder != nil {
		parentSets = append(parentSets, fmt.Sprintf("display_order = $%d", pidx))
		parentArgs = append(parentArgs, *data.DisplayOrder)
		pidx++
	}
	if len(parentSets) > 0 {
		parentSets = append(parentSets, "updated_at = NOW()")
		parentArgs = append(parentArgs, storyID)
		pq := fmt.Sprintf(`UPDATE ceramic_stories SET %s WHERE id = $%d`, strings.Join(parentSets, ", "), pidx)
		if _, err := tx.Exec(ctx, pq, parentArgs...); err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Parent: %w", err)
		}
	}

	// Translation localized fields.
	transSets := []string{}
	transArgs := []any{}
	tidx := 1
	if data.DynastyName != nil {
		transSets = append(transSets, fmt.Sprintf("dynasty_name = $%d", tidx))
		transArgs = append(transArgs, *data.DynastyName)
		tidx++
	}
	if data.Slug != nil {
		transSets = append(transSets, fmt.Sprintf("slug = $%d", tidx))
		transArgs = append(transArgs, *data.Slug)
		tidx++
	}
	if data.Period != nil {
		transSets = append(transSets, fmt.Sprintf("period = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.Period))
		tidx++
	}
	if data.Description != nil {
		transSets = append(transSets, fmt.Sprintf("description = $%d", tidx))
		transArgs = append(transArgs, *data.Description)
		tidx++
	}
	if data.CharacteristicsCraft != nil {
		transSets = append(transSets, fmt.Sprintf("characteristics_craft = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.CharacteristicsCraft))
		tidx++
	}
	if data.CharacteristicsArt != nil {
		transSets = append(transSets, fmt.Sprintf("characteristics_art = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.CharacteristicsArt))
		tidx++
	}
	if data.Takeaways != nil {
		transSets = append(transSets, fmt.Sprintf("takeaways = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.Takeaways))
		tidx++
	}
	if len(transSets) > 0 {
		transSets = append(transSets, "updated_at = NOW()")
		transArgs = append(transArgs, storyID, locale)
		tq := fmt.Sprintf(`UPDATE ceramic_story_translations SET %s WHERE story_id = $%d AND locale = $%d`,
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
	return r.FindAdminByID(ctx, storyID, locale)
}

func (r *Repository) GetTranslationStatus(ctx context.Context, storyID int64, locale string) (models.ContentStatus, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM ceramic_story_translations WHERE story_id = $1 AND locale = $2`,
		storyID, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", models.ErrNotFound
		}
		return "", fmt.Errorf("repository.GetTranslationStatus: %w", err)
	}
	return models.ContentStatus(status), nil
}

func (r *Repository) UpdateTranslationStatus(ctx context.Context, storyID int64, locale string, status models.ContentStatus, reviewerID *string) error {
	var publishedAt any
	if status == models.StatusPublished {
		publishedAt = time.Now()
	} else {
		publishedAt = nil
	}
	cmd, err := r.db.Exec(ctx,
		`UPDATE ceramic_story_translations
		    SET status = $3, reviewed_by = $4, published_at = $5, updated_at = NOW()
		    WHERE story_id = $1 AND locale = $2`,
		storyID, locale, string(status), reviewerID, publishedAt)
	if err != nil {
		return fmt.Errorf("repository.UpdateTranslationStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, storyID int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM ceramic_stories WHERE id = $1`, storyID)
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
