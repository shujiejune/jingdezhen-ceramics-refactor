package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Product + SKU: M2 commerce catalog (TDD §3.4, PRD §3.2.1)
//
// The parent `products` row holds non-localized data: artist link, category,
// thumbnail, display order, timestamps. Each (product, locale) has a
// translation row carrying the localized title, slug, description, meta tags,
// and an independent editorial workflow status (PRD §3.1.1). Products link to
// artist profiles (PRD §3.2.1) and are browsed in the public catalog.
//
// SKUs are the purchasable units. A product may have one SKU (one-of-a-kind
// artwork, stock=1) or many SKUs (limited edition variants). SKUs are NOT
// localized — they carry price (CNY minor units), stock, packed weight (for
// shipping), and a JSONB attribute map (size, technique, glaze, edition type,
// year, kiln — PRD §3.2.1). The JSONB map means new attributes require no
// migrations.
// =============================================================================

// Product is the merged public view: parent fields + the requested locale's
// translation. Scanned from a JOIN of products + product_translations.
type Product struct {
	ID              int64          `json:"id" db:"id"`
	ArtistID        *int64         `json:"artist_id,omitempty" db:"artist_id"`     // parent: FK to artists.id
	ArtistName      *string        `json:"artist_name,omitempty" db:"-"`           // populated by service (joined or from artist_translations)
	ArtistSlug      *string        `json:"artist_slug,omitempty" db:"-"`           // for cross-link to artist profile
	Category        *string        `json:"category,omitempty" db:"category"`       // parent: bare string for now (dead post-media; kept for back-comat)
	ThumbnailURL    *string        `json:"thumbnail_url,omitempty" db:"thumbnail_url"` // parent: primary display (dead post-media; gallery preferred)
	DisplayOrder    int            `json:"display_order" db:"display_order"`       // parent
	// Translation fields.
	Title           string         `json:"title" db:"title"`
	Slug            string         `json:"slug" db:"slug"`
	Description     *string        `json:"description,omitempty" db:"description"`
	MetaTitle       *string        `json:"meta_title,omitempty" db:"meta_title"`
	MetaDescription *string        `json:"meta_description,omitempty" db:"meta_description"`
	Locale          string         `json:"locale" db:"locale"`
	Status          ContentStatus  `json:"status" db:"status"`
	PublishedAt     *time.Time     `json:"published_at,omitempty" db:"published_at"`
	// SKUs loaded by the service (detail view only; omitted from list view).
	SKUs            []SKU          `json:"skus,omitempty" db:"-"`
	// Gallery loaded by the service (detail view only). Empty when no
	// product_media rows. The first item's media PublicURL is the preferred
	// thumbnail; ThumbnailURL above is the fallback for back-comat.
	Gallery         []ProductMediaItem `json:"gallery,omitempty" db:"-"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`             // parent
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`             // parent
}

// SKU is a purchasable variant of a product. Money is stored as BIGINT minor
// units (fen for CNY) — never floats (TDD §3.1). The attributes JSONB holds the
// launch attribute set (size, technique, glaze, edition type, year, kiln).
type SKU struct {
	ID                 int64           `json:"id" db:"id"`
	ProductID          int64           `json:"product_id" db:"product_id"`
	SKUCode            string          `json:"sku_code" db:"sku_code"`
	PriceCNY           int64           `json:"price_cny" db:"price_cny"`               // minor units (fen)
	Stock              int             `json:"stock" db:"stock"`
	WeightGrams        int             `json:"weight_grams" db:"weight_grams"`          // packed weight
	LowStockThreshold  int             `json:"low_stock_threshold" db:"low_stock_threshold"`
	Attributes         json.RawMessage `json:"attributes" db:"attributes"`             // JSONB map
	IsActive           bool            `json:"is_active" db:"is_active"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
	// Presentment-currency price (populated when the request carries ?currency=).
	// Nil when no conversion was requested. TDD §7: converted at read time from
	// the cached fx_rates, with the PRD rounding rule applied.
	Price         *int64  `json:"price,omitempty" db:"-"`         // presentment minor units
	PriceCurrency *string `json:"price_currency,omitempty" db:"-"` // ISO 4217
}

// --- Admin / CMS DTOs ---

// CreateProductData creates a new product + its first translation in one call.
// Locale defaults to en-US if empty.
type CreateProductData struct {
	Locale          string `json:"locale,omitempty" validate:"omitempty,len=5"`
	Title           string `json:"title" validate:"required,max=255"`
	Slug            string `json:"slug" validate:"required,max=100"`
	Description     string `json:"description,omitempty"`
	MetaTitle       string `json:"meta_title,omitempty" validate:"omitempty,max=255"`
	MetaDescription string `json:"meta_description,omitempty"`
	ArtistID        *int64 `json:"artist_id,omitempty"`
	Category        string `json:"category,omitempty" validate:"omitempty,max=100"`
	ThumbnailURL    string `json:"thumbnail_url,omitempty" validate:"omitempty,url"`
	DisplayOrder    int    `json:"display_order" validate:"gte=0"`
}

// UpdateProductData updates a product's translation (localized fields) and/or
// the parent's non-localized fields. A nil pointer = leave unchanged.
type UpdateProductData struct {
	Title           *string `json:"title,omitempty" validate:"omitempty,max=255"`
	Slug            *string `json:"slug,omitempty" validate:"omitempty,max=100"`
	Description     *string `json:"description,omitempty"`
	MetaTitle       *string `json:"meta_title,omitempty" validate:"omitempty,max=255"`
	MetaDescription *string `json:"meta_description,omitempty"`
	ArtistID        *int64  `json:"artist_id,omitempty"`
	Category        *string `json:"category,omitempty" validate:"omitempty,max=100"`
	ThumbnailURL    *string `json:"thumbnail_url,omitempty" validate:"omitempty,url"`
	DisplayOrder    *int    `json:"display_order,omitempty" validate:"omitempty,gte=0"`
}

// CreateSKUData creates a new SKU under a product.
type CreateSKUData struct {
	SKUCode           string          `json:"sku_code" validate:"required,max=100"`
	PriceCNY          int64           `json:"price_cny" validate:"gte=0"`
	Stock             int             `json:"stock" validate:"gte=0"`
	WeightGrams       int             `json:"weight_grams" validate:"gte=0"`
	LowStockThreshold *int            `json:"low_stock_threshold,omitempty" validate:"omitempty,gte=0"`
	Attributes        json.RawMessage `json:"attributes,omitempty"`
	IsActive          *bool           `json:"is_active,omitempty"`
}

// UpdateSKUData updates a SKU. A nil pointer = leave unchanged.
type UpdateSKUData struct {
	SKUCode           *string         `json:"sku_code,omitempty" validate:"omitempty,max=100"`
	PriceCNY          *int64          `json:"price_cny,omitempty" validate:"omitempty,gte=0"`
	Stock             *int            `json:"stock,omitempty" validate:"omitempty,gte=0"`
	WeightGrams       *int            `json:"weight_grams,omitempty" validate:"omitempty,gte=0"`
	LowStockThreshold *int            `json:"low_stock_threshold,omitempty" validate:"omitempty,gte=0"`
	Attributes        json.RawMessage `json:"attributes,omitempty"`
	IsActive          *bool           `json:"is_active,omitempty"`
}
