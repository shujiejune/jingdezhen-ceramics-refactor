package cart

import (
	"context"
	"errors"
	"fmt"
	"log"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines cart business logic.
type ServiceInterface interface {
	// GetCart returns the user's cart (items + totals) for the given locale.
	// Lazily creates the cart row.
	GetCart(ctx context.Context, userID string, locale string) (*models.Cart, error)
	// AddItem adds qty of an SKU to the cart (additive). Returns ErrNotFound if
	// the SKU doesn't exist, ErrConflict if the resulting qty exceeds stock.
	AddItem(ctx context.Context, userID string, skuID int64, qty int) error
	// SetItemQty sets the qty for an SKU in the cart to exactly qty (absolute).
	// Returns ErrNotFound if the SKU isn't in the cart, ErrConflict if qty >
	// stock.
	SetItemQty(ctx context.Context, userID string, skuID int64, qty int) error
	// RemoveItem deletes one SKU from the cart. Returns ErrNotFound if absent.
	RemoveItem(ctx context.Context, userID string, skuID int64) error
	// BulkRemove deletes several SKUs from the cart.
	BulkRemove(ctx context.Context, userID string, skuIDs []int64) (int, error)
	// Merge upserts guest-cart items into the user's server cart (additive,
	// capped at stock). Unknown SKUs are skipped (logged). Returns the merged
	// cart for the locale.
	Merge(ctx context.Context, userID string, items []models.MergeCartItem, locale string) (*models.Cart, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

func (s *Service) GetCart(ctx context.Context, userID string, locale string) (*models.Cart, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetCart.GetOrCreateCart: %w", err)
	}
	return s.buildCart(ctx, cartID, locale)
}

func (s *Service) AddItem(ctx context.Context, userID string, skuID int64, qty int) error {
	if qty < 1 {
		return models.ErrInvalidOperation
	}
	stock, exists, err := s.repo.SKUStock(ctx, skuID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}
	// Advisory guard: the resulting qty (current + added) must not exceed stock.
	// The authoritative atomic decrement happens at checkout (TDD §4.3).
	current, err := s.currentQty(ctx, userID, skuID)
	if err != nil {
		return err
	}
	if current+qty > stock {
		return models.ErrConflict
	}
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.AddItem(ctx, cartID, skuID, qty)
}

func (s *Service) SetItemQty(ctx context.Context, userID string, skuID int64, qty int) error {
	if qty < 1 {
		return models.ErrInvalidOperation
	}
	stock, exists, err := s.repo.SKUStock(ctx, skuID)
	if err != nil {
		return err
	}
	if !exists {
		return models.ErrNotFound
	}
	if qty > stock {
		return models.ErrConflict
	}
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.SetItemQty(ctx, cartID, skuID, qty); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) RemoveItem(ctx context.Context, userID string, skuID int64) error {
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.RemoveItem(ctx, cartID, skuID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return models.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) BulkRemove(ctx context.Context, userID string, skuIDs []int64) (int, error) {
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.BulkRemove(ctx, cartID, skuIDs)
}

func (s *Service) Merge(ctx context.Context, userID string, items []models.MergeCartItem, locale string) (*models.Cart, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.Merge.GetOrCreateCart: %w", err)
	}

	// Validate guest items: skip unknown SKUs, cap additive qty at stock so a
	// stale guest cart can't create an over-stock server cart.
	valid := make([]models.MergeCartItem, 0, len(items))
	for _, it := range items {
		if it.Qty < 1 {
			continue
		}
		stock, exists, err := s.repo.SKUStock(ctx, it.SkuID)
		if err != nil {
			return nil, err
		}
		if !exists {
			log.Printf("service.Merge: skipping unknown sku_id=%d", it.SkuID)
			continue
		}
		current, err := s.currentQtyByID(ctx, cartID, it.SkuID)
		if err != nil {
			return nil, err
		}
		// Cap the merged qty at stock (drop the excess rather than fail the whole
		// merge — the guest cart is best-effort).
		remaining := stock - current
		if remaining <= 0 {
			continue
		}
		addQty := it.Qty
		if addQty > remaining {
			addQty = remaining
		}
		valid = append(valid, models.MergeCartItem{SkuID: it.SkuID, Qty: addQty})
	}

	if len(valid) > 0 {
		if err := s.repo.MergeItems(ctx, cartID, valid); err != nil {
			return nil, err
		}
	}
	return s.buildCart(ctx, cartID, locale)
}

// currentQty returns the qty of an SKU in the user's cart (0 if absent or no
// cart yet). Used by the additive AddItem guard.
func (s *Service) currentQty(ctx context.Context, userID string, skuID int64) (int, error) {
	cartID, err := s.repo.GetOrCreateCart(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.GetItemQty(ctx, cartID, skuID)
}

// currentQtyByID reads the qty of an SKU in a specific cart (0 if absent).
func (s *Service) currentQtyByID(ctx context.Context, cartID, skuID int64) (int, error) {
	return s.repo.GetItemQty(ctx, cartID, skuID)
}

// buildCart assembles the Cart response (items + CNY totals). Presentment
// conversion is applied by the handler (which holds the FX converter), not
// here — the service has no FX dependency, keeping it unit-testable without
// mocking the FX service.
func (s *Service) buildCart(ctx context.Context, cartID int64, locale string) (*models.Cart, error) {
	items, err := s.repo.ListItems(ctx, cartID, locale)
	if err != nil {
		return nil, fmt.Errorf("service.buildCart.ListItems: %w", err)
	}
	cart := &models.Cart{Items: items, ItemCount: len(items)}
	for _, it := range items {
		cart.TotalCNY += it.LineTotalCNY
	}
	if cart.Items == nil {
		cart.Items = []models.CartItem{}
	}
	return cart, nil
}
