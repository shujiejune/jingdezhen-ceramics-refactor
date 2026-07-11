package product

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines product/SKU business logic (i18n-aware).
type ServiceInterface interface {
	// --- Public reads ---
	GetProducts(ctx context.Context, locale, category string, artistID int64, page, limit int) ([]models.Product, int, error)
	GetProductBySlug(ctx context.Context, slug string, locale string) (*models.Product, error)

	// --- Admin / CMS (products) ---
	AdminListProducts(ctx context.Context, locale, status string, page, limit int) ([]models.Product, int, error)
	AdminGetProduct(ctx context.Context, slug string, locale string) (*models.Product, error)
	AdminCreateProduct(ctx context.Context, data models.CreateProductData) (*models.Product, error)
	AdminUpdateProduct(ctx context.Context, productID int64, locale string, data models.UpdateProductData, actor i18ncontent.WorkflowActor) (*models.Product, error)
	AdminTransitionProduct(ctx context.Context, productID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Product, error)
	AdminDeleteProduct(ctx context.Context, productID int64) error

	// --- Admin / CMS (SKUs) ---
	AdminCreateSKU(ctx context.Context, productID int64, data models.CreateSKUData) (*models.SKU, error)
	AdminUpdateSKU(ctx context.Context, skuID int64, data models.UpdateSKUData) (*models.SKU, error)
	AdminDeleteSKU(ctx context.Context, skuID int64) error

	// --- Catalog helpers ---
	GetCategories(ctx context.Context) ([]string, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// --- Public ---

func (s *Service) GetProducts(ctx context.Context, locale, category string, artistID int64, page, limit int) ([]models.Product, int, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	products, total, err := s.repo.FindAllPublished(ctx, locale, category, artistID, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetProducts: %w", err)
	}
	return products, total, nil
}

func (s *Service) GetProductBySlug(ctx context.Context, slug string, locale string) (*models.Product, error) {
	if slug == "" {
		return nil, fmt.Errorf("service.GetProductBySlug: slug cannot be empty")
	}
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	product, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetProductBySlug: %w", err)
	}
	// Load SKUs for the detail view.
	skus, err := s.repo.FindSKUsByProductID(ctx, product.ID)
	if err != nil {
		return nil, fmt.Errorf("service.GetProductBySlug.SKUs: %w", err)
	}
	product.SKUs = skus
	return product, nil
}

// --- Admin / CMS (products) ---

func (s *Service) AdminListProducts(ctx context.Context, locale, status string, page, limit int) ([]models.Product, int, error) {
	if locale != "" {
		var err error
		locale, err = i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			return nil, 0, models.ErrInvalidLocale
		}
	}
	return s.repo.FindAllAdmin(ctx, locale, status, page, limit)
}

func (s *Service) AdminGetProduct(ctx context.Context, slug string, locale string) (*models.Product, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	return s.repo.FindAdminBySlug(ctx, locale, slug)
}

func (s *Service) AdminCreateProduct(ctx context.Context, data models.CreateProductData) (*models.Product, error) {
	locale, err := i18ncontent.NormalizeLocale(data.Locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	data.Locale = locale
	return s.repo.CreateWithTranslation(ctx, data)
}

func (s *Service) AdminUpdateProduct(ctx context.Context, productID int64, locale string, data models.UpdateProductData, actor i18ncontent.WorkflowActor) (*models.Product, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	currentStatus, err := s.repo.GetTranslationStatus(ctx, productID, locale)
	if err != nil {
		return nil, err
	}
	if !i18ncontent.CanEdit(currentStatus, actor) {
		return nil, models.ErrInvalidWorkflowTransition
	}
	return s.repo.UpdateTranslation(ctx, productID, locale, data)
}

func (s *Service) AdminTransitionProduct(ctx context.Context, productID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Product, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	from, err := s.repo.GetTranslationStatus(ctx, productID, locale)
	if err != nil {
		return nil, err
	}
	if _, err := i18ncontent.Transition(from, to, actor); err != nil {
		return nil, err
	}
	var reviewer *string
	if reviewerID != "" {
		reviewer = &reviewerID
	}
	if err := s.repo.UpdateTranslationStatus(ctx, productID, locale, to, reviewer); err != nil {
		return nil, err
	}
	return s.repo.FindAdminByID(ctx, productID, locale)
}

func (s *Service) AdminDeleteProduct(ctx context.Context, productID int64) error {
	return s.repo.Delete(ctx, productID)
}

// --- Admin / CMS (SKUs) ---

func (s *Service) AdminCreateSKU(ctx context.Context, productID int64, data models.CreateSKUData) (*models.SKU, error) {
	return s.repo.CreateSKU(ctx, productID, data)
}

func (s *Service) AdminUpdateSKU(ctx context.Context, skuID int64, data models.UpdateSKUData) (*models.SKU, error) {
	return s.repo.UpdateSKU(ctx, skuID, data)
}

func (s *Service) AdminDeleteSKU(ctx context.Context, skuID int64) error {
	return s.repo.DeleteSKU(ctx, skuID)
}

// --- Catalog helpers ---

func (s *Service) GetCategories(ctx context.Context) ([]string, error) {
	return s.repo.FindAllCategories(ctx)
}
