package ceramicstory

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines ceramic-story storage operations (i18n-aware).
type RepositoryInterface interface {
	// FindAllPublished returns all stories that have a published translation for
	// the given locale, ordered by display_order (parent) then start_year.
	FindAllPublished(ctx context.Context, locale string) ([]models.CeramicStory, error)
	// FindPublishedBySlug returns the published translation for (slug, locale),
	// or ErrNotFound if no published translation exists in that locale.
	FindPublishedBySlug(ctx context.Context, locale, slug string) (*models.CeramicStory, error)
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
