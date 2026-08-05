package analytics

import (
	"errors"
	"log"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// RecordEvent: POST /analytics/events — public (no auth) + consent-gated.
//
// The endpoint is intentionally public: pageview/event tracking fires before
// login (and for anonymous visitors). Consent (cookie_analytics) is checked
// server-side from the visitor's IP-hash consent record; if not granted the
// request is acknowledged with 204 No Content and the event is dropped — the
// client should not retry, this is not an error.
//
// @Summary      Record an analytics event
// @Description  Records a pseudonymous analytics event (pageview or named event).
// @Description  Public + consent-gated: the visitor must have granted cookie_analytics
// @Description  consent (checked by IP hash). Not-consented → 204, event dropped.
// @Description  Country is resolved server-side from GeoLite2 (no raw IP stored).
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        body body models.AnalyticsEventRequest true "Event (kind, path; name for events)"
// @Success      201 {object} object "{id: <int64>}"
// @Success      204 "Consent not granted — event silently dropped (not an error)"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /analytics/events [post]
func (h *Handler) RecordEvent(c *fiber.Ctx) error {
	var req models.AnalyticsEventRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	ev, err := h.service.Record(c.Context(), c.IP(), c.Get("User-Agent"), req)
	if err != nil {
		if errors.Is(err, models.ErrConsentNotGranted) {
			// Consent not granted: silently drop. 204, no body — not a client error.
			return c.SendStatus(fiber.StatusNoContent)
		}
		log.Printf("Handler.RecordEvent: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to record event"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": ev.ID})
}
