package models

import "time"

// Artist represents an artist (can be a platform user or historical)
type Artist struct {
	ID        int64      `json:"id" db:"id"`
	Name      string     `json:"name" db:"name"`
	Bio       *string    `json:"bio,omitempty" db:"bio"`
	UserID    *string    `json:"user_id,omitempty" db:"user_id"` // Link to users.id (UUID string)
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"`
}

// ArtworkImage represents an image associated with an artwork
type ArtworkImage struct {
	ID           int    `json:"id" db:"id"`
	ArtworkID    int64  `json:"artwork_id" db:"artwork_id"`
	ImageURL     string `json:"image_url" db:"image_url"`
	IsPrimary    bool   `json:"is_primary" db:"is_primary"`
	Caption      string `json:"caption,omitempty" db:"caption"`
	DisplayOrder int    `json:"display_order" db:"display_order"`
}

// Artwork represents a piece of ceramic art in the gallery
type Artwork struct {
	ID                 int64          `json:"id" db:"id"` // Use int64 for BIGSERIAL
	Title              string         `json:"title" db:"title"`
	ArtistID           *int64         `json:"artist_id,omitempty" db:"artist_id"` // FK to artists.id
	ArtistName         *string        `json:"artist_name,omitempty" db:"-"`       // Populated by JOIN if ArtistID is present
	ArtistNameOverride *string        `json:"artist_name_override,omitempty" db:"artist_name_override"`
	ThumbnailURL       string         `json:"thumbnail_url" db:"thumbnail_url"`
	Description        *string        `json:"description,omitempty" db:"description"`
	Period             string         `json:"period" db:"period"`
	Dimensions         *string        `json:"dimensions,omitempty" db:"dimensions"`
	Category           string         `json:"category" db:"category"`       // Or CategoryID if you have an artwork_categories table
	IsFavorite         bool           `json:"is_favorite,omitempty" db:"-"` // For current user, populated in service
	FavoriteCount      int            `json:"favorite_count" db:"-"`        // Calculated
	Images             []ArtworkImage `json:"images,omitempty" db:"-"`      // Loaded separately or via JOIN aggregation
	Tags               []Tag          `json:"tags,omitempty" db:"-"`        // Loaded via junction table
	CreatedAt          time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt          *time.Time     `json:"updated_at" db:"updated_at"`
}

// CreateArtworkData is for creating new artworks
type CreateArtworkData struct {
	Title              string   `json:"title" validate:"required,max=255"`
	ArtistID           *int     `json:"artist_id,omitempty"`
	ArtistNameOverride string   `json:"artist_name_override,omitempty"`
	Description        string   `json:"description,omitempty"`
	Period             string   `json:"period" validate:"required"`
	Category           string   `json:"category" validate:"required"`
	ImageURLs          []string `json:"image_urls,omitempty" validate:"omitempty,dive,url"` // For initial upload
	Tags               []Tag    `json:"tags,omitempty"`
}

type UserFavArtworkEntry struct {
	Artwork     Artwork   `json:"artwork"`
	FavoritedAt time.Time `json:"favorited_at"`
}

// ArtworkFilters defines the available query parameters for filtering artworks.
type ArtworkFilters struct {
	Category string
	ArtistID int64
	Page     int
	Limit    int
}
