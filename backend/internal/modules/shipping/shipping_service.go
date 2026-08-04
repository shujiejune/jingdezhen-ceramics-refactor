package shipping

import (
	"context"
	"errors"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
	platformshipping "jingdezhen-ceramics-backend/internal/platform/shipping"
)

// ServiceInterface defines shipping business logic.
type ServiceInterface interface {
	// Quote returns a shipping-fee preview for a country + weight. Does not
	// raise on unshippable/overweight — returns a shippable=false quote so the
	// public preview endpoint can render a clear message (PRD §3.2.3).
	Quote(ctx context.Context, country string, weightGrams int) (*models.ShippingQuoteResponse, error)
	// TiersForCountry loads the fee tiers for a country (used by checkout's
	// shipping calc). Returns an empty slice if none configured.
	TiersForCountry(ctx context.Context, country string) ([]platformshipping.Tier, error)
	// ListAll returns all tiers (admin).
	ListAll(ctx context.Context) ([]models.ShippingFeeTier, error)
	Create(ctx context.Context, req models.CreateShippingTierRequest) (*models.ShippingFeeTier, error)
	Update(ctx context.Context, id int64, req models.UpdateShippingTierRequest) (*models.ShippingFeeTier, error)
	Delete(ctx context.Context, id int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

func (s *Service) Quote(ctx context.Context, country string, weightGrams int) (*models.ShippingQuoteResponse, error) {
	tiers, err := s.repo.ListByCountry(ctx, country)
	if err != nil {
		return nil, err
	}
	fee, err := platformshipping.CalcFee(tiers, weightGrams)
	resp := &models.ShippingQuoteResponse{Country: country, WeightGrams: weightGrams}
	switch {
	case err == nil:
		resp.FeeCNY = fee
		resp.Shippable = true
	case isUnshippable(err):
		resp.Shippable = false
		resp.Reason = "unshippable"
	case isOverweight(err):
		resp.Shippable = false
		resp.Reason = "overweight"
	default:
		return nil, fmt.Errorf("shipping.Service.Quote: %w", err)
	}
	return resp, nil
}

func (s *Service) TiersForCountry(ctx context.Context, country string) ([]platformshipping.Tier, error) {
	return s.repo.ListByCountry(ctx, country)
}

func (s *Service) ListAll(ctx context.Context) ([]models.ShippingFeeTier, error) {
	return s.repo.ListAll(ctx)
}

func (s *Service) Create(ctx context.Context, req models.CreateShippingTierRequest) (*models.ShippingFeeTier, error) {
	return s.repo.Create(ctx, req.Country, req.MaxWeightGrams, req.FeeCNY)
}

func (s *Service) Update(ctx context.Context, id int64, req models.UpdateShippingTierRequest) (*models.ShippingFeeTier, error) {
	t, err := s.repo.Update(ctx, id, req.Country, req.MaxWeightGrams, req.FeeCNY)
	if err != nil {
		if isNotFound(err) {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if isNotFound(err) {
			return models.ErrNotFound
		}
		return err
	}
	return nil
}

// --- error helpers (pgx wraps; models.Err* are sentinel values) ---------------

func isUnshippable(err error) bool { return errors.Is(err, models.ErrUnshippable) }

func isOverweight(err error) bool { return errors.Is(err, models.ErrOverweight) }

func isNotFound(err error) bool { return errors.Is(err, models.ErrNotFound) }
