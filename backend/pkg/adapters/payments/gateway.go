// Package payments defines the Gateway adapter interface for the payment
// service providers (TDD §10): Airwallex (card, settles to CNY) and PayPal
// (parallel checkout) both implement it, selected by env (PAYMENTS_MODE =
// mock | sandbox | live). The MockGateway is the fully-tested dev impl.
//
// Integration model (PRD §3.2.3): hosted checkout — CreateIntent returns a
// hosted redirect URL the customer pays at off-site; the gateway's webhook
// drives the order created→paid. Full refunds only, to the original payment
// method, in the original presentment currency.
package payments

import (
	"context"
	"encoding/json"
	"errors"
)

// Gateway is the contract every payment provider satisfies. Services depend on
// this interface, never on a concrete client, so swapping sandbox→live is an
// env-var flip (TDD §4.1, §10).
type Gateway interface {
	// Name is the gateway slug ("airwallex" | "paypal" | "mock") — stored on the
	// payments row + used by the registry to resolve a gateway by name.
	Name() string

	// CreateIntent creates a payment intent for an order and returns a hosted
	// checkout URL (the customer pays off-site) + the gateway reference stored
	// on the payments row.
	CreateIntent(ctx context.Context, req IntentRequest) (IntentResponse, error)

	// Refund issues a full refund for a succeeded payment (PRD §3.2.3: full
	// refunds only, to the original payment method, in the original currency).
	Refund(ctx context.Context, req RefundRequest) (RefundResponse, error)

	// VerifyWebhook verifies the gateway's signature on a raw webhook body +
	// returns the normalized event. Returns ErrWebhookSignatureInvalid on
	// mismatch (security boundary — never trust an unverified webhook).
	VerifyWebhook(ctx context.Context, rawBody []byte, headers map[string]string) (WebhookEvent, error)
}

// ErrWebhookSignatureInvalid is returned by VerifyWebhook when the gateway's
// signature does not match. The payment service maps it to
// models.ErrWebhookSignatureInvalid for the API response.
var ErrWebhookSignatureInvalid = errors.New("webhook signature verification failed")

// IntentRequest creates a payment intent.
type IntentRequest struct {
	OrderID     int64  // the platform order id (drives the webhook→payment:finalize)
	AmountMinor int64  // presentment minor units (matches the order's total_minor)
	Currency    string // presentment currency (USD/EUR/GBP)
	Reference   string // merchant order reference shown to the customer (e.g. "JDZ-#42")
	ReturnURL   string // where the gateway redirects after payment
}

// IntentResponse is the result of CreateIntent.
type IntentResponse struct {
	GatewayRef string // gateway intent/order id (UNIQUE-ish; stored on payments.gateway_ref)
	HostedURL  string // hosted checkout URL the client redirects to
}

// RefundRequest issues a full refund for a succeeded payment.
type RefundRequest struct {
	GatewayRef  string // the succeeded payment's gateway_ref
	AmountMinor int64  // full amount (presentment minor units)
	Currency    string // original presentment currency
	Reason      string // optional reason
}

// RefundResponse is the result of Refund.
type RefundResponse struct {
	RefundRef string // gateway refund reference (for audit)
}

// WebhookEvent is the normalized result of a verified webhook.
type WebhookEvent struct {
	OrderID        int64           // platform order id
	GatewayRef     string          // the payment's gateway_ref (matches a payments row)
	AmountMinor    int64           // confirmed amount (presentment minor units)
	Currency       string          // confirmed currency
	Status         WebhookStatus   // succeeded | failed
	IdempotencyKey string          // gateway ref + event id → UNIQUE on payments
	Raw            json.RawMessage // the verified raw body (audit)
}

// WebhookStatus is the normalized gateway event status.
type WebhookStatus string

const (
	WebhookSucceeded WebhookStatus = "succeeded"
	WebhookFailed    WebhookStatus = "failed"
)
