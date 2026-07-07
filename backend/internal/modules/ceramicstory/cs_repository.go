package ceramicstory

import (
	"context"
	"database/sql"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines the methods for interacting with ceramic story storage.
type RepositoryInterface interface {
	FindAll(ctx context.Context) ([]models.CeramicStory, error)
	FindByIDOrSlug(ctx context.Context, idOrSlug string) (*models.CeramicStory, error)
	// Admin methods (example, not directly used by current public routes)
	// Create(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error)
	// Update(ctx context.Context, id int64, data models.UpdateCeramicStoryData) (*models.CeramicStory, error)
	// Delete(ctx context.Context, id int64) error
}

// Repository provides access to the ceramic story storage.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new ceramic story repository.
func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// FindAll retrieves all ceramic stories, ordered by display_order.
// For the timeline view, you might want to select fewer fields if it's just a summary.
func (r *Repository) FindAll(ctx context.Context) ([]models.CeramicStory, error) {
	stories := []models.CeramicStory{}
	query := `
		SELECT id, dynasty_name, slug, period, start_year, end_year, 
		       description, characteristics_craft, characteristics_art, 
		       image_url, takeaways, display_order
		FROM ceramic_stories
		ORDER BY display_order ASC, start_year DSC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAll.Query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var story models.CeramicStory
		err := rows.Scan(
			&story.ID, &story.DynastyName, &story.Slug, &story.Period, &story.StartYear, &story.EndYear,
			&story.Description, &story.CharacteristicsCraft, &story.CharacteristicsArt,
			&story.ImageURL, &story.Takeaways, &story.DisplayOrder,
		)
		if err != nil {
			return nil, fmt.Errorf("repository.FindAll.Scan: %w", err)
		}
		stories = append(stories, story)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository.FindAll.RowsErr: %w", err)
	}

	return stories, nil
}

// FindByIDOrSlug retrieves a single ceramic story by its ID or slug.
func (r *Repository) FindByIDOrSlug(ctx context.Context, idOrSlug string) (*models.CeramicStory, error) {
	var story models.CeramicStory
	query := `
		SELECT id, dynasty_name, slug, period, start_year, end_year, 
		       description, characteristics_craft, characteristics_art, 
		       image_url, takeaways, display_order
		FROM ceramic_stories
	`
	var err error
	// Try to parse idOrSlug as an integer (ID) first
	id, convErr := strconv.ParseInt(idOrSlug, 10, 64)
	if convErr == nil {
		// It's a numeric ID
		query += " WHERE id = $1"
		err = r.db.QueryRow(ctx, query, id).Scan(
			&story.ID, &story.DynastyName, &story.Slug, &story.Period, &story.StartYear, &story.EndYear,
			&story.Description, &story.CharacteristicsCraft, &story.CharacteristicsArt,
			&story.ImageURL, &story.Takeaways, &story.DisplayOrder,
		)
	} else {
		// Assume it's a slug (string)
		query += " WHERE slug = $1"
		err = r.db.QueryRow(ctx, query, idOrSlug).Scan(
			&story.ID, &story.DynastyName, &story.Slug, &story.Period, &story.StartYear, &story.EndYear,
			&story.Description, &story.CharacteristicsCraft, &story.CharacteristicsArt,
			&story.ImageURL, &story.Takeaways, &story.DisplayOrder,
		)
	}

	if err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows in result set") { // pgx might use different error
			return nil, models.ErrNotFound // Use a common error type from your models package
		}
		return nil, fmt.Errorf("repository.FindByIDOrSlug: %w", err)
	}
	return &story, nil
}

// --- Admin methods (Example Implementations - uncomment and complete if needed for admin panel) ---
/*
func (r *Repository) Create(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error) {
	story := models.CeramicStory{}
	query := `
		INSERT INTO ceramic_stories (
			dynasty_name, slug, period, start_year, end_year, description,
			characteristics_craft, characteristics_art, image_url, takeaways, display_order
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, dynasty_name, slug, period, start_year, end_year, description,
		          characteristics_craft, characteristics_art, image_url, takeaways, display_order
	`
	err := r.db.QueryRow(ctx, query,
		data.DynastyName, data.Slug, data.Period, data.StartYear, data.EndYear, data.Description,
		data.CharacteristicsCraft, data.CharacteristicsArt, data.ImageURL, data.Takeaways, data.DisplayOrder,
	).Scan(
		&story.ID, &story.DynastyName, &story.Slug, &story.Period, &story.StartYear, &story.EndYear,
		&story.Description, &story.CharacteristicsCraft, &story.CharacteristicsArt,
		&story.ImageURL, &story.Takeaways, &story.DisplayOrder,
	)
	if err != nil {
		// Handle potential duplicate slug error (unique constraint)
		return nil, fmt.Errorf("repository.Create: %w", err)
	}
	return &story, nil
}

func (r *Repository) Update(ctx context.Context, id int64, data models.UpdateCeramicStoryData) (*models.CeramicStory, error) {
	// Build query dynamically based on which fields in `data` are not nil
	// For brevity, this is a simplified example assuming all fields might be updated
	// A more robust implementation would use squirrel or build SET clauses carefully.
	story := models.CeramicStory{}
	query := `
		UPDATE ceramic_stories SET
			dynasty_name = COALESCE($2, dynasty_name),
			slug = COALESCE($3, slug),
			period = COALESCE($4, period),
			start_year = COALESCE($5, start_year),
			end_year = COALESCE($6, end_year),
			description = COALESCE($7, description),
			characteristics_craft = COALESCE($8, characteristics_craft),
			characteristics_art = COALESCE($9, characteristics_art),
			image_url = COALESCE($10, image_url),
			takeaways = COALESCE($11, takeaways),
			display_order = COALESCE($12, display_order)
		WHERE id = $1
		RETURNING id, dynasty_name, slug, period, start_year, end_year, description,
		          characteristics_craft, characteristics_art, image_url, takeaways, display_order
	`
	err := r.db.QueryRow(ctx, query,
		id, data.DynastyName, data.Slug, data.Period, data.StartYear, data.EndYear, data.Description,
		data.CharacteristicsCraft, data.CharacteristicsArt, data.ImageURL, data.Takeaways, data.DisplayOrder,
	).Scan(
		&story.ID, &story.DynastyName, &story.Slug, &story.Period, &story.StartYear, &story.EndYear,
		&story.Description, &story.CharacteristicsCraft, &story.CharacteristicsArt,
		&story.ImageURL, &story.Takeaways, &story.DisplayOrder,
	)
	if err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows") {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.Update: %w", err)
	}
	return &story, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM ceramic_stories WHERE id = $1"
	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("repository.Delete: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}
*/
