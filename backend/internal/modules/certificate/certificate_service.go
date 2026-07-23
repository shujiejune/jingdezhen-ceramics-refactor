package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/adapters/certchain"
)

// SaleItemSnapshot has moved to internal/models so order + certificate share
// it without an import edge.

// ServiceInterface defines certificate business logic.
type ServiceInterface interface {
	// IssueForProduct auto-generates a certificate for a product (PRD §3.2.1:
	// generated automatically at product creation). Best-effort caller wraps
	// errors so product creation never fails on issuance.
	IssueForProduct(ctx context.Context, productID int64) error
	// GetByCode loads the public certificate (product + artist + provenance).
	GetByCode(ctx context.Context, code, locale string) (*models.Certificate, error)
	// ListAdmin paginates all certificates.
	ListAdmin(ctx context.Context, page, limit int) ([]models.Certificate, int, error)
	// GetByID loads a certificate by id (admin detail).
	GetByID(ctx context.Context, id int64) (*models.Certificate, error)
	// Regenerate issues a new cert_code + a `created` provenance (PRD: operators
	// can regenerate). Returns the new code.
	Regenerate(ctx context.Context, id int64) (string, error)
	// RecordSale appends a `sold` provenance record per order item's product
	// certificate (TDD §8: provenance at `paid`). Called by order.MarkPaid.
	// Best-effort: logs + skips a missing cert (e.g. a product with no cert yet).
	RecordSale(ctx context.Context, orderID int64, items []models.SaleItemSnapshot) error
}

type Service struct {
	repo  RepositoryInterface
	chain certchain.Chain
}

func NewService(repo RepositoryInterface, chain certchain.Chain) ServiceInterface {
	return &Service{repo: repo, chain: chain}
}

func (s *Service) IssueForProduct(ctx context.Context, productID int64) error {
	code, err := s.repo.NextCode(ctx)
	if err != nil {
		return fmt.Errorf("certificate.IssueForProduct.NextCode: %w", err)
	}
	// Register on-chain (Noop for v1, PRD §5.4) before the insert so a chain
	// failure surfaces before the cert row exists. The Noop never fails.
	detail, _ := json.Marshal(map[string]any{"product_id": productID})
	if _, err := s.chain.RegisterCreation(ctx, code, productID, detail); err != nil {
		return fmt.Errorf("certificate.IssueForProduct.Chain: %w", err)
	}
	if _, err := s.repo.Issue(ctx, productID, code); err != nil {
		// ErrConflict = a cert already exists for the product (idempotent re-call);
		// surface as a no-op success so the caller's best-effort wrapper is happy.
		if errors.Is(err, models.ErrConflict) {
			return nil
		}
		return fmt.Errorf("certificate.IssueForProduct.Issue: %w", err)
	}
	return nil
}

func (s *Service) GetByCode(ctx context.Context, code, locale string) (*models.Certificate, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	c, err := s.repo.GetByCode(ctx, code, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("certificate.GetByCode: %w", err)
	}
	if c.Provenance == nil {
		c.Provenance = []models.ProvenanceRecord{}
	}
	return c, nil
}

func (s *Service) ListAdmin(ctx context.Context, page, limit int) ([]models.Certificate, int, error) {
	return s.repo.ListAdmin(ctx, page, limit)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*models.Certificate, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, models.ErrNotFound
		}
		return nil, err
	}
	prov, err := s.repo.LoadProvenance(ctx, c.ID)
	if err == nil {
		c.Provenance = prov
	}
	if c.Provenance == nil {
		c.Provenance = []models.ProvenanceRecord{}
	}
	return c, nil
}

func (s *Service) Regenerate(ctx context.Context, id int64) (string, error) {
	return s.repo.RegenerateCode(ctx, id)
}

func (s *Service) RecordSale(ctx context.Context, orderID int64, items []models.SaleItemSnapshot) error {
	skuIDs := make([]int64, 0, len(items))
	for _, it := range items {
		skuIDs = append(skuIDs, it.SkuID)
	}
	certs, err := s.repo.FindCertsBySKUs(ctx, skuIDs)
	if err != nil {
		return fmt.Errorf("certificate.RecordSale.Find: %w", err)
	}
	for _, it := range items {
		cert, ok := certs[it.SkuID]
		if !ok {
			// A product without a cert (shouldn't happen — certs are auto-issued
			// at product creation). Skip + log; the cert is the source of truth
			// and a missed provenance row can be backfilled.
			log.Printf("certificate.RecordSale: no cert for sku %d (order=%d)", it.SkuID, orderID)
			continue
		}
		detail, _ := json.Marshal(map[string]any{
			"order_id": orderID, "sku_id": it.SkuID, "qty": it.Qty,
		})
		if _, err := s.chain.RegisterSale(ctx, cert.CertCode, orderID, detail); err != nil {
			log.Printf("certificate.RecordSale.Chain(cert=%s order=%d): %v", cert.CertCode, orderID, err)
			// continue — the provenance row is still appended below
		}
		if err := s.repo.AppendProvenance(ctx, cert.ID, models.ProvenanceSold, detail); err != nil {
			log.Printf("certificate.RecordSale.Append(cert=%s order=%d): %v", cert.CertCode, orderID, err)
		}
	}
	return nil
}
