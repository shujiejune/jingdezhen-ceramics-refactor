package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Cart (PRD §3.2.3, TDD §3.4)
//
// Server-side cart for signed-in customers. One cart per user (carts.user_id
// UNIQUE). Items are keyed on SKU (the purchasable unit, like wishlists).
// Guests use a localStorage cart that is merged into the server cart at login
// via POST /cart/merge (PRD §3.2.3).
//
// Stock is NOT decremented at the cart stage — the authoritative atomic
// decrement happens at checkout (TDD §4.3). The qty>stock guard in the cart
// service is advisory (prevents obviously-stale carts).
//
// Money is CNY minor units (fen) at rest; the FX pipeline (TDD §7) converts to
// presentment currency at read time when ?currency= is supplied.
// =============================================================================

// Cart is the merged public view: items enriched with display info for the
// requested locale + totals. CNY totals are always present; presentment totals
// are populated only when ?currency= is supplied and FX rates are available.
type Cart struct {
	Items      []CartItem `json:"items"`
	ItemCount  int        `json:"item_count"` // sum of distinct SKUs (not sum of qty)
	TotalCNY   int64      `json:"total_cny"`  // sum of line_total_cny, minor units (fen)
	Total      *int64     `json:"total,omitempty" db:"-"`         // presentment minor units (when ?currency=)
	Currency   *string    `json:"currency,omitempty" db:"-"`     // ISO 4217 (when ?currency=)
}

// CartItem is a cart line enriched with the parent product's display info for
// the requested locale (published translations only), mirroring WishlistItem.
type CartItem struct {
	SkuID         int64           `json:"sku_id"`
	SKUCode       string          `json:"sku_code"`
	Qty           int             `json:"qty"`
	UnitPriceCNY  int64           `json:"unit_price_cny"` // minor units (fen)
	LineTotalCNY  int64           `json:"line_total_cny"` // unit_price_cny * qty
	Stock         int             `json:"stock"`
	WeightGrams   int             `json:"weight_grams"`
	ProductID     int64           `json:"product_id"`
	ProductSlug   string          `json:"product_slug"`
	ProductTitle  string          `json:"product_title"`
	ThumbnailURL  *string         `json:"thumbnail_url,omitempty"`
	ArtistName    *string         `json:"artist_name,omitempty"`
	Attributes    json.RawMessage `json:"attributes,omitempty"`
	AddedAt       time.Time       `json:"added_at"`
	// Presentment-currency fields (populated when ?currency= is supplied).
	UnitPrice  *int64  `json:"unit_price,omitempty" db:"-"`      // presentment minor units
	LineTotal  *int64  `json:"line_total,omitempty" db:"-"`      // presentment minor units
}

// --- Request DTOs ---

// AddCartItemRequest is the body for POST /cart/items. POST is additive:
// the existing qty for this SKU is incremented by qty (default 1 if omitted).
type AddCartItemRequest struct {
	SkuID int64 `json:"sku_id" validate:"required,gt=0"`
	Qty   int  `json:"qty,omitempty" validate:"omitempty,gte=1"`
}

// UpdateCartItemRequest is the body for PATCH /cart/items/:sku_id. PATCH is a
// set-absolute operation: the qty becomes exactly qty.
type UpdateCartItemRequest struct {
	Qty int `json:"qty" validate:"required,gte=1"`
}

// BulkRemoveRequest is the body for DELETE /cart/items (bulk remove).
type BulkRemoveRequest struct {
	SkuIDs []int64 `json:"sku_ids" validate:"required,min=1,dive,gt=0"`
}

// MergeCartRequest is the body for POST /cart/merge (guest → server cart on
// login). Each guest item is upserted additively, capped at the SKU's stock.
type MergeCartRequest struct {
	Items []MergeCartItem `json:"items" validate:"omitempty,dive"`
}

// MergeCartItem is one line of a guest cart payload.
type MergeCartItem struct {
	SkuID int64 `json:"sku_id" validate:"required,gt=0"`
	Qty   int  `json:"qty" validate:"required,gte=1"`
}
