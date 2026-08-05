package payment_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/payment"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeWebhookService is a controllable ServiceInterface whose HandleWebhook
// returns whatever the test wired in. The other 4 methods are unused by the
// webhook handler path; they panic if called (a regression would surface fast).
type fakeWebhookService struct {
	err error
}

func (f *fakeWebhookService) CreateIntent(context.Context, string, int64, int64, string) (string, error) {
	panic("unused")
}
func (f *fakeWebhookService) CreateQuoteIntent(context.Context, string, int64, int64, string) (string, error) {
	panic("unused")
}
func (f *fakeWebhookService) HandleWebhook(context.Context, string, []byte, map[string]string) error {
	return f.err
}
func (f *fakeWebhookService) Refund(context.Context, int64, string) error      { panic("unused") }
func (f *fakeWebhookService) RefundQuote(context.Context, int64, string) error { panic("unused") }

// webhookApp builds a Fiber app with the two webhook routes wired to a
// fakeWebhookService.
func webhookApp(svc payment.ServiceInterface) *fiber.App {
	app := fiber.New()
	h := payment.NewHandler(svc)
	app.Post("/webhooks/airwallex", h.AirwallexWebhook)
	app.Post("/webhooks/paypal", h.PayPalWebhook)
	return app
}

// TestHandleWebhook_Success_Acks200: a verified+recorded event → 200.
func TestHandleWebhook_Success_Acks200(t *testing.T) {
	app := webhookApp(&fakeWebhookService{err: nil})
	req := httptest.NewRequest("POST", "/webhooks/airwallex", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// TestHandleWebhook_SignatureInvalid_Returns400: a bad signature → 400
// (terminal — the gateway sent garbage; no point retrying).
func TestHandleWebhook_SignatureInvalid_Returns400(t *testing.T) {
	app := webhookApp(&fakeWebhookService{err: models.ErrWebhookSignatureInvalid})
	req := httptest.NewRequest("POST", "/webhooks/paypal", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

// TestHandleWebhook_GatewayUnavailable_Returns503: an unconfigured gateway → 503.
func TestHandleWebhook_GatewayUnavailable_Returns503(t *testing.T) {
	app := webhookApp(&fakeWebhookService{err: models.ErrGatewayUnavailable})
	req := httptest.NewRequest("POST", "/webhooks/airwallex", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)
}

// TestHandleWebhook_InternalError_Returns500 is the priority fix: an internal
// error (DB write failure, PayPal verify-API outage, etc.) must return 500 so
// the gateway RETRIES — not 200 (which would ack the event as processed and
// silently lose it). The idempotency_key makes the retry safe (TDD §11).
func TestHandleWebhook_InternalError_Returns500(t *testing.T) {
	app := webhookApp(&fakeWebhookService{err: errors.New("db down")})
	req := httptest.NewRequest("POST", "/webhooks/paypal", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode, "internal error → 500 so the gateway retries")

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Webhook processing failed")
}

// TestHandleWebhook_BothRoutesExerciseSamePath: the Airwallex + PayPal
// handlers both delegate to handleWebhook; assert the 500 path holds for both.
func TestHandleWebhook_BothRoutes_InternalError_Returns500(t *testing.T) {
	app := webhookApp(&fakeWebhookService{err: errors.New("transient")})
	for _, path := range []string{"/webhooks/airwallex", "/webhooks/paypal"} {
		req := httptest.NewRequest("POST", path, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 500, resp.StatusCode, "%s → 500 on internal error", path)
	}
}
