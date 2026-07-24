package product

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"log"
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

// CertificateIssuer auto-generates a digital certificate for a product at
// creation (PRD §3.2.1). Best-effort: the caller logs + continues on error.
type CertificateIssuer interface {
	IssueForProduct(ctx context.Context, productID int64) error
}

type Service struct {
	repo           RepositoryInterface
	certIssuer     CertificateIssuer // optional; nil => no auto-issue (e.g. worker mode)
	galleryLoader  GalleryLoader    // optional; nil => no gallery (list view)
}

// GalleryLoader loads a product's ordered media gallery. Implemented by the
// media service; injected post-construction to avoid a product→media import
// edge (the media module is a sibling).
type GalleryLoader interface {
	ListProductMedia(ctx context.Context, productID int64) ([]models.ProductMediaItem, error)
}

func NewService(repo RepositoryInterface) *Service {
	return &Service{repo: repo}
}

// SetCertificateIssuer wires the auto-issue client post-construction (called
// in main.go after both the product + certificate services are built).
func (s *Service) SetCertificateIssuer(ci CertificateIssuer) { s.certIssuer = ci }

// SetGalleryLoader wires the gallery loader post-construction.
func (s *Service) SetGalleryLoader(gl GalleryLoader) { s.galleryLoader = gl }

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
	// Load the ordered media gallery (detail view). The first item's media
	// PublicURL is the preferred thumbnail; ThumbnailURL is the fallback.
	if s.galleryLoader != nil {
		gallery, gerr := s.galleryLoader.ListProductMedia(ctx, product.ID)
		if gerr != nil {
			log.Printf("product.GetProductBySlug.Gallery(%d): %v", product.ID, gerr)
		} else if len(gallery) > 0 {
			product.Gallery = gallery
			if gallery[0].MediaAsset.PublicURL != "" {
				u := gallery[0].MediaAsset.PublicURL
				product.ThumbnailURL = &u
			}
		}
	}
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
	p, err := s.repo.FindAdminBySlug(ctx, locale, slug)
	if err != nil {
		return nil, err
	}
	// Load the gallery for the admin detail view too (CMS edits the gallery here).
	if s.galleryLoader != nil {
		gallery, gerr := s.galleryLoader.ListProductMedia(ctx, p.ID)
		if gerr != nil {
			log.Printf("product.AdminGetProduct.Gallery(%d): %v", p.ID, gerr)
		} else if len(gallery) > 0 {
			p.Gallery = gallery
			if gallery[0].MediaAsset.PublicURL != "" {
				u := gallery[0].MediaAsset.PublicURL
				p.ThumbnailURL = &u
			}
		}
	}
	return p, nil
}

func (s *Service) AdminCreateProduct(ctx context.Context, data models.CreateProductData) (*models.Product, error) {
	locale, err := i18ncontent.NormalizeLocale(data.Locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	data.Locale = locale
	p, err := s.repo.CreateWithTranslation(ctx, data)
	if err != nil {
		return nil, err
	}
	// Auto-generate the digital certificate (PRD §3.2.1). Best-effort: a
	// certificate error must not fail product creation — the operator can
	// regenerate from the CMS.
	if s.certIssuer != nil {
		if err := s.certIssuer.IssueForProduct(ctx, p.ID); err != nil {
			// Log + continue; the product is created. The cert can be issued
			// later via regenerate (or the admin certificate list will show none).
			fmt.Printf("product.AdminCreateProduct.IssueCert(product=%d): %v\n", p.ID, err)
		}
	}
	return p, nil
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
