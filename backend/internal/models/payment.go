package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Payment (PRD §3.2.3, TDD §3.4/§10)
//
// One payment per gateway intent. The webhook handler upserts by idempotency_key
// (UNIQUE) so a gateway retry or duplicate webhook is a no-op (TDD §11 priority:
// webhook idempotency). The order's created→paid transition is driven by the
// payment:finalize job enqueued after a verified succeeded webhook.
//
// Full refunds only (PRD §3.2.3): a refund moves the row status→refunded and
// the order paid|shipped→refunded, in the original presentment currency.
// =============================================================================

// PaymentStatus is the payment row state.
type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentSucceeded PaymentStatus = "succeeded"
	PaymentFailed   PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
)

// Payment is a payment record (one per gateway intent).
type Payment struct {
	ID             int64           `json:"id" db:"id"`
	OrderID        int64           `json:"order_id" db:"order_id"`
	Gateway        string          `json:"gateway" db:"gateway"`
	GatewayRef     string          `json:"gateway_ref" db:"gateway_ref"`
	Status         PaymentStatus   `json:"status" db:"status"`
	AmountMinor    int64           `json:"amount_minor" db:"amount_minor"`
	Currency       string          `json:"currency" db:"currency"`
	RawWebhook     json.RawMessage `json:"raw_webhook,omitempty" db:"raw_webhook"`
	IdempotencyKey string          `json:"idempotency_key" db:"idempotency_key"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
}

// GatewayName constants — match the payments.gateway CHECK constraint +
// adapters.Gateway.Name(). Stored on the payments row + used by the registry.
const (
	GatewayAirwallex = "airwallex"
	GatewayPayPal    = "paypal"
	GatewayMock      = "mock"
)
