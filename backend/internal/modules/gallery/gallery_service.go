package gallery

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"log"
)

type ServiceInterface interface {
	GetArtworks(ctx context.Context, userID string, filters models.ArtworkFilters) ([]models.Artwork, int, error)
	GetArtworkByID(ctx context.Context, userID string, artworkID int64) (*models.Artwork, error)
	GetGalleryCategories(ctx context.Context) ([]string, error)
	GetFavArtworks(ctx context.Context, userID string, page, limit int) ([]models.UserFavArtworkEntry, int, error)
	MarkAsFavorite(ctx context.Context, userID string, artworkID int64) error
	UnmarkAsFavorite(ctx context.Context, userID string, artworkID int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

func (s *Service) GetArtworks(ctx context.Context, userID string, filters models.ArtworkFilters) ([]models.Artwork, int, error) {
	artworks, total, err := s.repo.FindAllArtworks(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetArtworks: %w", err)
	}

	if len(artworks) > 0 && userID != "" {
		artworkIDs := make([]int64, len(artworks))
		for i, art := range artworks {
			artworkIDs[i] = art.ID
		}
		favoriteMap, err := s.repo.CheckFavorites(ctx, userID, artworkIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("service.GetArtworks.CheckFavorites: %w", err)
		}
		for i := range artworks {
			if favoriteMap[artworks[i].ID] {
				artworks[i].IsFavorite = true
			}
		}
	}

	return artworks, total, nil
}

func (s *Service) GetArtworkByID(ctx context.Context, userID string, artworkID int64) (*models.Artwork, error) {
	artwork, err := s.repo.FindArtworkByID(ctx, artworkID)
	if err != nil {
		return nil, fmt.Errorf("service.GetArtworkByID: %w", err)
	}

	// Fetch related data
	images, err := s.repo.GetArtworkImages(ctx, artworkID)
	if err != nil {
		log.Printf("failed to get images for artwork: %d", artworkID)
	}
	artwork.Images = images

	tags, err := s.repo.GetArtworkTags(ctx, artworkID)
	if err != nil {
		log.Printf("failed to get tags for artwork: %d", artworkID)
	}
	artwork.Tags = tags

	if userID != "" {
		favoriteMap, err := s.repo.CheckFavorites(ctx, userID, []int64{artworkID})
		if err != nil {
			log.Printf("failed to get favorites of user: %s", userID)
		}
		if favoriteMap[artworkID] {
			artwork.IsFavorite = true
		}
	}

	return artwork, nil
}

func (s *Service) GetGalleryCategories(ctx context.Context) ([]string, error) {
	return s.repo.FindAllCategories(ctx)
}

func (s *Service) GetFavArtworks(ctx context.Context, userID string, page, limit int) ([]models.UserFavArtworkEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	} // Default/max limit
	favArtworks, total, err := s.repo.GetFavArtworks(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetFavArtworks: %w", err)
	}
	return favArtworks, total, nil
}

func (s *Service) MarkAsFavorite(ctx context.Context, userID string, artworkID int64) error {
	// Business logic: check if artwork exists first?
	// For simplicity, we let the DB foreign key handle this.
	return s.repo.AddFavorite(ctx, userID, artworkID)
}

func (s *Service) UnmarkAsFavorite(ctx context.Context, userID string, artworkID int64) error {
	return s.repo.RemoveFavorite(ctx, userID, artworkID)
}


