package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPayPalTestServer(t *testing.T) (*httptest.Server, *PayPalGateway) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "cid", u)
		assert.Equal(t, "secret", p)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/v2/checkout/orders", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "ORDER123",
			"links": []map[string]string{
				{"href": "https://paypal.example/approve", "rel": "approve"},
			},
		})
	})
	mux.HandleFunc("/v2/checkout/orders/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"purchase_units": []map[string]any{{
				"payments": map[string]any{"captures": []map[string]string{{"id": "CAP1"}}},
			}},
		})
	})
	mux.HandleFunc("/v2/payments/captures/CAP1/refund", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "REF1"})
	})
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"verification_status": "SUCCESS"})
	})
	srv := httptest.NewServer(mux)
	g := NewPayPalGateway("cid", "secret", "sandbox", "whid")
	g.baseURL = srv.URL
	g.http = srv.Client()
	return srv, g
}

func TestPayPalCreateIntent(t *testing.T) {
	srv, g := newPayPalTestServer(t)
	defer srv.Close()
	resp, err := g.CreateIntent(context.Background(), IntentRequest{OrderID: 42, AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "ORDER123", resp.GatewayRef)
	assert.Equal(t, "https://paypal.example/approve", resp.HostedURL)
}

func TestPayPalRefund(t *testing.T) {
	srv, g := newPayPalTestServer(t)
	defer srv.Close()
	resp, err := g.Refund(context.Background(), RefundRequest{GatewayRef: "ORDER123", AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "REF1", resp.RefundRef)
}

func TestPayPalVerifyWebhookValid(t *testing.T) {
	srv, g := newPayPalTestServer(t)
	defer srv.Close()
	body := []byte(`{"event_type":"CHECKOUT.ORDER.APPROVED","id":"WH-abc","resource":{"id":"ORDER123","supplementary_data":{"related_ids":{"order_id":"ORDER123"}},"amount":{"currency_code":"USD","value":"535.50"}}}`)
	ev, err := g.VerifyWebhook(context.Background(), body, map[string]string{
		"PAYPAL-AUTH-ALGO":         "RS256",
		"PAYPAL-CERT-URL":          "https://example/cert",
		"PAYPAL-TRANSMISSION-ID":   "tid",
		"PAYPAL-TRANSMISSION-SIG":  "sig",
		"PAYPAL-TRANSMISSION-TIME": "2026-07-23T00:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, WebhookSucceeded, ev.Status)
	assert.Equal(t, "WH-abc", ev.IdempotencyKey)
	assert.Equal(t, int64(53550), ev.AmountMinor)
}

func TestPayPalVerifyWebhookInvalid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/v1/notifications/verify-webhook-signature", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"verification_status": "FAILURE"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	g := NewPayPalGateway("cid", "secret", "sandbox", "whid")
	g.baseURL = srv.URL
	g.http = srv.Client()
	_, err := g.VerifyWebhook(context.Background(), []byte(`{}`), map[string]string{})
	assert.ErrorIs(t, err, ErrWebhookSignatureInvalid)
}
