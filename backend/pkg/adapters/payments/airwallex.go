package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// AirwallexGateway implements Gateway against the Airwallex Payment Intents API
// (PRD §3.2.3). Hosted-checkout: CreateIntent returns the intent's hosted
// checkout URL; the customer pays off-site; the webhook (HMAC-SHA256 signed)
// drives the order created→paid. Refund via the refunds endpoint.
//
// Sandbox base: https://api-demo.airwallex.com/api/v1 (AIRWALLEX_ENV=sandbox).
// Live base:    https://api.airwallex.com/api/v1      (AIRWALLEX_ENV=live).
//
// Auth: ClientID + APIKey → POST /authentication/login → access token (bearer
// for subsequent calls). The token is short-lived; this client fetches one per
// request (acceptable for MVP volume; cache later if needed).
type AirwallexGateway struct {
	clientID      string
	apiKey        string
	webhookSecret string
	baseURL       string
	http          *http.Client
}

func NewAirwallexGateway(clientID, apiKey, env, webhookSecret string) *AirwallexGateway {
	base := "https://api.airwallex.com/api/v1"
	if env == "sandbox" {
		base = "https://api-demo.airwallex.com/api/v1"
	}
	return &AirwallexGateway{
		clientID:      clientID,
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		baseURL:       base,
		http:          &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *AirwallexGateway) Name() string { return "airwallex" }

func (g *AirwallexGateway) CreateIntent(ctx context.Context, req IntentRequest) (IntentResponse, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return IntentResponse{}, fmt.Errorf("airwallex.CreateIntent.token: %w", err)
	}
	body := map[string]any{
		"request_id":        "jdz-" + strconv.FormatInt(req.OrderID, 10),
		"amount":            float64(req.AmountMinor) / 100.0,
		"currency":          req.Currency,
		"merchant_order_id": strconv.FormatInt(req.OrderID, 10),
		"return_url":        req.ReturnURL,
		// Hosted page (Airwallex Drop-in / PaymentIntent with auto_capture).
		"payment_method_options": map[string]any{
			"card": map[string]any{"auto_capture": true},
		},
	}
	raw, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/pa/payment_intents/create", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("x-client-id", g.clientID)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return IntentResponse{}, fmt.Errorf("airwallex.CreateIntent.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return IntentResponse{}, fmt.Errorf("airwallex.CreateIntent.status: %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		ID string `json:"id"`
		// Hosted checkout URL. Airwallex returns the next-action URL for hosted
		// checkout; we expose it as HostedURL so the client redirects.
		NextAction struct {
			URL string `json:"url"`
		} `json:"next_action"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return IntentResponse{}, fmt.Errorf("airwallex.CreateIntent.parse: %w", err)
	}
	hosted := out.NextAction.URL
	if hosted == "" {
		// Fallback: some flows return the URL at a different key; the client may
		// also render the Drop-in from the intent id directly.
		hosted = "https://checkout.airwallex.com/#/payments/" + out.ID
	}
	return IntentResponse{GatewayRef: out.ID, HostedURL: hosted}, nil
}

func (g *AirwallexGateway) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("airwallex.Refund.token: %w", err)
	}
	body := map[string]any{
		"request_id":        "jdz-refund-" + req.GatewayRef,
		"amount":            float64(req.AmountMinor) / 100.0,
		"reason":            req.Reason,
		"payment_intent_id": req.GatewayRef,
	}
	raw, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/pa/refunds/create", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("x-client-id", g.clientID)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("airwallex.Refund.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return RefundResponse{}, fmt.Errorf("airwallex.Refund.status: %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &out)
	return RefundResponse{RefundRef: out.ID}, nil
}

// VerifyWebhook verifies the Airwallex HMAC-SHA256 signature header
// (x-airwallex-signature) over the raw body. On mismatch →
// ErrWebhookSignatureInvalid (security boundary). The body is the raw webhook
// JSON (Airwallex sends the payment intent event with merchant_order_id).
func (g *AirwallexGateway) VerifyWebhook(ctx context.Context, rawBody []byte, headers map[string]string) (WebhookEvent, error) {
	sig := headers["X-Airwallex-Signature"]
	if sig == "" {
		sig = headers["x-airwallex-signature"]
	}
	if !g.verifySig(rawBody, sig) {
		return WebhookEvent{}, ErrWebhookSignatureInvalid
	}
	// Parse the Airwallex event. The data.object carries the payment intent.
	var ev struct {
		Name string `json:"name"` // e.g. "payment_intent.succeeded"
		Data struct {
			Object struct {
				ID              string  `json:"id"`
				MerchantOrderID string  `json:"merchant_order_id"`
				Amount          float64 `json:"amount"`
				Currency        string  `json:"currency"`
			} `json:"object"`
		} `json:"data"`
		ID string `json:"id"` // event id
	}
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		return WebhookEvent{}, fmt.Errorf("airwallex.VerifyWebhook.parse: %w", err)
	}
	orderID, _ := strconv.ParseInt(ev.Data.Object.MerchantOrderID, 10, 64)
	st := WebhookFailed
	if ev.Name == "payment_intent.succeeded" {
		st = WebhookSucceeded
	}
	return WebhookEvent{
		OrderID:        orderID,
		GatewayRef:     ev.Data.Object.ID,
		AmountMinor:    int64(ev.Data.Object.Amount * 100),
		Currency:       ev.Data.Object.Currency,
		Status:         st,
		IdempotencyKey: ev.Data.Object.ID + ":" + ev.ID,
		Raw:            rawBody,
	}, nil
}

func (g *AirwallexGateway) verifySig(body []byte, sig string) bool {
	if g.webhookSecret == "" || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	// Constant-time compare.
	return hmac.Equal([]byte(want), []byte(sig))
}

func (g *AirwallexGateway) accessToken(ctx context.Context) (string, error) {
	body := map[string]string{"api_key": g.apiKey}
	raw, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/authentication/login", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", g.clientID)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("login status %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return out.Token, nil
}
