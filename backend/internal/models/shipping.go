package models

import "time"

// =============================================================================
// Shipping fee tiers (PRD §3.2.3, TDD §3.7/§3.4)
//
// Per-country, weight-tiered fee table maintained by E-commerce Operators in
// the CMS. Fees are CNY minor units (fen). The calculator
// (internal/platform/shipping.CalcFee) maps an order weight to a tier.
// =============================================================================

// ShippingFeeTier is one row of the per-country shipping-fee table.
type ShippingFeeTier struct {
	ID             int64     `json:"id" db:"id"`
	Country        string    `json:"country" db:"country"` // ISO 3166-1 alpha-2
	MaxWeightGrams int       `json:"max_weight_grams" db:"max_weight_grams"`
	FeeCNY         int64     `json:"fee_cny" db:"fee_cny"` // minor units (fen)
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// ShippingQuoteResponse is the result of GET /shipping/quote (public preview).
type ShippingQuoteResponse struct {
	Country     string `json:"country"`
	WeightGrams int    `json:"weight_grams"`
	FeeCNY      int64  `json:"fee_cny"`          // minor units (fen); 0 if unshippable/overweight
	Shippable   bool   `json:"shippable"`        // false if unshippable OR overweight
	Reason      string `json:"reason,omitempty"` // "unshippable" | "overweight" | ""
}

// CreateShippingTierRequest is the body for POST /admin/shipping/tiers.
type CreateShippingTierRequest struct {
	Country        string `json:"country" validate:"required,len=2"`
	MaxWeightGrams int    `json:"max_weight_grams" validate:"required,gt=0"`
	FeeCNY         int64  `json:"fee_cny" validate:"gte=0"`
}

// UpdateShippingTierRequest is the body for PUT /admin/shipping/tiers/:id.
type UpdateShippingTierRequest struct {
	Country        string `json:"country" validate:"required,len=2"`
	MaxWeightGrams int    `json:"max_weight_grams" validate:"required,gt=0"`
	FeeCNY         int64  `json:"fee_cny" validate:"gte=0"`
}
