package wishlist

import (
	"context"
	"errors"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines wishlist business logic.
type ServiceInterface interface {
	List(ctx context.Context, userID string, locale string, page, limit int) ([]models.WishlistItem, int, error)
	Add(ctx context.Context, userID string, skuID int64) error
	Remove(ctx context.Context, userID string, skuID int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, userID string, locale string, page, limit int) ([]models.WishlistItem, int, error) {
	// Normalize the locale for the product-title JOIN (fallback to default
	// if the user's locale has no translation; the published JOIN returns
	// nothing for an unsupported locale, which surfaces as an empty wishlist
	// — acceptable for MVP; a fallback to en-US could be added if needed).
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	items, total, err := s.repo.List(ctx, userID, locale, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.List: %w", err)
	}
	return items, total, nil
}

func (s *Service) Add(ctx context.Context, userID string, skuID int64) error {
	// Validate the SKU exists before inserting (so a bad sku_id returns a
	// clean ErrNotFound, not a FK violation).
	exists, err := s.repo.SKUExists(ctx, skuID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}
	if err := s.repo.Add(ctx, userID, skuID); err != nil {
		return err
	}
	return nil
}

func (s *Service) Remove(ctx context.Context, userID string, skuID int64) error {
	if err := s.repo.Remove(ctx, userID, skuID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		return err
	}
	return nil
}
