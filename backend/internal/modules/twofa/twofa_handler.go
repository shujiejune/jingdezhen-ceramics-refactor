package twofa

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"

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

// Enroll: POST /profile/2fa/enroll — start TOTP enrollment (staff only).
// Returns the otpauth:// URI (QR) and the raw secret (shown once).
func (h *Handler) Enroll(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	var req models.EnrollTwoFARequest
	if err := c.BodyParser(&req); err != nil {
		// Body is optional (defaults applied); a parse error on an empty body
		// is tolerable — proceed with defaults.
		req = models.EnrollTwoFARequest{}
	}
	resp, err := h.service.Enroll(c.Context(), userID, req)
	if err != nil {
		log.Printf("Handler.Enroll2FA: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to start 2FA enrollment"})
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Confirm: POST /profile/2fa/confirm — verify the first code and enable 2FA.
func (h *Handler) Confirm(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	var req models.ConfirmTwoFARequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	backupCodes, err := h.service.Confirm(c.Context(), userID, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "No pending 2FA enrollment found — call /2fa/enroll first"})
		}
		if errors.Is(err, models.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid TOTP code"})
		}
		log.Printf("Handler.Confirm2FA: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to confirm 2FA"})
	}
	// Backup codes are shown ONCE here. Instruct the user to save them; they are
	// the recovery path if the authenticator is lost and are never returned again.
	return c.Status(fiber.StatusOK).JSON(models.TwoFAConfirmResponse{
		Message:     "2FA enabled — save your backup codes now (shown once)",
		BackupCodes: backupCodes,
	})
}

// Disable: DELETE /profile/2fa — turn 2FA off (keeps the staged secret).
func (h *Handler) Disable(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.Disable(c.Context(), userID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "2FA is not enabled"})
		}
		log.Printf("Handler.Disable2FA: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to disable 2FA"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "2FA disabled"})
}

// RegenerateBackupCodes: POST /profile/2fa/backup-codes/regenerate — invalidate
// remaining unused codes and issue a fresh set. Shown ONCE. Protected by JWT.
func (h *Handler) RegenerateBackupCodes(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	codes, err := h.service.RegenerateBackupCodes(c.Context(), userID)
	if err != nil {
		if errors.Is(err, models.ErrInvalidOperation) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "2FA is not enabled"})
		}
		log.Printf("Handler.RegenerateBackupCodes: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to regenerate backup codes"})
	}
	return c.Status(fiber.StatusOK).JSON(models.TwoFAConfirmResponse{
		Message:     "Backup codes regenerated — save them now (shown once; old unused codes are invalidated)",
		BackupCodes: codes,
	})
}

// BackupCodesRemaining: GET /profile/2fa/backup-codes — how many unused codes
// remain (NOT the codes themselves; those are shown once at generate time).
func (h *Handler) BackupCodesRemaining(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	n, err := h.service.CountUnusedBackupCodes(c.Context(), userID)
	if err != nil {
		log.Printf("Handler.BackupCodesRemaining: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to count backup codes"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"remaining": n})
}
