package payment

import (
	"errors"
	"log"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
)

// Handler handles payment webhook endpoints (PRD §3.2.3, TDD §5.2/§10).
type Handler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service}
}

// AirwallexWebhook: POST /webhooks/airwallex (public, signature-verified).
// Reads the raw body + headers, verifies via the gateway, acks 200 immediately
// (enqueue-and-ack per TDD §2.2). A signature mismatch → 400 (rejected).
//
// @Summary      Airwallex webhook
// @Description  Receives Airwallex payment events. Verifies the signature via
// @Description  the gateway, then acks 200 immediately (enqueue-and-ack). A
// @Description  signature mismatch returns 400. Idempotent on the idempotency_key.
// @Tags         webhooks,payments
// @Accept       json
// @Produce      json
// @Success      200 "OK (event acknowledged + enqueued)"
// @Failure      400 {object} models.ErrorResponse "Webhook signature verification failed"
// @Failure      503 {object} models.ErrorResponse "Payment gateway not configured"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /webhooks/airwallex [post]
func (h *Handler) AirwallexWebhook(c *fiber.Ctx) error {
	return h.handleWebhook(c, models.GatewayAirwallex)
}

// PayPalWebhook: POST /webhooks/paypal (public, signature-verified).
//
// @Summary      PayPal webhook
// @Description  Receives PayPal payment events. Verifies the signature via the
// @Description  gateway, then acks 200 immediately (enqueue-and-ack). A signature
// @Description  mismatch returns 400. Idempotent on the idempotency_key.
// @Tags         webhooks,payments
// @Accept       json
// @Produce      json
// @Success      200 "OK (event acknowledged + enqueued)"
// @Failure      400 {object} models.ErrorResponse "Webhook signature verification failed"
// @Failure      503 {object} models.ErrorResponse "Payment gateway not configured"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /webhooks/paypal [post]
func (h *Handler) PayPalWebhook(c *fiber.Ctx) error {
	return h.handleWebhook(c, models.GatewayPayPal)
}

func (h *Handler) handleWebhook(c *fiber.Ctx, gatewayName string) error {
	rawBody := c.BodyRaw()
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(k, v []byte) {
		headers[string(k)] = string(v)
	})
	if err := h.service.HandleWebhook(c.Context(), gatewayName, rawBody, headers); err != nil {
		switch {
		case errors.Is(err, models.ErrWebhookSignatureInvalid):
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Webhook signature verification failed"})
		case errors.Is(err, models.ErrGatewayUnavailable):
			return c.Status(fiber.StatusServiceUnavailable).JSON(models.ErrorResponse{Message: "Payment gateway not configured"})
		default:
			log.Printf("Handler.Webhook(%s): %v", gatewayName, err)
			// Still ack 200 so the gateway stops retrying for an internal error
			// (the event was not signature-invalid; we'll recover via the
			// idempotency_key on a retry). TDD §2.2 enqueue-and-ack.
			return c.SendStatus(fiber.StatusOK)
		}
	}
	return c.SendStatus(fiber.StatusOK)
}
