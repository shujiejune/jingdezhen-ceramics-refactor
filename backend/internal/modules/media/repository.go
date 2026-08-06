package media

import (
	"context"

	"jingdezhen-ceramics-backend/internal/models"
)

// RepositoryInterface is the contract the media repository satisfies. The
// service depends on this interface (not the concrete type) for testability.
type RepositoryInterface interface {
	// RegisterAsset inserts a media_assets row.
	RegisterAsset(ctx context.Context, a *models.MediaAsset) error
	// GetAsset fetches a single media_assets row by id.
	GetAsset(ctx context.Context, id int64) (*models.MediaAsset, error)
	// ListAssets lists media_assets (admin), newest first.
	ListAssets(ctx context.Context, kind string, page, limit int) ([]models.MediaAsset, int, error)
	// DeleteAsset deletes a media_assets row by id.
	DeleteAsset(ctx context.Context, id int64) error
	// AttachToProduct inserts a product_media row. sortOrder nil = append-last.
	AttachToProduct(ctx context.Context, productID, mediaID int64, sortOrder *int, caption *string) error
	// ListProductMedia loads a product's ordered gallery (media joined in).
	ListProductMedia(ctx context.Context, productID int64) ([]models.ProductMediaItem, error)
	// DetachFromProduct deletes a product_media row.
	DetachFromProduct(ctx context.Context, productID, mediaID int64) error
	// ReorderProductMedia sets sort_order for a batch of media on a product.
	ReorderProductMedia(ctx context.Context, productID int64, items []models.ReorderMediaItem) error

	// --- Artist gallery (mirrors product_media; PRD §3.1.3) ---
	AttachToArtist(ctx context.Context, artistID, mediaID int64, sortOrder *int, caption *string) error
	ListArtistMedia(ctx context.Context, artistID int64) ([]models.GalleryItem, error)
	DetachFromArtist(ctx context.Context, artistID, mediaID int64) error
	ReorderArtistMedia(ctx context.Context, artistID int64, items []models.ReorderMediaItem) error

	// --- Ceramic story gallery (History & Heritage, PRD §3.1.2) ---
	AttachToStory(ctx context.Context, storyID, mediaID int64, sortOrder *int, caption *string) error
	ListStoryMedia(ctx context.Context, storyID int64) ([]models.GalleryItem, error)
	DetachFromStory(ctx context.Context, storyID, mediaID int64) error
	ReorderStoryMedia(ctx context.Context, storyID int64, items []models.ReorderMediaItem) error

	// --- Activity gallery (Destinations & Local Lifestyle, PRD §3.1.2/§3.1.3) ---
	AttachToActivity(ctx context.Context, activityID, mediaID int64, sortOrder *int, caption *string) error
	ListActivityMedia(ctx context.Context, activityID int64) ([]models.GalleryItem, error)
	DetachFromActivity(ctx context.Context, activityID, mediaID int64) error
	ReorderActivityMedia(ctx context.Context, activityID int64, items []models.ReorderMediaItem) error
}
