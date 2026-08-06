package product

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/sitemap"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines product/SKU business logic (i18n-aware).
type ServiceInterface interface {
	// --- Public reads ---
	GetProducts(ctx context.Context, locale, category string, artistID int64, tags []string, page, limit int) ([]models.Product, int, error)
	GetProductBySlug(ctx context.Context, slug string, locale string) (*models.Product, error)
	GetTags(ctx context.Context, locale string) ([]models.TagWithCount, error)

	// --- Admin / CMS (products) ---
	AdminListProducts(ctx context.Context, locale, status string, tags []string, page, limit int) ([]models.Product, int, error)
	AdminGetProduct(ctx context.Context, slug string, locale string) (*models.Product, error)
	AdminCreateProduct(ctx context.Context, data models.CreateProductData) (*models.Product, error)
	AdminBulkImport(ctx context.Context, rows []models.BulkImportRow) (models.BulkImportSummary, error)
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
	repo            RepositoryInterface
	certIssuer      CertificateIssuer // optional; nil => no auto-issue (e.g. worker mode)
	galleryLoader   GalleryLoader     // optional; nil => no gallery (list view)
	sitemapEnqueuer sitemap.Enqueuer  // optional; nil => no sitemap rebuild (worker/tests)
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

// SetSitemapEnqueuer wires the sitemap-rebuild trigger (PRD §4.4). Called in
// main.go after the product service + jobs client are built. nil-safe.
func (s *Service) SetSitemapEnqueuer(e sitemap.Enqueuer) { s.sitemapEnqueuer = e }

// enqueueSitemapRebuild fires a sitemap rebuild best-effort (PRD §4.4). nil-
// safe (worker/tests); logs on error, never returns one — a Redis blip must
// not fail a content transition (the next publish rebuilds; /sitemap.xml
// rebuilds on read).
func (s *Service) enqueueSitemapRebuild(ctx context.Context) {
	if s.sitemapEnqueuer == nil {
		return
	}
	if err := s.sitemapEnqueuer.EnqueueSitemapRebuild(ctx); err != nil {
		log.Printf("product.enqueueSitemapRebuild: %v", err)
	}
}

// --- Public ---

func (s *Service) GetProducts(ctx context.Context, locale, category string, artistID int64, tags []string, page, limit int) ([]models.Product, int, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	products, total, err := s.repo.FindAllPublished(ctx, locale, category, artistID, tags, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetProducts: %w", err)
	}
	// Batch-load tags (locale-resolved name) for the page — no N+1.
	if len(products) > 0 {
		ids := make([]int64, len(products))
		for i := range products {
			ids[i] = products[i].ID
		}
		tagMap, terr := s.repo.FindTagsByProductIDs(ctx, ids, locale)
		if terr != nil {
			log.Printf("product.GetProducts.Tags: %v", terr) // best-effort
		} else {
			for i := range products {
				products[i].Tags = tagMap[products[i].ID]
			}
		}
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
	// Load the product's tags (locale-resolved name).
	tagMap, terr := s.repo.FindTagsByProductIDs(ctx, []int64{product.ID}, locale)
	if terr != nil {
		log.Printf("product.GetProductBySlug.Tags(%d): %v", product.ID, terr)
	} else {
		product.Tags = tagMap[product.ID]
	}
	// Load hreflang alternates (PRD §4.4): locale→slug for every OTHER published
	// translation. Best-effort — a failure logs + leaves Alternates empty; the
	// detail still renders (the frontend just emits no <link rel=alternate>).
	alts, aerr := s.repo.FindPublishedAlternates(ctx, product.ID, locale)
	if aerr != nil {
		log.Printf("product.GetProductBySlug.Alternates(%d): %v", product.ID, aerr)
	} else if len(alts) > 0 {
		product.Alternates = alts
	}
	return product, nil
}

// --- Admin / CMS (products) ---

func (s *Service) AdminListProducts(ctx context.Context, locale, status string, tags []string, page, limit int) ([]models.Product, int, error) {
	if locale != "" {
		var err error
		locale, err = i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			return nil, 0, models.ErrInvalidLocale
		}
	}
	products, total, err := s.repo.FindAllAdmin(ctx, locale, status, tags, page, limit)
	if err != nil {
		return nil, 0, err
	}
	// Batch-load tags (locale-resolved name) for the admin list too.
	if len(products) > 0 {
		ids := make([]int64, len(products))
		for i := range products {
			ids[i] = products[i].ID
		}
		tagMap, terr := s.repo.FindTagsByProductIDs(ctx, ids, locale)
		if terr != nil {
			log.Printf("product.AdminListProducts.Tags: %v", terr)
		} else {
			for i := range products {
				products[i].Tags = tagMap[products[i].ID]
			}
		}
	}
	return products, total, nil
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
	// Load the product's tags (locale-resolved name).
	tagMap, terr := s.repo.FindTagsByProductIDs(ctx, []int64{p.ID}, locale)
	if terr != nil {
		log.Printf("product.AdminGetProduct.Tags(%d): %v", p.ID, terr)
	} else {
		p.Tags = tagMap[p.ID]
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
	// Attach the tags we just set so the create response is complete.
	tagMap, terr := s.repo.FindTagsByProductIDs(ctx, []int64{p.ID}, locale)
	if terr != nil {
		log.Printf("product.AdminCreateProduct.Tags(%d): %v", p.ID, terr)
	} else {
		p.Tags = tagMap[p.ID]
	}
	return p, nil
}

// AdminBulkImport creates products (+ their first SKU) from CSV rows, one tx
// per row so a bad row doesn't poison good ones. Returns a per-row report
// (PRD §3.4.1 line 175: "bulk upload CSV/Excel import"). Multi-SKU products
// are created via the regular per-product SKU endpoint after import.
func (s *Service) AdminBulkImport(ctx context.Context, rows []models.BulkImportRow) (models.BulkImportSummary, error) {
	summary := models.BulkImportSummary{Results: []models.BulkImportResult{}}
	for i, row := range rows {
		res := models.BulkImportResult{Row: i + 1}
		if row.Title == "" || row.Slug == "" {
			res.Error = "missing required field: title or slug"
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		locale := row.Locale
		if locale == "" {
			locale = "en-US"
		}
		loc, err := i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			res.Error = "invalid locale: " + locale
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		data := models.CreateProductData{
			Locale: loc, Title: row.Title, Slug: row.Slug, Category: row.Category,
			ArtistID: row.ArtistID, ThumbnailURL: row.ThumbnailURL,
			DisplayOrder: row.DisplayOrder, Description: row.Description,
			Tags: row.Tags,
		}
		p, err := s.repo.CreateWithTranslation(ctx, data)
		if err != nil {
			res.Error = fmt.Sprintf("create product: %v", err)
			summary.Failed++
			summary.Results = append(summary.Results, res)
			continue
		}
		res.ProductID = p.ID
		// Best-effort cert auto-issue (same as AdminCreateProduct).
		if s.certIssuer != nil {
			if err := s.certIssuer.IssueForProduct(ctx, p.ID); err != nil {
				log.Printf("product.BulkImport.IssueCert(product=%d): %v", p.ID, err)
			}
		}
		// Optional first SKU.
		if row.SKUCode != "" {
			skuData := models.CreateSKUData{
				SKUCode: row.SKUCode, PriceCNY: row.PriceCNY, Stock: row.Stock,
				WeightGrams: row.WeightGrams, LowStockThreshold: row.LowStockThreshold,
			}
			if row.Attributes != "" {
				skuData.Attributes = json.RawMessage(row.Attributes)
			}
			sku, err := s.repo.CreateSKU(ctx, p.ID, skuData)
			if err != nil {
				res.Error = fmt.Sprintf("product %d created but SKU failed: %v", p.ID, err)
				summary.Failed++
				summary.Results = append(summary.Results, res)
				continue
			}
			res.SKUCode = sku.SKUCode
		}
		summary.Imported++
		summary.Results = append(summary.Results, res)
	}
	return summary, nil
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
	p, err := s.repo.UpdateTranslation(ctx, productID, locale, data)
	if err != nil {
		return nil, err
	}
	// Attach the (possibly updated) tag set so the update response is complete.
	tagMap, terr := s.repo.FindTagsByProductIDs(ctx, []int64{p.ID}, locale)
	if terr != nil {
		log.Printf("product.AdminUpdateProduct.Tags(%d): %v", p.ID, terr)
	} else {
		p.Tags = tagMap[p.ID]
	}
	return p, nil
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
	// Rebuild the sitemap on publish or unpublish (PRD §4.4). Best-effort —
	// a Redis blip logs + doesn't fail the transition (the next publish/
	// unpublish rebuilds, and /sitemap.xml rebuilds on read anyway).
	if to == models.StatusPublished || from == models.StatusPublished {
		s.enqueueSitemapRebuild(ctx)
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

// GetTags lists tags attached to ≥1 published product, with the locale-resolved
// display name + a product count (public facet list, GET /catalog/tags).
func (s *Service) GetTags(ctx context.Context, locale string) ([]models.TagWithCount, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	return s.repo.FindAllTagsInUse(ctx, locale)
}
