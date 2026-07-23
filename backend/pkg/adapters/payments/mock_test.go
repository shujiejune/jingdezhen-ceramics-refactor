package payments

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockGatewayCreateIntent(t *testing.T) {
	g := NewMockGateway()
	resp, err := g.CreateIntent(context.Background(), IntentRequest{OrderID: 42, AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "mock-42", resp.GatewayRef)
	assert.Equal(t, "mock://pay/42", resp.HostedURL)
}

func TestMockGatewayRefund(t *testing.T) {
	g := NewMockGateway()
	resp, err := g.Refund(context.Background(), RefundRequest{GatewayRef: "mock-42", AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "mock-refund-mock-42", resp.RefundRef)
}

func TestMockGatewayVerifyWebhook(t *testing.T) {
	g := NewMockGateway()
	body, _ := json.Marshal(map[string]any{
		"order_id": 42, "gateway_ref": "mock-42", "status": "succeeded",
		"event_id": "evt-1", "amount_minor": 53550, "currency": "USD",
	})
	ev, err := g.VerifyWebhook(context.Background(), body, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(42), ev.OrderID)
	assert.Equal(t, "mock-42", ev.GatewayRef)
	assert.Equal(t, WebhookSucceeded, ev.Status)
	assert.Equal(t, "mock-42:evt-1", ev.IdempotencyKey)
	assert.Equal(t, int64(53550), ev.AmountMinor)
}

func TestMockGatewayVerifyWebhookInvalid(t *testing.T) {
	g := NewMockGateway()
	_, err := g.VerifyWebhook(context.Background(), []byte("{bad json"), nil)
	assert.Error(t, err)
	// Missing gateway_ref/event_id → treated as an unverified (invalid) webhook.
	body, _ := json.Marshal(map[string]any{"order_id": 42, "status": "succeeded"})
	_, err = g.VerifyWebhook(context.Background(), body, nil)
	assert.Error(t, err)
}
