package twofa

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

// Enroll: POST /profile/2fa/enroll — start TOTP enrollment (staff only).
// Returns the otpauth:// URI (QR) and the raw secret (shown once).
//
//	@Summary		Start 2FA enrollment
//	@Description	Begins TOTP enrollment. Returns the otpauth:// URI (QR) + raw secret (shown once).
//	@Description	Confirm via /profile/2fa/confirm with the first TOTP code.
//	@Tags			profile,2fa
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string						true	"Bearer <access_token>"
//	@Param			body			body		models.EnrollTwoFARequest	false	"Optional enrollment params (defaults applied)"
//	@Success		200				{object}	models.TwoFAEnrollResponse
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/profile/2fa/enroll [post]
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
//
//	@Summary		Confirm 2FA enrollment
//	@Description	Verifies the first TOTP code, enables 2FA, and returns backup codes (shown once).
//	@Tags			profile,2fa
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string						true	"Bearer <access_token>"
//	@Param			body			body		models.ConfirmTwoFARequest	true	"6-digit TOTP code"
//	@Success		200				{object}	models.TwoFAConfirmResponse
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required / invalid TOTP code"
//	@Failure		404				{object}	models.ErrorResponse	"No pending 2FA enrollment found — call /2fa/enroll first"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/profile/2fa/confirm [post]
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
//
//	@Summary		Disable 2FA
//	@Description	Turns 2FA off (keeps the staged secret for re-enrollment).
//	@Tags			profile,2fa
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true				"Bearer <access_token>"
//	@Success		200				{object}	object					"{message: \"2FA	disabled\"}"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		404				{object}	models.ErrorResponse	"2FA is not enabled"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/profile/2fa [delete]
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
//
//	@Summary		Regenerate backup codes
//	@Description	Invalidates remaining unused backup codes + issues a fresh set (shown once).
//	@Tags			profile,2fa
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Success		200				{object}	models.TwoFAConfirmResponse
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		409				{object}	models.ErrorResponse	"2FA is not enabled"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/profile/2fa/backup-codes/regenerate [post]
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
//
//	@Summary		Count remaining backup codes
//	@Description	Returns how many unused backup codes remain (not the codes themselves).
//	@Tags			profile,2fa
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Success		200				{object}	object					"{remaining: int}"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/profile/2fa/backup-codes [get]
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
