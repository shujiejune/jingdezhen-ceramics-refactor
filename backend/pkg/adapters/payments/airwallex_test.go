package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAirwallexTestServer returns a stub Airwallex API + a gateway pointed at it.
func newAirwallexTestServer(t *testing.T, webhookSecret string) (*httptest.Server, *AirwallexGateway) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/authentication/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
	})
	mux.HandleFunc("/pa/payment_intents/create", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "int_abc",
			"next_action": map[string]string{"url": "https://checkout.example/pay/int_abc"},
		})
	})
	mux.HandleFunc("/pa/refunds/create", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "rfa_xyz"})
	})
	srv := httptest.NewServer(mux)
	g := NewAirwallexGateway("cid", "apikey", "sandbox", webhookSecret)
	g.baseURL = srv.URL
	g.http = srv.Client()
	return srv, g
}

func TestAirwallexCreateIntent(t *testing.T) {
	srv, g := newAirwallexTestServer(t, "secret")
	defer srv.Close()
	resp, err := g.CreateIntent(context.Background(), IntentRequest{OrderID: 42, AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "int_abc", resp.GatewayRef)
	assert.Equal(t, "https://checkout.example/pay/int_abc", resp.HostedURL)
}

func TestAirwallexRefund(t *testing.T) {
	srv, g := newAirwallexTestServer(t, "secret")
	defer srv.Close()
	resp, err := g.Refund(context.Background(), RefundRequest{GatewayRef: "int_abc", AmountMinor: 53550, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, "rfa_xyz", resp.RefundRef)
}

func TestAirwallexVerifyWebhookValid(t *testing.T) {
	srv, g := newAirwallexTestServer(t, "secret")
	defer srv.Close()
	body := []byte(`{"name":"payment_intent.succeeded","id":"evt_1","data":{"object":{"id":"int_abc","merchant_order_id":"42","amount":535.50,"currency":"USD"}}}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	ev, err := g.VerifyWebhook(context.Background(), body, map[string]string{"x-airwallex-signature": sig})
	require.NoError(t, err)
	assert.Equal(t, int64(42), ev.OrderID)
	assert.Equal(t, "int_abc", ev.GatewayRef)
	assert.Equal(t, WebhookSucceeded, ev.Status)
	assert.Equal(t, "int_abc:evt_1", ev.IdempotencyKey)
}

func TestAirwallexVerifyWebhookTampered(t *testing.T) {
	srv, g := newAirwallexTestServer(t, "secret")
	defer srv.Close()
	body := []byte(`{"name":"payment_intent.succeeded"}`)
	_, err := g.VerifyWebhook(context.Background(), body, map[string]string{"x-airwallex-signature": "deadbeef"})
	assert.ErrorIs(t, err, ErrWebhookSignatureInvalid)
}

func TestAirwallexVerifyWebhookMissingSecret(t *testing.T) {
	srv, g := newAirwallexTestServer(t, "")
	defer srv.Close()
	body := []byte(`{}`)
	_, err := g.VerifyWebhook(context.Background(), body, map[string]string{"x-airwallex-signature": "x"})
	assert.ErrorIs(t, err, ErrWebhookSignatureInvalid)
}
