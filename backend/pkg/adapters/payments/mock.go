package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// MockGateway is the fully-tested dev/test impl (TDD §10 "MockGateway in tests").
// CreateIntent returns a synthetic hosted URL + ref; VerifyWebhook parses a
// JSON body so the dev webhook path is exercisable end-to-end without a network.
type MockGateway struct{}

func NewMockGateway() *MockGateway { return &MockGateway{} }

func (MockGateway) Name() string { return "mock" }

func (MockGateway) CreateIntent(_ context.Context, req IntentRequest) (IntentResponse, error) {
	ref := "mock-" + strconv.FormatInt(req.OrderID, 10)
	return IntentResponse{
		GatewayRef: ref,
		HostedURL:  "mock://pay/" + strconv.FormatInt(req.OrderID, 10),
	}, nil
}

func (MockGateway) Refund(_ context.Context, req RefundRequest) (RefundResponse, error) {
	return RefundResponse{RefundRef: "mock-refund-" + req.GatewayRef}, nil
}

// VerifyWebhook (dev): the body is a JSON object
//
//	{"order_id":N, "gateway_ref":"mock-N", "status":"succeeded"|"failed",
//	 "event_id":"<id>", "amount_minor":N, "currency":"USD"}
//
// The idempotency key = gateway_ref + ":" + event_id. No signature in dev.
func (MockGateway) VerifyWebhook(_ context.Context, rawBody []byte, _ map[string]string) (WebhookEvent, error) {
	var b struct {
		OrderID     int64  `json:"order_id"`
		GatewayRef  string `json:"gateway_ref"`
		Status      string `json:"status"`
		EventID     string `json:"event_id"`
		AmountMinor int64  `json:"amount_minor"`
		Currency    string `json:"currency"`
	}
	if err := json.Unmarshal(rawBody, &b); err != nil {
		return WebhookEvent{}, fmt.Errorf("payments.MockGateway.VerifyWebhook: %w", err)
	}
	if b.GatewayRef == "" || b.EventID == "" {
		return WebhookEvent{}, ErrWebhookSignatureInvalid
	}
	st := WebhookFailed
	if b.Status == "succeeded" {
		st = WebhookSucceeded
	}
	return WebhookEvent{
		OrderID:        b.OrderID,
		GatewayRef:     b.GatewayRef,
		AmountMinor:    b.AmountMinor,
		Currency:       b.Currency,
		Status:         st,
		IdempotencyKey: b.GatewayRef + ":" + b.EventID,
		Raw:            rawBody,
	}, nil
}
