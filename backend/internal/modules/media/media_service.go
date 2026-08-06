package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"
)

// ServiceInterface is the contract the media service satisfies. The handler +
// other modules (product, order) depend on this interface.
type ServiceInterface interface {
	// RegisterAsset records an already-uploaded file. The browser uploads to
	// storage FIRST (presigned OSS PUT or the dev upload handler), then POSTs
	// the oss_key + metadata here.
	RegisterAsset(ctx context.Context, data models.RegisterAssetData, uploadedBy *string) (*models.MediaAsset, error)
	// UploadLocal is the dev-only server-side upload (STORAGE_MODE=local): the
	// handler receives a multipart file, the service stores it via Store.Put +
	// returns the oss_key + public_url so the caller can RegisterAsset next.
	UploadLocal(ctx context.Context, kind storage.Kind, mime string, r io.Reader, uploadedBy *string) (ossKey, publicURL string, err error)
	// GetAsset fetches a single asset + resolves its public URL.
	GetAsset(ctx context.Context, id int64) (*models.MediaAsset, error)
	// ListAssets lists assets (admin).
	ListAssets(ctx context.Context, kind string, page, limit int) ([]models.MediaAsset, int, error)
	// DeleteAsset deletes the asset row + the stored object (best-effort GC).
	DeleteAsset(ctx context.Context, id int64) error
	// AttachToProduct attaches a media asset to a product's gallery.
	AttachToProduct(ctx context.Context, productID, mediaID int64, sortOrder *int, caption string) error
	// ListProductMedia loads a product's ordered gallery (public URLs resolved).
	ListProductMedia(ctx context.Context, productID int64) ([]models.ProductMediaItem, error)
	// DetachFromProduct removes a media asset from a product's gallery.
	DetachFromProduct(ctx context.Context, productID, mediaID int64) error
	// ReorderProductMedia sets sort_order for a batch.
	ReorderProductMedia(ctx context.Context, productID int64, items []models.ReorderMediaItem) error

	// --- Artist gallery (PRD §3.1.3) ---
	AttachToArtist(ctx context.Context, artistID, mediaID int64, sortOrder *int, caption string) error
	ListArtistMedia(ctx context.Context, artistID int64) ([]models.GalleryItem, error)
	DetachFromArtist(ctx context.Context, artistID, mediaID int64) error
	ReorderArtistMedia(ctx context.Context, artistID int64, items []models.ReorderMediaItem) error

	// --- Ceramic story gallery (PRD §3.1.2) ---
	AttachToStory(ctx context.Context, storyID, mediaID int64, sortOrder *int, caption string) error
	ListStoryMedia(ctx context.Context, storyID int64) ([]models.GalleryItem, error)
	DetachFromStory(ctx context.Context, storyID, mediaID int64) error
	ReorderStoryMedia(ctx context.Context, storyID int64, items []models.ReorderMediaItem) error

	// --- Activity gallery (PRD §3.1.2/§3.1.3) ---
	AttachToActivity(ctx context.Context, activityID, mediaID int64, sortOrder *int, caption string) error
	ListActivityMedia(ctx context.Context, activityID int64) ([]models.GalleryItem, error)
	DetachFromActivity(ctx context.Context, activityID, mediaID int64) error
	ReorderActivityMedia(ctx context.Context, activityID int64, items []models.ReorderMediaItem) error
}

type Service struct {
	repo RepositoryInterface
	// store resolves oss_key → public URL + performs dev uploads / GC deletes.
	// Nil in tests that don't need URL resolution.
	store storage.Store
}

func NewService(repo RepositoryInterface, store storage.Store) *Service {
	return &Service{repo: repo, store: store}
}

func (s *Service) RegisterAsset(ctx context.Context, data models.RegisterAssetData, uploadedBy *string) (*models.MediaAsset, error) {
	if data.Kind != "image" && data.Kind != "video" {
		return nil, models.ErrInvalidOperation
	}
	a := &models.MediaAsset{
		Kind: data.Kind, OSSKey: data.OSSKey, MIME: data.MIME,
		Width: data.Width, Height: data.Height, Duration: data.Duration,
		UploadedBy: uploadedBy,
	}
	if err := s.repo.RegisterAsset(ctx, a); err != nil {
		return nil, fmt.Errorf("media.RegisterAsset: %w", err)
	}
	if s.store != nil {
		a.PublicURL = s.store.PublicURL(a.OSSKey)
	}
	return a, nil
}

func (s *Service) UploadLocal(ctx context.Context, kind storage.Kind, mime string, r io.Reader, uploadedBy *string) (string, string, error) {
	if s.store == nil {
		return "", "", errors.New("media.UploadLocal: no store configured")
	}
	key, err := s.store.Key(kind, mime)
	if err != nil {
		return "", "", fmt.Errorf("media.UploadLocal.Key: %w", err)
	}
	if err := s.store.Put(ctx, key, r, mime); err != nil {
		return "", "", fmt.Errorf("media.UploadLocal.Put: %w", err)
	}
	return key, s.store.PublicURL(key), nil
}

func (s *Service) GetAsset(ctx context.Context, id int64) (*models.MediaAsset, error) {
	a, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.store != nil {
		a.PublicURL = s.store.PublicURL(a.OSSKey)
	}
	return a, nil
}

func (s *Service) ListAssets(ctx context.Context, kind string, page, limit int) ([]models.MediaAsset, int, error) {
	assets, total, err := s.repo.ListAssets(ctx, kind, page, limit)
	if err != nil {
		return nil, 0, err
	}
	if s.store != nil {
		for i := range assets {
			assets[i].PublicURL = s.store.PublicURL(assets[i].OSSKey)
		}
	}
	return assets, total, nil
}

func (s *Service) DeleteAsset(ctx context.Context, id int64) error {
	a, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	// Delete the stored object (best-effort: a missing file is not fatal).
	if s.store != nil {
		if err := s.store.Delete(ctx, a.OSSKey); err != nil {
			log.Printf("media.DeleteAsset.Store(oss_key=%s): %v", a.OSSKey, err)
		}
	}
	return s.repo.DeleteAsset(ctx, id)
}

func (s *Service) AttachToProduct(ctx context.Context, productID, mediaID int64, sortOrder *int, caption string) error {
	var cap *string
	if caption != "" {
		c := caption
		cap = &c
	}
	if err := s.repo.AttachToProduct(ctx, productID, mediaID, sortOrder, cap); err != nil {
		return fmt.Errorf("media.AttachToProduct: %w", err)
	}
	return nil
}

func (s *Service) ListProductMedia(ctx context.Context, productID int64) ([]models.ProductMediaItem, error) {
	items, err := s.repo.ListProductMedia(ctx, productID)
	if err != nil {
		return nil, err
	}
	if s.store != nil {
		for i := range items {
			items[i].MediaAsset.PublicURL = s.store.PublicURL(items[i].MediaAsset.OSSKey)
		}
	}
	return items, nil
}

func (s *Service) DetachFromProduct(ctx context.Context, productID, mediaID int64) error {
	if err := s.repo.DetachFromProduct(ctx, productID, mediaID); err != nil {
		return fmt.Errorf("media.DetachFromProduct: %w", err)
	}
	return nil
}

func (s *Service) ReorderProductMedia(ctx context.Context, productID int64, items []models.ReorderMediaItem) error {
	if err := s.repo.ReorderProductMedia(ctx, productID, items); err != nil {
		return fmt.Errorf("media.ReorderProductMedia: %w", err)
	}
	return nil
}

// =============================================================================
// Entity galleries (artist / ceramic-story / activity) — mirror product_media.
// Each List*Media resolves the media asset PublicURLs via the storage adapter.
// =============================================================================

func resolveGalleryURLs(items []models.GalleryItem, store storage.Store) {
	if store == nil {
		return
	}
	for i := range items {
		items[i].MediaAsset.PublicURL = store.PublicURL(items[i].MediaAsset.OSSKey)
	}
}

func (s *Service) AttachToArtist(ctx context.Context, artistID, mediaID int64, sortOrder *int, caption string) error {
	var cap *string
	if caption != "" {
		c := caption
		cap = &c
	}
	if err := s.repo.AttachToArtist(ctx, artistID, mediaID, sortOrder, cap); err != nil {
		return fmt.Errorf("media.AttachToArtist: %w", err)
	}
	return nil
}

func (s *Service) ListArtistMedia(ctx context.Context, artistID int64) ([]models.GalleryItem, error) {
	items, err := s.repo.ListArtistMedia(ctx, artistID)
	if err != nil {
		return nil, err
	}
	resolveGalleryURLs(items, s.store)
	return items, nil
}

func (s *Service) DetachFromArtist(ctx context.Context, artistID, mediaID int64) error {
	if err := s.repo.DetachFromArtist(ctx, artistID, mediaID); err != nil {
		return fmt.Errorf("media.DetachFromArtist: %w", err)
	}
	return nil
}

func (s *Service) ReorderArtistMedia(ctx context.Context, artistID int64, items []models.ReorderMediaItem) error {
	if err := s.repo.ReorderArtistMedia(ctx, artistID, items); err != nil {
		return fmt.Errorf("media.ReorderArtistMedia: %w", err)
	}
	return nil
}

func (s *Service) AttachToStory(ctx context.Context, storyID, mediaID int64, sortOrder *int, caption string) error {
	var cap *string
	if caption != "" {
		c := caption
		cap = &c
	}
	if err := s.repo.AttachToStory(ctx, storyID, mediaID, sortOrder, cap); err != nil {
		return fmt.Errorf("media.AttachToStory: %w", err)
	}
	return nil
}

func (s *Service) ListStoryMedia(ctx context.Context, storyID int64) ([]models.GalleryItem, error) {
	items, err := s.repo.ListStoryMedia(ctx, storyID)
	if err != nil {
		return nil, err
	}
	resolveGalleryURLs(items, s.store)
	return items, nil
}

func (s *Service) DetachFromStory(ctx context.Context, storyID, mediaID int64) error {
	if err := s.repo.DetachFromStory(ctx, storyID, mediaID); err != nil {
		return fmt.Errorf("media.DetachFromStory: %w", err)
	}
	return nil
}

func (s *Service) ReorderStoryMedia(ctx context.Context, storyID int64, items []models.ReorderMediaItem) error {
	if err := s.repo.ReorderStoryMedia(ctx, storyID, items); err != nil {
		return fmt.Errorf("media.ReorderStoryMedia: %w", err)
	}
	return nil
}

func (s *Service) AttachToActivity(ctx context.Context, activityID, mediaID int64, sortOrder *int, caption string) error {
	var cap *string
	if caption != "" {
		c := caption
		cap = &c
	}
	if err := s.repo.AttachToActivity(ctx, activityID, mediaID, sortOrder, cap); err != nil {
		return fmt.Errorf("media.AttachToActivity: %w", err)
	}
	return nil
}

func (s *Service) ListActivityMedia(ctx context.Context, activityID int64) ([]models.GalleryItem, error) {
	items, err := s.repo.ListActivityMedia(ctx, activityID)
	if err != nil {
		return nil, err
	}
	resolveGalleryURLs(items, s.store)
	return items, nil
}

func (s *Service) DetachFromActivity(ctx context.Context, activityID, mediaID int64) error {
	if err := s.repo.DetachFromActivity(ctx, activityID, mediaID); err != nil {
		return fmt.Errorf("media.DetachFromActivity: %w", err)
	}
	return nil
}

func (s *Service) ReorderActivityMedia(ctx context.Context, activityID int64, items []models.ReorderMediaItem) error {
	if err := s.repo.ReorderActivityMedia(ctx, activityID, items); err != nil {
		return fmt.Errorf("media.ReorderActivityMedia: %w", err)
	}
	return nil
}
