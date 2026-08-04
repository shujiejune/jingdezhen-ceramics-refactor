package consent

import (
	"errors"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

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

// RecordConsent: POST /consent
// Public (no auth required) so anonymous visitors can record cookie consent
// before signing up. If a JWT is present, the user is linked; otherwise the
// record is stored anonymously (by IP hash) and back-linked later.
func (h *Handler) RecordConsent(c *fiber.Ctx) error {
	var req models.RecordConsentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	// Link to the authenticated user if a valid JWT is present; otherwise
	// record anonymously (nil user_id). We deliberately do not 401 here —
	// cookie consent must be recordable before login.
	var userID *string
	if id, err := utils.GetUserIDFromContext(c); err == nil && id != "" {
		userID = &id
	}

	rec, err := h.service.RecordConsent(c.Context(), userID, c.IP(), req)
	if err != nil {
		log.Printf("Handler.RecordConsent: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to record consent"})
	}
	return c.Status(fiber.StatusCreated).JSON(rec)
}

// GetConsentState: GET /profile/consent/:kind
// Authenticated. Returns the latest consent record for the given kind, or
// {granted: false} if no record exists yet.
func (h *Handler) GetConsentState(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	kind := models.ConsentKind(c.Params("kind"))
	// Light validation without importing the validator for a path param.
	switch kind {
	case models.ConsentKindPrivacyPolicy, models.ConsentKindToS,
		models.ConsentKindCookieAnalytics, models.ConsentKindCookieMarketing:
	default:
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid consent kind"})
	}

	rec, err := h.service.GetConsentState(c.Context(), userID, kind)
	if err != nil {
		log.Printf("Handler.GetConsentState: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to get consent state"})
	}
	if rec == nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"kind": kind, "granted": false, "recorded": false})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"kind":        rec.Kind,
		"granted":     rec.Granted,
		"recorded":    true,
		"doc_version": rec.DocVersion,
		"created_at":  rec.CreatedAt,
	})
}

// ListConsentHistory: GET /profile/consent
// Authenticated. Returns the full consent history for the requesting user
// (GDPR data export).
func (h *Handler) ListConsentHistory(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	history, err := h.service.ListUserConsentHistory(c.Context(), userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": []models.ConsentRecord{}})
		}
		log.Printf("Handler.ListConsentHistory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list consent history"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": history})
}
