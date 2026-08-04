package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PayPalGateway implements Gateway against the PayPal Orders v2 API (PRD §3.2.3).
// Hosted-checkout: CreateIntent creates an order + returns the PayPal approve
// link; the customer pays at PayPal; the webhook (verified via PayPal's
// verify-webhook-signature API) drives the order created→paid.
//
// Sandbox base: https://api-m.sandbox.paypal.com (PAYPAL_ENV=sandbox).
// Live base:    https://api-m.paypal.com           (PAYPAL_ENV=live).
//
// Auth: client-credentials → bearer access token (cached until expiry).
type PayPalGateway struct {
	clientID     string
	clientSecret string
	webhookID    string
	baseURL      string
	http         *http.Client
	tokenMu      sync.Mutex
	token        string
	tokenExp     time.Time
}

func NewPayPalGateway(clientID, clientSecret, env, webhookID string) *PayPalGateway {
	base := "https://api-m.paypal.com"
	if env == "sandbox" {
		base = "https://api-m.sandbox.paypal.com"
	}
	return &PayPalGateway{
		clientID: clientID, clientSecret: clientSecret, webhookID: webhookID,
		baseURL: base, http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *PayPalGateway) Name() string { return "paypal" }

func (g *PayPalGateway) CreateIntent(ctx context.Context, req IntentRequest) (IntentResponse, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return IntentResponse{}, fmt.Errorf("paypal.CreateIntent.token: %w", err)
	}
	body := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{
			"reference_id": fmt.Sprintf("%d", req.OrderID),
			"amount": map[string]string{
				"currency_code": req.Currency,
				"value":         fmt.Sprintf("%.2f", float64(req.AmountMinor)/100.0),
			},
		}},
	}
	raw, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v2/checkout/orders", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return IntentResponse{}, fmt.Errorf("paypal.CreateIntent.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return IntentResponse{}, fmt.Errorf("paypal.CreateIntent.status: %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		ID    string `json:"id"`
		Links []struct {
			HRef string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return IntentResponse{}, fmt.Errorf("paypal.CreateIntent.parse: %w", err)
	}
	approve := ""
	for _, l := range out.Links {
		if l.Rel == "approve" {
			approve = l.HRef
			break
		}
	}
	return IntentResponse{GatewayRef: out.ID, HostedURL: approve}, nil
}

func (g *PayPalGateway) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("paypal.Refund.token: %w", err)
	}
	body := map[string]any{
		"amount": map[string]string{
			"currency_code": req.Currency,
			"value":         fmt.Sprintf("%.2f", float64(req.AmountMinor)/100.0),
		},
		"note_to_payer": req.Reason,
	}
	raw, _ := json.Marshal(body)
	// Refund against the capture (req.GatewayRef is the PayPal order id; the
	// capture id is fetched first).
	captureID, err := g.captureIDForOrder(ctx, token, req.GatewayRef)
	if err != nil {
		return RefundResponse{}, err
	}
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/v2/payments/captures/"+captureID+"/refund", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("paypal.Refund.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return RefundResponse{}, fmt.Errorf("paypal.Refund.status: %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &out)
	return RefundResponse{RefundRef: out.ID}, nil
}

// VerifyWebhook uses PayPal's verify-webhook-signature API (recommended over
// manual cert verification). On mismatch → ErrWebhookSignatureInvalid.
func (g *PayPalGateway) VerifyWebhook(ctx context.Context, rawBody []byte, headers map[string]string) (WebhookEvent, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("paypal.VerifyWebhook.token: %w", err)
	}
	verifyBody := map[string]any{
		"auth_algo":         headers["PAYPAL-AUTH-ALGO"],
		"cert_url":          headers["PAYPAL-CERT-URL"],
		"transmission_id":   headers["PAYPAL-TRANSMISSION-ID"],
		"transmission_sig":  headers["PAYPAL-TRANSMISSION-SIG"],
		"transmission_time": headers["PAYPAL-TRANSMISSION-TIME"],
		"webhook_id":        g.webhookID,
		"webhook_event":     json.RawMessage(rawBody),
	}
	raw, _ := json.Marshal(verifyBody)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v1/notifications/verify-webhook-signature", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("paypal.VerifyWebhook.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return WebhookEvent{}, fmt.Errorf("paypal.VerifyWebhook.status: %d: %s", resp.StatusCode, string(respBody))
	}
	var vr struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.Unmarshal(respBody, &vr); err != nil {
		return WebhookEvent{}, fmt.Errorf("paypal.VerifyWebhook.parse: %w", err)
	}
	if strings.ToUpper(vr.VerificationStatus) != "SUCCESS" {
		return WebhookEvent{}, ErrWebhookSignatureInvalid
	}
	// Parse the event.
	var ev struct {
		EventType string `json:"event_type"`
		ID        string `json:"id"` // webhook event id
		Resource  struct {
			ID                string `json:"id"` // the order/capture id
			SupplementaryData struct {
				RelatedIDs struct {
					OrderID string `json:"order_id"`
				} `json:"related_ids"`
			} `json:"supplementary_data"`
			Amount struct {
				CurrencyCode string `json:"currency_code"`
				Value        string `json:"value"`
			} `json:"amount"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		return WebhookEvent{}, fmt.Errorf("paypal.VerifyWebhook.eventParse: %w", err)
	}
	// OrderID lives in supplementary_data.related_ids.order_id (capture events)
	// or in resource.id (checkout-order-approved events). Parse best-effort.
	orderRef := ev.Resource.SupplementaryData.RelatedIDs.OrderID
	if orderRef == "" {
		orderRef = ev.Resource.ID
	}
	orderID := parsePayPalOrderRef(orderRef)
	st := WebhookFailed
	if ev.EventType == "CHECKOUT.ORDER.APPROVED" || ev.EventType == "PAYMENT.CAPTURE.COMPLETED" {
		st = WebhookSucceeded
	}
	return WebhookEvent{
		OrderID:        orderID,
		GatewayRef:     orderRef,
		AmountMinor:    parsePayPalAmount(ev.Resource.Amount.Value),
		Currency:       ev.Resource.Amount.CurrencyCode,
		Status:         st,
		IdempotencyKey: ev.ID, // PayPal webhook event id is unique
		Raw:            rawBody,
	}, nil
}

func (g *PayPalGateway) accessToken(ctx context.Context) (string, error) {
	g.tokenMu.Lock()
	defer g.tokenMu.Unlock()
	if g.token != "" && time.Now().Before(g.tokenExp.Add(-30*time.Second)) {
		return g.token, nil
	}
	body := strings.NewReader("grant_type=client_credentials")
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/v1/oauth2/token", body)
	httpReq.SetBasicAuth(g.clientID, g.clientSecret)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("paypal token status %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	g.token = out.AccessToken
	g.tokenExp = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	return g.token, nil
}

func (g *PayPalGateway) captureIDForOrder(ctx context.Context, token, orderID string) (string, error) {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/v2/checkout/orders/"+orderID, nil)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("paypal.captureID.http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("paypal.captureID.status %d: %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					ID string `json:"id"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	for _, pu := range out.PurchaseUnits {
		for _, c := range pu.Payments.Captures {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("paypal: no capture for order %s", orderID)
}

// parsePayPalOrderRef extracts the platform order id from a PayPal reference.
// We set purchase_units[].reference_id to the order id at CreateIntent.
func parsePayPalOrderRef(ref string) int64 {
	if ref == "" {
		return 0
	}
	// PayPal order ids are not numeric; the reference_id is the order id.
	// We map back via the reference_id stored on the purchase unit — for the
	// capture event we read related_ids.order_id (the PayPal order id), then
	// look up our payments row by gateway_ref. OrderID here is best-effort.
	// The payment service re-derives the order id from the payments row.
	var n int64
	_, err := fmt.Sscanf(ref, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

// parsePayPalAmount converts a PayPal amount string ("12.34") to minor units.
func parsePayPalAmount(value string) int64 {
	var major float64
	_, err := fmt.Sscanf(value, "%f", &major)
	if err != nil {
		return 0
	}
	return int64(major * 100)
}
