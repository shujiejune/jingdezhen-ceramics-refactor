package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Wishlist (PRD §3.5, TDD §3.4)
//
// Favorites are keyed on SKU (the purchasable unit), not product — a customer
// favorites a specific variant. The wishlist read path JOINs wishlists → skus
// → products → product_translations to surface display info (product title,
// thumbnail, price) for the locale the customer is browsing in.
// =============================================================================

// WishlistItem is the enriched wishlist row returned to the frontend: the
// SKU + the parent product's display info for the requested locale.
type WishlistItem struct {
	SkuID         int64          `json:"sku_id"`
	SKUCode        string         `json:"sku_code"`
	PriceCNY       int64          `json:"price_cny"` // minor units (fen)
	Stock          int            `json:"stock"`
	ProductID      int64          `json:"product_id"`
	ProductSlug    string         `json:"product_slug"`     // translation
	ProductTitle   string         `json:"product_title"`    // translation
	ThumbnailURL   *string        `json:"thumbnail_url,omitempty"`
	ArtistName     *string        `json:"artist_name,omitempty"`
	Attributes     json.RawMessage `json:"attributes,omitempty"`
	FavoritedAt    time.Time      `json:"favorited_at"`
}

// AddWishlistRequest is the body for POST /wishlist.
type AddWishlistRequest struct {
	SkuID int64 `json:"sku_id" validate:"required,gt=0"`
}
