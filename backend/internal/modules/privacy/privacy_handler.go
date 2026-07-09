package privacy

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler serves GDPR self-service endpoints (PRD §4.3): personal-data export
// and account erasure. All routes require a valid JWT (signed-in user only).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// ExportUserData: GET /profile/export?locale=en-US
//
// Returns the complete machine-readable personal-data package (JSON) for the
// authenticated user. Synchronous for MVP; will become an async job + emailed
// download link once order/itinerary history makes the payload large (M2/M3).
func (h *Handler) ExportUserData(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	locale := c.Query("locale", "")
	export, err := h.service.ExportUserData(c.Context(), userID, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "User not found"})
		}
		log.Printf("Handler.ExportUserData: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to export user data"})
	}
	// Content-Disposition nudges browsers into a "save as" download (the JSON
	// payload can be large once order history is included).
	c.Set("Content-Disposition", `attachment; filename="jingdezhen-data-export.json"`)
	return c.Status(fiber.StatusOK).JSON(export)
}

// DeleteAccount: POST /privacy/delete-account
//
// Performs irreversible GDPR erasure. The body must contain {"confirm":"DELETE"}
// to guard against accidental erasure. On success the user's JWT is no longer
// valid for re-login (the account is an anonymized stub); the client MUST
// discard the current token (note: existing unexpired JWTs remain technically
// valid until a server-side blocklist lands in the security pass — M4).
func (h *Handler) DeleteAccount(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.DeleteAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Confirmation required: send {\"confirm\":\"DELETE\"} to confirm irreversible account deletion"})
	}
	if err := h.service.DeleteAccount(c.Context(), userID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "User not found"})
		}
		if errors.Is(err, models.ErrAccountDeleted) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Account has already been deleted"})
		}
		log.Printf("Handler.DeleteAccount: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete account"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
