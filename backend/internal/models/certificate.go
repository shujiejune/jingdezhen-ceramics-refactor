package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Certificate + ProvenanceRecord (PRD §3.2.1/§5.4, TDD §3.4/§8)
//
// Each product gets one certificate (product_id UNIQUE) with a unique cert_code
// + a public QR-target page (GET /certificates/:code). Auto-generated at product
// creation; operators can regenerate. Provenance records append the authenticity
// chain: `created` at issue, `sold` at order-paid (TDD §8), `transferred` later.
//
// qr_key/pdf_key are nullable OSS object keys — the QR is served on-demand
// (GET /certificates/:code/qr) until the storage adapter lands; the printable
// PDF is deferred pending the TDD §12 engine decision.
//
// Reserved blockchain integration (PRD §5.4): the certificate service calls a
// certchain.Chain adapter at issue/sale; Noop for v1.
// =============================================================================

// ProvenanceKind is a provenance record's event type.
type ProvenanceKind string

const (
	ProvenanceCreated     ProvenanceKind = "created"     // certificate issued / regenerated
	ProvenanceSold        ProvenanceKind = "sold"        // order paid (TDD §8)
	ProvenanceTransferred ProvenanceKind = "transferred" // future ownership transfer
)

// Certificate is the certificate header. The public-page view (GetByCode) also
// loads the product/artist display info + provenance chain.
type Certificate struct {
	ID        int64     `json:"id" db:"id"`
	ProductID int64     `json:"product_id" db:"product_id"`
	CertCode  string    `json:"cert_code" db:"cert_code"`
	QRKey     *string   `json:"qr_key,omitempty" db:"qr_key"`
	PDFKey    *string   `json:"pdf_key,omitempty" db:"pdf_key"`
	IssuedAt  time.Time `json:"issued_at" db:"issued_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	// Display fields populated by the locale-aware JOIN (public page) — db:"-"
	ProductTitle string             `json:"product_title,omitempty" db:"-"`
	ProductSlug  string             `json:"product_slug,omitempty" db:"-"`
	ArtistName   *string            `json:"artist_name,omitempty" db:"-"`
	ArtistSlug   *string            `json:"artist_slug,omitempty" db:"-"`
	ThumbnailURL *string            `json:"thumbnail_url,omitempty" db:"-"`
	Attributes   json.RawMessage    `json:"attributes,omitempty" db:"-"` // the product's default-SKU attributes
	Provenance   []ProvenanceRecord `json:"provenance,omitempty" db:"-"`
}

// ProvenanceRecord is one entry in a certificate's authenticity chain.
type ProvenanceRecord struct {
	ID            int64           `json:"id" db:"id"`
	CertificateID int64           `json:"certificate_id" db:"certificate_id"`
	Kind          ProvenanceKind  `json:"kind" db:"kind"`
	Detail        json.RawMessage `json:"detail,omitempty" db:"detail"`
	At            time.Time       `json:"at" db:"at"`
}

// SaleItemSnapshot is the minimal info the order service passes to
// RecordSale (one per order line). Lives in models so the order + certificate
// modules share it without one importing the other. Carries SkuID (which
// order_items snapshots) — the certificate repo resolves product_id via the SKU.
type SaleItemSnapshot struct {
	SkuID int64
	Qty   int
}
