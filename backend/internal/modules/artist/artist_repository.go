package artist

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/sitemap"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines artist-profile storage operations (i18n-aware).
type RepositoryInterface interface {
	// --- Public reads ---
	FindAllPublished(ctx context.Context, locale string, page, limit int) ([]models.Artist, int, error)
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Artist, error)
	// FindPublishedAlternates returns locale→slug for every OTHER published
	// translation of this artist (excludes currentLocale). For hreflang.
	FindPublishedAlternates(ctx context.Context, artistID int64, currentLocale string) (map[string]string, error)

	// --- Admin / CMS ---
	FindAllAdmin(ctx context.Context, locale, status string, page, limit int) ([]models.Artist, int, error)
	FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Artist, error)
	FindAdminByID(ctx context.Context, artistID int64, locale string) (*models.Artist, error)
	CreateWithTranslation(ctx context.Context, data models.CreateArtistData) (*models.Artist, error)
	UpdateTranslation(ctx context.Context, artistID int64, locale string, data models.UpdateArtistData) (*models.Artist, error)
	GetTranslationStatus(ctx context.Context, artistID int64, locale string) (models.ContentStatus, error)
	UpdateTranslationStatus(ctx context.Context, artistID int64, locale string, status models.ContentStatus, reviewerID *string) error
	Delete(ctx context.Context, artistID int64) error
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// Columns selected from the JOIN of parent + translation, in scan order.
const artistJoinColumns = `
    a.id, a.avatar_url, a.user_id, a.display_order, a.created_at, a.updated_at,
    t.name, t.slug, t.bio, t.meta_title, t.meta_description,
    t.locale, t.status, t.published_at
`

const artistJoinFrom = `
    FROM artists a
    JOIN artist_translations t ON t.artist_id = a.id
`

func (r *Repository) scanArtist(row pgx.Row) (*models.Artist, error) {
	var ar models.Artist
	var avatarURL, userID, bio, metaTitle, metaDesc *string
	err := row.Scan(
		&ar.ID, &avatarURL, &userID, &ar.DisplayOrder, &ar.CreatedAt, &ar.UpdatedAt,
		&ar.Name, &ar.Slug, &bio, &metaTitle, &metaDesc,
		&ar.Locale, &ar.Status, &ar.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	ar.AvatarURL = avatarURL
	ar.UserID = userID
	ar.Bio = bio
	ar.MetaTitle = metaTitle
	ar.MetaDescription = metaDesc
	return &ar, nil
}

// --- Public reads ---

func (r *Repository) FindAllPublished(ctx context.Context, locale string, page, limit int) ([]models.Artist, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) ` + artistJoinFrom + ` WHERE t.locale = $1 AND t.status = 'published'`
	if err := r.db.QueryRow(ctx, countQuery, locale).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Count: %w", err)
	}
	if total == 0 {
		return []models.Artist{}, 0, nil
	}

	offset := (page - 1) * limit
	query := `SELECT ` + artistJoinColumns + artistJoinFrom + `
		WHERE t.locale = $1 AND t.status = 'published'
		ORDER BY a.display_order ASC, a.created_at ASC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, locale, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Artist{}
	for rows.Next() {
		ar, err := r.scanArtist(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllPublished.Scan: %w", err)
		}
		out = append(out, *ar)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllPublished.RowsErr: %w", err)
	}
	return out, total, nil
}

func (r *Repository) FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.Artist, error) {
	query := `SELECT ` + artistJoinColumns + artistJoinFrom + `
		WHERE t.locale = $1 AND t.slug = $2 AND t.status = 'published'`
	ar, err := r.scanArtist(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindPublishedBySlug: %w", err)
	}
	return ar, nil
}

// FindPublishedAlternates returns locale→slug for every OTHER published
// translation of this artist (excludes currentLocale). Delegates to the
// shared sitemap.FindAlternates (hreflang, PRD §4.4).
func (r *Repository) FindPublishedAlternates(ctx context.Context, artistID int64, currentLocale string) (map[string]string, error) {
	return sitemap.FindAlternates(ctx, r.db, "artist_translations", "artist_id", artistID, currentLocale)
}

// --- Admin / CMS ---

func (r *Repository) FindAllAdmin(ctx context.Context, locale, status string, page, limit int) ([]models.Artist, int, error) {
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
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) `+artistJoinFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Count: %w", err)
	}
	if total == 0 {
		return []models.Artist{}, 0, nil
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	query := `SELECT ` + artistJoinColumns + artistJoinFrom + where +
		fmt.Sprintf(" ORDER BY t.updated_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllAdmin.Query: %w", err)
	}
	defer rows.Close()

	out := []models.Artist{}
	for rows.Next() {
		ar, err := r.scanArtist(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllAdmin.Scan: %w", err)
		}
		out = append(out, *ar)
	}
	return out, total, nil
}

func (r *Repository) FindAdminBySlug(ctx context.Context, locale, slug string) (*models.Artist, error) {
	query := `SELECT ` + artistJoinColumns + artistJoinFrom + ` WHERE t.locale = $1 AND t.slug = $2`
	ar, err := r.scanArtist(r.db.QueryRow(ctx, query, locale, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminBySlug: %w", err)
	}
	return ar, nil
}

func (r *Repository) FindAdminByID(ctx context.Context, artistID int64, locale string) (*models.Artist, error) {
	query := `SELECT ` + artistJoinColumns + artistJoinFrom + ` WHERE a.id = $1 AND t.locale = $2`
	ar, err := r.scanArtist(r.db.QueryRow(ctx, query, artistID, locale))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindAdminByID: %w", err)
	}
	return ar, nil
}

func (r *Repository) CreateWithTranslation(ctx context.Context, data models.CreateArtistData) (*models.Artist, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var artistID int64
	parentQuery := `
		INSERT INTO artists (avatar_url, user_id, display_order)
		VALUES ($1, $2, $3) RETURNING id`
	if err := tx.QueryRow(ctx, parentQuery,
		nullableStr(data.AvatarURL), nullableStr(data.UserID), data.DisplayOrder).Scan(&artistID); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Parent: %w", err)
	}

	transQuery := `
		INSERT INTO artist_translations
		    (artist_id, locale, name, slug, bio, meta_title, meta_description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.Exec(ctx, transQuery,
		artistID, data.Locale, data.Name, data.Slug,
		nullableStr(data.Bio), nullableStr(data.MetaTitle), nullableStr(data.MetaDescription)); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Translation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("repository.CreateWithTranslation.Commit: %w", err)
	}
	return r.FindAdminByID(ctx, artistID, data.Locale)
}

func (r *Repository) UpdateTranslation(ctx context.Context, artistID int64, locale string, data models.UpdateArtistData) (*models.Artist, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository.UpdateTranslation.BeginTx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Parent non-localized fields.
	parentSets := []string{}
	parentArgs := []any{}
	pidx := 1
	if data.AvatarURL != nil {
		parentSets = append(parentSets, fmt.Sprintf("avatar_url = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.AvatarURL))
		pidx++
	}
	if data.UserID != nil {
		parentSets = append(parentSets, fmt.Sprintf("user_id = $%d", pidx))
		parentArgs = append(parentArgs, nullableStr(*data.UserID))
		pidx++
	}
	if data.DisplayOrder != nil {
		parentSets = append(parentSets, fmt.Sprintf("display_order = $%d", pidx))
		parentArgs = append(parentArgs, *data.DisplayOrder)
		pidx++
	}
	if len(parentSets) > 0 {
		parentSets = append(parentSets, "updated_at = NOW()")
		parentArgs = append(parentArgs, artistID)
		pq := fmt.Sprintf(`UPDATE artists SET %s WHERE id = $%d`, strings.Join(parentSets, ", "), pidx)
		if _, err := tx.Exec(ctx, pq, parentArgs...); err != nil {
			return nil, fmt.Errorf("repository.UpdateTranslation.Parent: %w", err)
		}
	}

	// Translation localized fields.
	transSets := []string{}
	transArgs := []any{}
	tidx := 1
	if data.Name != nil {
		transSets = append(transSets, fmt.Sprintf("name = $%d", tidx))
		transArgs = append(transArgs, *data.Name)
		tidx++
	}
	if data.Slug != nil {
		transSets = append(transSets, fmt.Sprintf("slug = $%d", tidx))
		transArgs = append(transArgs, *data.Slug)
		tidx++
	}
	if data.Bio != nil {
		transSets = append(transSets, fmt.Sprintf("bio = $%d", tidx))
		transArgs = append(transArgs, nullableStr(*data.Bio))
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
		transArgs = append(transArgs, artistID, locale)
		tq := fmt.Sprintf(`UPDATE artist_translations SET %s WHERE artist_id = $%d AND locale = $%d`,
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
	return r.FindAdminByID(ctx, artistID, locale)
}

func (r *Repository) GetTranslationStatus(ctx context.Context, artistID int64, locale string) (models.ContentStatus, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM artist_translations WHERE artist_id = $1 AND locale = $2`,
		artistID, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", models.ErrNotFound
		}
		return "", fmt.Errorf("repository.GetTranslationStatus: %w", err)
	}
	return models.ContentStatus(status), nil
}

func (r *Repository) UpdateTranslationStatus(ctx context.Context, artistID int64, locale string, status models.ContentStatus, reviewerID *string) error {
	var publishedAt any
	if status == models.StatusPublished {
		publishedAt = time.Now()
	} else {
		publishedAt = nil
	}
	cmd, err := r.db.Exec(ctx,
		`UPDATE artist_translations
		    SET status = $3, reviewed_by = $4, published_at = $5, updated_at = NOW()
		    WHERE artist_id = $1 AND locale = $2`,
		artistID, locale, string(status), reviewerID, publishedAt)
	if err != nil {
		return fmt.Errorf("repository.UpdateTranslationStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, artistID int64) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM artists WHERE id = $1`, artistID)
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
