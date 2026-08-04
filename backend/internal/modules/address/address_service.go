package address

import (
	"context"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
)

// ServiceInterface defines user-address business logic.
type ServiceInterface interface {
	ListAddresses(ctx context.Context, userID string) ([]models.UserAddress, error)
	GetAddress(ctx context.Context, userID string, id int64) (*models.UserAddress, error)
	CreateAddress(ctx context.Context, userID string, req models.CreateAddressRequest) (*models.UserAddress, error)
	UpdateAddress(ctx context.Context, userID string, id int64, req models.UpdateAddressRequest) (*models.UserAddress, error)
	DeleteAddress(ctx context.Context, userID string, id int64) error
	SetDefaultAddress(ctx context.Context, userID string, id int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

func (s *Service) ListAddresses(ctx context.Context, userID string) ([]models.UserAddress, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) GetAddress(ctx context.Context, userID string, id int64) (*models.UserAddress, error) {
	a, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, fmt.Errorf("service.GetAddress: %w", err)
	}
	return a, nil
}

func (s *Service) CreateAddress(ctx context.Context, userID string, req models.CreateAddressRequest) (*models.UserAddress, error) {
	a, err := s.repo.Create(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("service.CreateAddress: %w", err)
	}
	return a, nil
}

func (s *Service) UpdateAddress(ctx context.Context, userID string, id int64, req models.UpdateAddressRequest) (*models.UserAddress, error) {
	a, err := s.repo.Update(ctx, userID, id, req)
	if err != nil {
		return nil, fmt.Errorf("service.UpdateAddress: %w", err)
	}
	return a, nil
}

func (s *Service) DeleteAddress(ctx context.Context, userID string, id int64) error {
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("service.DeleteAddress: %w", err)
	}
	return nil
}

func (s *Service) SetDefaultAddress(ctx context.Context, userID string, id int64) error {
	if err := s.repo.SetDefault(ctx, userID, id); err != nil {
		return fmt.Errorf("service.SetDefaultAddress: %w", err)
	}
	return nil
}
