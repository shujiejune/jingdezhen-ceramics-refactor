package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Order + OrderItem (PRD §3.2.3, TDD §3.4/§7/§8)
//
// Orders are immutable snapshots: at checkout the presentment totals
// (subtotal/shipping/total minor units + currency + fx_rate_used) and the CNY
// totals (for settlement) are frozen on the order. Later product edits, FX rate
// changes, or cart changes never affect a placed order. Each line snapshots the
// product title + SKU attributes at purchase time (order_items survive product
// edits).
//
// Lifecycle (TDD §8):
//   created → paid → shipped → completed
//   created → cancelled (stock restored)
//   paid|shipped → refunded (full refunds only)
//
// Money is BIGINT minor units everywhere (TDD §7); never floats.
// =============================================================================

// OrderStatus is the order state-machine value.
type OrderStatus string

const (
	StatusCreated   OrderStatus = "created"
	StatusPaid      OrderStatus = "paid"
	StatusShipped   OrderStatus = "shipped"
	StatusCompleted OrderStatus = "completed"
	StatusCancelled OrderStatus = "cancelled"
	StatusRefunded  OrderStatus = "refunded"
)

// Order is the order header + its items (loaded for detail views).
type Order struct {
	ID              int64          `json:"id" db:"id"`
	UserID          string         `json:"user_id" db:"user_id"`
	Status          OrderStatus    `json:"status" db:"status"`
	Currency        string         `json:"currency" db:"currency"`
	SubtotalMinor   int64          `json:"subtotal_minor" db:"subtotal_minor"`
	ShippingMinor   int64          `json:"shipping_minor" db:"shipping_minor"`
	TotalMinor      int64          `json:"total_minor" db:"total_minor"`
	SubtotalCNY     int64          `json:"subtotal_cny" db:"subtotal_cny"`
	ShippingCNY     int64          `json:"shipping_cny" db:"shipping_cny"`
	TotalCNY        int64          `json:"total_cny" db:"total_cny"`
	FxRateUsed      *float64       `json:"fx_rate_used,omitempty" db:"-"` // rendered from string
	FxRateUsedRaw   *string         `json:"-" db:"fx_rate_used"`
	Address         json.RawMessage `json:"address" db:"address"`
	Locale          *string        `json:"locale,omitempty" db:"locale"`
	CarrierName     *string        `json:"carrier_name,omitempty" db:"carrier_name"`
	TrackingNumber  *string        `json:"tracking_number,omitempty" db:"tracking_number"`
	PlacedAt        time.Time      `json:"placed_at" db:"placed_at"`
	PaidAt          *time.Time     `json:"paid_at,omitempty" db:"paid_at"`
	ShippedAt       *time.Time     `json:"shipped_at,omitempty" db:"shipped_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty" db:"completed_at"`
	CancelledAt     *time.Time     `json:"cancelled_at,omitempty" db:"cancelled_at"`
	RefundedAt      *time.Time     `json:"refunded_at,omitempty" db:"refunded_at"`
	CancelReason    *string        `json:"cancel_reason,omitempty" db:"cancel_reason"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
	// Items loaded by the service (detail view only).
	Items           []OrderItem    `json:"items,omitempty" db:"-"`
	// HostedURL is the gateway's hosted-checkout URL (sandbox/live). Empty in
	// mock mode. The client redirects the customer here to pay off-site.
	HostedURL       string         `json:"hosted_url,omitempty" db:"-"`
}

// OrderItem is an immutable line snapshot on an order.
type OrderItem struct {
	ID                int64           `json:"id" db:"id"`
	OrderID           int64           `json:"order_id" db:"order_id"`
	SkuID             int64           `json:"sku_id" db:"sku_id"`
	Qty               int             `json:"qty" db:"qty"`
	UnitPriceMinor    int64           `json:"unit_price_minor" db:"unit_price_minor"` // presentment
	UnitPriceCNY      int64           `json:"unit_price_cny" db:"unit_price_cny"`     // CNY base
	TitleSnapshot    json.RawMessage `json:"title_snapshot,omitempty" db:"title_snapshot"`
	AttributesSnapshot json.RawMessage `json:"attributes_snapshot,omitempty" db:"attributes_snapshot"`
	CreatedAt         time.Time       `json:"created_at" db:"created_at"`
}

// --- Request DTOs ---

// CheckoutRequest is the body for POST /checkout. The address must belong to
// the signed-in user; currency defaults to the user's preferred_currency (or
// USD) if omitted (PRD §3.2.3). Gateway is required in sandbox/live mode
// ("airwallex" | "paypal"); ignored in mock mode (dev auto-succeeds).
type CheckoutRequest struct {
	AddressID int64  `json:"address_id" validate:"required,gt=0"`
	Currency  string `json:"currency,omitempty" validate:"omitempty,len=3"`
	Gateway   string `json:"gateway,omitempty" validate:"omitempty,oneof=airwallex paypal mock"`
}

// ShipOrderRequest is the body for POST /admin/orders/:id/ship. Operator enters
// the carrier name + tracking number (PRD §3.2.3: no carrier API integration).
type ShipOrderRequest struct {
	CarrierName    string `json:"carrier_name" validate:"required,max=100"`
	TrackingNumber string `json:"tracking_number" validate:"required,max=200"`
}

// CancelOrderRequest is the optional body for POST /orders/:id/cancel.
type CancelOrderRequest struct {
	Reason string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// RefundOrderRequest is the optional body for POST /admin/orders/:id/refund.
type RefundOrderRequest struct {
	Reason string `json:"reason,omitempty" validate:"omitempty,max=500"`
}
