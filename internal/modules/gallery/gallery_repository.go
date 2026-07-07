package gallery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBExecutor defines an interface for executing SQL queries, implemented by both *pgxpool.Pool and pgx.Tx.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type RepositoryInterface interface {
	FindAllArtworks(ctx context.Context, filters models.ArtworkFilters) ([]models.Artwork, int, error)
	FindArtworkByID(ctx context.Context, artworkID int64) (*models.Artwork, error)
	GetArtworkImages(ctx context.Context, artworkID int64) ([]models.ArtworkImage, error)
	GetArtworkTags(ctx context.Context, artworkID int64) ([]models.Tag, error)
	FindAllArtists(ctx context.Context, page, limit int) ([]models.Artist, int, error)
	FindArtistByID(ctx context.Context, artistID int64) (*models.Artist, error)
	FindAllCategories(ctx context.Context) ([]string, error)
	GetFavArtworks(ctx context.Context, userID string, page, limit int) ([]models.UserFavArtworkEntry, int, error)
	CheckFavorites(ctx context.Context, userID string, artworkIDs []int64) (map[int64]bool, error)
	AddFavorite(ctx context.Context, userID string, artworkID int64) error
	RemoveFavorite(ctx context.Context, userID string, artworkID int64) error

	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) *Repository
}

type Repository struct {
	db       *pgxpool.Pool
	executor DBExecutor
}

func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db, executor: db}
}

type Scannable interface {
	Scan(dest ...any) error
}

func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: r.db, executor: tx}
}

func (r *Repository) scanPartialArtwork(row Scannable) (*models.Artwork, error) {
	var art models.Artwork
	var artistID sql.NullInt64
	var artistName sql.NullString

	// This scan must match the SELECT in FindAllArtworks
	err := row.Scan(
		&art.ID,
		&art.Title,
		&art.Category,
		&art.ThumbnailURL,
		&art.CreatedAt,
		&artistID,
		&artistName,
	)
	if err != nil {
		return nil, err
	}

	if artistID.Valid {
		id := artistID.Int64
		art.ArtistID = &id
	}
	if artistName.Valid {
		art.ArtistName = &artistName.String
	}

	return &art, nil
}

func (r *Repository) scanFullArtwork(row Scannable) (*models.Artwork, error) {
	var art models.Artwork
	var artistID sql.NullInt64
	var artistName sql.NullString
	var artistNameOverride sql.NullString
	var description sql.NullString
	var dimensions sql.NullString
	var updatedAt sql.NullTime

	// The order of these &variables must match the order of columns in the SELECT statement.
	err := row.Scan(
		&art.ID,
		&art.Title,
		&art.ThumbnailURL,
		&art.Category,
		&art.CreatedAt,
		&updatedAt,
		&artistID,
		&artistName,
		&artistNameOverride,
		&description,
		&dimensions,
	)
	if err != nil {
		return nil, err
	}

	if updatedAt.Valid {
		art.UpdatedAt = &updatedAt.Time
	}
	if artistID.Valid {
		id := artistID.Int64
		art.ArtistID = &id
	}
	if artistName.Valid {
		art.ArtistName = &artistName.String
	}
	if artistNameOverride.Valid {
		art.ArtistNameOverride = &artistNameOverride.String
	}
	if description.Valid {
		art.Description = &description.String
	}
	if dimensions.Valid {
		art.Dimensions = &dimensions.String
	}

	return &art, nil
}

func (r *Repository) FindAllArtworks(ctx context.Context, filters models.ArtworkFilters) ([]models.Artwork, int, error) {
	// Use squirrel or another query builder for more complex dynamic queries.
	// This is a manual string building example.
	baseQuery := `
		FROM artworks a
		LEFT JOIN artists ar ON a.artist_id = ar.id
	`
	var whereClauses []string
	var args []any
	argIdx := 1

	if filters.Category != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("a.category = $%d", argIdx))
		args = append(args, filters.Category)
		argIdx++
	}
	if filters.ArtistID > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("a.artist_id = $%d", argIdx))
		args = append(args, filters.ArtistID)
		argIdx++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count with filters
	var total int
	countQuery := "SELECT COUNT(a.id) " + baseQuery + whereClause
	err := r.executor.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllArtworks.Count: %w", err)
	}

	if total == 0 {
		return []models.Artwork{}, 0, nil
	}

	// Get paginated data
	selectQuery := `
		SELECT a.id, a.title, a.category, a.thumbnail_url, a.created_at, a.artist_id, ar.name as artist_name
	`
	limitOffsetClause := fmt.Sprintf(" ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filters.Limit, (filters.Page-1)*filters.Limit)

	fullQuery := selectQuery + baseQuery + whereClause + limitOffsetClause
	rows, err := r.executor.Query(ctx, fullQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllArtworks.Query: %w", err)
	}
	defer rows.Close()

	artworks := []models.Artwork{}
	for rows.Next() {
		art, err := r.scanPartialArtwork(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllArtworks.Scan: %w", err)
		}
		artworks = append(artworks, *art)
	}

	return artworks, total, nil
}

func (r *Repository) FindArtworkByID(ctx context.Context, artworkID int64) (*models.Artwork, error) {
	art := &models.Artwork{}
	query := `
		SELECT
			a.id, a.title, a.thumbnail_url, a.category, a.created_at, a.updated_at,
			a.artist_id, ar.name as artist_name, a.artist_name_override,
			a.description, a.creation_year, a.dimensions, a.introduction
		FROM artworks a
		LEFT JOIN artists ar ON a.artist_id = ar.id
		WHERE a.id = $1
	`
	row := r.executor.QueryRow(ctx, query, artworkID)
	art, err := r.scanFullArtwork(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindArtworkByID: %w", err)
	}
	return art, nil
}

func (r *Repository) GetArtworkImages(ctx context.Context, artworkID int64) ([]models.ArtworkImage, error) {
	images := []models.ArtworkImage{}
	query := `SELECT id, image_url, caption FROM artwork_images WHERE artwork_id = $1 ORDER BY display_order ASC`
	rows, err := r.executor.Query(ctx, query, artworkID)
	if err != nil {
		return nil, fmt.Errorf("repository.GetArtworkImages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var img models.ArtworkImage
		if err := rows.Scan(&img.ID, &img.ImageURL, &img.Caption); err != nil {
			return nil, fmt.Errorf("repository.GetArtworkImages.Scan: %w", err)
		}
		images = append(images, img)
	}
	return images, nil
}

func (r *Repository) GetArtworkTags(ctx context.Context, artworkID int64) ([]models.Tag, error) {
	query := `
		SELECT t.id, t.name FROM tags t
		JOIN artwork_tags at ON t.id = at.tag_id
		WHERE at.artwork_id = $1
		ORDER BY t.name
	`
	rows, err := r.executor.Query(ctx, query, artworkID)
	if err != nil {
		return nil, fmt.Errorf("repository.GetArtworkTags.Query: %w", err)
	}

	tags, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Tag])
	if err != nil {
		return nil, fmt.Errorf("repository.GetArtworkTags.Scan: %w", err)
	}

	return tags, nil
}

func (r *Repository) scanArtist(row Scannable) (*models.Artist, error) {
	var artist models.Artist

	// Handle nullable fields for the Artist model
	var bio sql.NullString
	var userID sql.NullString

	err := row.Scan(
		&artist.ID,
		&artist.Name,
		&bio,
		&userID,
		&artist.CreatedAt,
		&artist.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if bio.Valid {
		artist.Bio = &bio.String
	}
	if userID.Valid {
		artist.UserID = &userID.String
	}

	return &artist, nil
}

// FindAllArtists retrieves a paginated list of all artists.
func (r *Repository) FindAllArtists(ctx context.Context, page, limit int) ([]models.Artist, int, error) {
	// First, get the total count of artists for pagination metadata.
	var total int
	countQuery := `SELECT COUNT(*) FROM artists`
	err := r.executor.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllArtists.Count: %w", err)
	}

	if total == 0 {
		return []models.Artist{}, 0, nil
	}

	// Then, fetch the paginated list of artists.
	offset := (page - 1) * limit
	query := `
		SELECT id, name, bio, user_id, created_at, updated_at
		FROM artists
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.executor.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllArtists.Query: %w", err)
	}
	defer rows.Close()

	artists := []models.Artist{}
	for rows.Next() {
		artist, err := r.scanArtist(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.FindAllArtists.Scan: %w", err)
		}
		artists = append(artists, *artist)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.FindAllArtists.RowsErr: %w", err)
	}

	return artists, total, nil
}

// FindArtistByID retrieves a single artist by their ID.
func (r *Repository) FindArtistByID(ctx context.Context, artistID int64) (*models.Artist, error) {
	// The SELECT statement must match the fields expected by the scanArtist helper.
	query := `
		SELECT id, name, bio, user_id, created_at, updated_at
		FROM artists
		WHERE id = $1
	`
	row := r.executor.QueryRow(ctx, query, artistID)
	artist, err := r.scanArtist(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("repository.FindArtistByID: %w", err)
	}
	return artist, nil
}

func (r *Repository) FindAllCategories(ctx context.Context) ([]string, error) {
	categories := []string{}
	query := `SELECT DISTINCT category FROM artworks WHERE category IS NOT NULL AND category != '' ORDER BY category ASC`
	rows, err := r.executor.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("repository.FindAllCategories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("repository.FindAllCategories.Scan: %w", err)
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

func (r *Repository) GetFavArtworks(ctx context.Context, userID string, page, limit int) ([]models.UserFavArtworkEntry, int, error) {
	offset := (page - 1) * limit

	// 1. Get artwork_ids and total count of favorites
	var artworkIDs []int64
	var favoritedAtTimes []time.Time

	queryFavs := `SELECT ufa.artwork_id, ufa.created_at
	          FROM user_favorite_artworks ufa WHERE ufa.user_id = $1 ORDER BY ufa.created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, queryFavs, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.GetFavArtworks.QueryFavs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var artworkID int64
		var favTime time.Time

		if err := rows.Scan(&artworkID, &favTime); err != nil {
			return nil, 0, fmt.Errorf("repository.GetFavArtworks.ScanFavIDs: %w", err)
		}
		artworkIDs = append(artworkIDs, artworkID)
		favoritedAtTimes = append(favoritedAtTimes, favTime)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.GetFavArtworks.RowsErr: %w", err)
	}

	var total int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM user_favorite_artworks WHERE user_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.GetFavArtworks.Count: %w", err)
	}

	// 2. Fetch artwork details for the retrieved IDs
	artworksQuery := `
        SELECT
            a.id, a.title, a.thumbnail_url,
            ar.name as artist_name
        FROM artworks a
        LEFT JOIN artists ar ON a.artist_id = ar.id
        WHERE a.id = ANY($1::bigint[])` // Use ANY for array of IDs

	artworkRows, err := r.db.Query(ctx, artworksQuery, artworkIDs)
	if err != nil {
		return nil, total, fmt.Errorf("repository.GetFavArtworks.QueryArtworks: %w", err)
	}
	defer artworkRows.Close()

	favArtworksMap := make(map[int64]models.UserFavArtworkEntry)
	for artworkRows.Next() {
		var art models.UserFavArtworkEntry
		var artistName sql.NullString // Handle potentially NULL artist name
		if err := artworkRows.Scan(&art.Artwork.ID, &art.Artwork.Title, &art.Artwork.ThumbnailURL, &artistName); err != nil {
			return nil, total, fmt.Errorf("repository.GetFavArtworks.ScanArtworks: %w", err)
		}
		if artistName.Valid {
			art.Artwork.ArtistName = &artistName.String
		}
		// You might want to add the 'favorited_at' time to the Artwork struct for display
		// For now, just populating the map.
		favArtworksMap[art.Artwork.ID] = art
	}
	if err := artworkRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repository.GetFavArtworks.ArtworkRowsErr: %w", err)
	}

	// Order results according to artworkIDs (which were ordered by favorite time)
	orderedFavArtworks := make([]models.UserFavArtworkEntry, 0, len(artworkIDs))
	for i, id := range artworkIDs {
		if art, ok := favArtworksMap[id]; ok {
			art.FavoritedAt = favoritedAtTimes[i]
			orderedFavArtworks = append(orderedFavArtworks, art)
		}
	}

	return orderedFavArtworks, total, nil
}

func (r *Repository) CheckFavorites(ctx context.Context, userID string, artworkIDs []int64) (map[int64]bool, error) {
	favoriteMap := make(map[int64]bool)
	if userID == "" || len(artworkIDs) == 0 {
		return favoriteMap, nil
	}
	query := `SELECT artwork_id FROM user_favorite_artworks WHERE user_id = $1 AND artwork_id = ANY($2)`
	rows, err := r.executor.Query(ctx, query, userID, artworkIDs)
	if err != nil {
		return nil, fmt.Errorf("repository.CheckFavorites: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var artworkID int64
		if err := rows.Scan(&artworkID); err != nil {
			return nil, fmt.Errorf("repository.CheckFavorites.Scan: %w", err)
		}
		favoriteMap[artworkID] = true
	}
	return favoriteMap, nil
}

func (r *Repository) AddFavorite(ctx context.Context, userID string, artworkID int64) error {
	query := `INSERT INTO user_favorite_artworks (user_id, artwork_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.executor.Exec(ctx, query, userID, artworkID)
	if err != nil {
		return fmt.Errorf("repository.AddFavorite: %w", err)
	}
	return nil
}

func (r *Repository) RemoveFavorite(ctx context.Context, userID string, artworkID int64) error {
	query := `DELETE FROM user_favorite_artworks WHERE user_id = $1 AND artwork_id = $2`
	cmdTag, err := r.executor.Exec(ctx, query, userID, artworkID)
	if err != nil {
		return fmt.Errorf("repository.RemoveFavorite: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		// This isn't necessarily an error, could just mean it wasn't a favorite to begin with.
		// Returning ErrNotFound could be misleading. Returning nil is often fine.
		log.Printf("failed to remove a fravorite artwork")
	}
	return nil
}
