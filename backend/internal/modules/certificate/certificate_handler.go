package certificate

import (
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
)

// Handler handles certificate endpoints (PRD §3.2.1).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

func requestLocale(c *fiber.Ctx) string {
	loc := c.Query("locale")
	if loc != "" {
		return loc
	}
	accept := c.Get("Accept-Language")
	if accept != "" {
		for i := 0; i < len(accept); i++ {
			if accept[i] == ',' || accept[i] == ';' || accept[i] == ' ' {
				return accept[:i]
			}
		}
		return accept
	}
	return ""
}

// GetByCode: GET /certificates/:code?locale= (public QR target, no auth — PRD §3.2.1)
func (h *Handler) GetByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Certificate code is required"})
	}
	cert, err := h.service.GetByCode(c.Context(), code, requestLocale(c))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Certificate not found"})
		}
		log.Printf("Handler.GetByCode: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve certificate"})
	}
	return c.Status(fiber.StatusOK).JSON(cert)
}

// QRCode: GET /certificates/:code/qr (public, returns image/png)
// Renders a QR encoding the public certificate URL on-demand (no OSS storage
// needed; qr_key is populated when the storage adapter lands).
func (h *Handler) QRCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Certificate code is required"})
	}
	// Verify the certificate exists (404 for an unknown code, not a bogus QR).
	if _, err := h.service.GetByCode(c.Context(), code, "en-US"); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Certificate not found"})
		}
		log.Printf("Handler.QRCode: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to generate QR"})
	}
	// The QR encodes the public certificate page URL. CLIENT_ORIGIN is the site
	// origin (configurable); the path resolves to the public cert page.
	target := fmt.Sprintf("%s/certificates/%s", c.Get("X-Client-Origin", ""), code)
	png, err := qrcode.Encode(target, qrcode.Medium, 256)
	if err != nil {
		log.Printf("Handler.QRCode.Encode(%s): %v", code, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to generate QR"})
	}
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "public, max-age=86400") // QR is stable per code
	return c.Send(png) //nolint:errcheck
}

// --- Admin ---

func (h *Handler) ListCertificates(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	certs, total, err := h.service.ListAdmin(c.Context(), page, limit)
	if err != nil {
		log.Printf("Handler.ListCertificates: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list certificates"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(certs, page, limit, total))
}

func (h *Handler) GetCertificate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid certificate ID"})
	}
	cert, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Certificate not found"})
		}
		log.Printf("Handler.GetCertificate: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve certificate"})
	}
	return c.Status(fiber.StatusOK).JSON(cert)
}

// Regenerate: POST /admin/certificates/:id/regenerate (PRD: operators can regenerate)
func (h *Handler) Regenerate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid certificate ID"})
	}
	newCode, err := h.service.Regenerate(c.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Certificate not found"})
		}
		log.Printf("Handler.Regenerate: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to regenerate certificate"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"cert_code": newCode})
}
