package certificate

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
)

// Handler handles certificate endpoints (PRD §3.2.1).
type Handler struct {
	service  ServiceInterface
	store    storage.Store // resolves pdf_key → public URL (local path or CDN)
	validate *validator.Validate
}

func NewHandler(service ServiceInterface, store storage.Store) *Handler {
	return &Handler{service: service, store: store, validate: validator.New()}
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
//
//	@Summary		Get a certificate by code (public)
//	@Description	Public authenticity-certificate lookup by its JDZ-<6-base32> code
//	@Description	(the QR target). Returns the cert + product/artist display info +
//	@Description	the provenance chain. No auth.
//	@Tags			certificates
//	@Accept			json
//	@Produce		json
//	@Param			code	path		string	true	"Certificate code (JDZ-<6-base32>)"
//	@Param			locale	query		string	false	"BCP 47 locale (e.g. en-US). Overrides Accept-Language."	default("en-US")
//	@Success		200		{object}	models.Certificate
//	@Failure		400		{object}	models.ErrorResponse	"Missing code"
//	@Failure		404		{object}	models.ErrorResponse	"Certificate not found"
//	@Failure		500		{object}	models.ErrorResponse	"Internal error"
//	@Router			/certificates/{code} [get]
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
//
//	@Summary		Get a certificate's QR code (public)
//	@Description	Renders a QR PNG encoding the public certificate URL on-demand.
//	@Description	Returns image/png (cached 24h; QR is stable per code). No auth.
//	@Tags			certificates
//	@Produce		png
//	@Param			code	path		string					true	"Certificate code (JDZ-<6-base32>)"
//	@Success		200		{file}		binary					"QR PNG image"
//	@Failure		400		{object}	models.ErrorResponse	"Missing code"
//	@Failure		404		{object}	models.ErrorResponse	"Certificate not found"
//	@Failure		500		{object}	models.ErrorResponse	"Internal error"
//	@Router			/certificates/{code}/qr [get]
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
	return c.Send(png)                              //nolint:errcheck
}

// --- Admin ---

// PDFDownload: GET /certificates/:code/pdf (public, no auth — PRD §3.2.1).
// Serves the pre-rendered certificate PDF. The PDF is generated asynchronously
// (pdf:generate job at issue/regenerate) + stored via the storage adapter; this
// endpoint resolves the stored pdf_key to its public URL (CDN or local path).
// ?download=1 forces a Content-Disposition: attachment (otherwise inline).
//
// 404 + a clear message when pdf_key is NULL: in local mode (NoopGenerator)
// no PDF is ever generated, so the endpoint 404s gracefully; the QR + JSON
// cert endpoints still work. A planner can trigger regeneration in chromedp
// mode to (re)render.
//
//	@Summary		Download a certificate PDF (public)
//	@Description	Serves the pre-rendered certificate PDF (302 redirect to the
//	@Description	storage/CDN URL). ?download=1 forces an attachment; otherwise inline.
//	@Description	404 when the PDF has not yet been generated (local mode / pre-render).
//	@Description	No auth.
//	@Tags			certificates
//	@Produce		json
//	@Param			code		path		string					true	"Certificate code (JDZ-<6-base32>)"
//	@Param			download	query		int						false	"1 = force Content-Disposition: attachment"
//	@Success		302			{string}	string					"Redirect to the PDF storage URL"
//	@Failure		400			{object}	models.ErrorResponse	"Missing code"
//	@Failure		404			{object}	models.ErrorResponse	"Certificate not found / PDF not yet generated"
//	@Failure		500			{object}	models.ErrorResponse	"Internal error"
//	@Router			/certificates/{code}/pdf [get]
func (h *Handler) PDFDownload(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Certificate code is required"})
	}
	cert, err := h.service.GetByCode(c.Context(), code, "en-US")
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Certificate not found"})
		}
		log.Printf("Handler.PDFDownload: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve certificate"})
	}
	if cert.PDFKey == nil || *cert.PDFKey == "" {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "PDF not yet generated"})
	}
	url := h.store.PublicURL(*cert.PDFKey)
	if c.Query("download") == "1" {
		c.Set("Content-Disposition", `attachment; filename="JDZ-`+code+`.pdf"`)
	} else {
		c.Set("Content-Disposition", `inline; filename="JDZ-`+code+`.pdf"`)
	}
	// Redirect to the storage URL (CDN in OSS mode; local static path in dev).
	return c.Redirect(url, fiber.StatusFound)
}

// ListCertificates: GET /admin/certificates?page=&limit= (PermCertificateManage)
//
//	@Summary		List certificates (admin)
//	@Description	Paginated list of all certificates. Access: ecommerce_operator.
//	@Tags			admin,certificates
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			page			query		int		false	"Page number (1-based)"	default(1)
//	@Param			limit			query		int		false	"Page size (max 100)"	default(20)
//	@Success		200				{object}	models.PaginatedResponse{data=[]models.Certificate}
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs certificate.manage)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/certificates [get]
func (h *Handler) ListCertificates(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	certs, total, err := h.service.ListAdmin(c.Context(), page, limit)
	if err != nil {
		log.Printf("Handler.ListCertificates: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list certificates"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(certs, page, limit, total))
}

// GetCertificate: GET /admin/certificates/:id (PermCertificateManage)
//
//	@Summary		Get a certificate by ID (admin)
//	@Description	Fetches a single certificate (with provenance) by its DB ID.
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,certificates
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Certificate ID"
//	@Success		200				{object}	models.Certificate
//	@Failure		400				{object}	models.ErrorResponse	"Invalid certificate ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs certificate.manage)"
//	@Failure		404				{object}	models.ErrorResponse	"Certificate not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/certificates/{id} [get]
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
//
//	@Summary		Regenerate a certificate (admin)
//	@Description	Issues a new cert_code for an existing certificate (replaces the
//	@Description	old code; re-renders the PDF async via the pdf:generate job).
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,certificates
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			id				path		int						true	"Certificate ID"
//	@Success		200				{object}	object					"{cert_code: \"JDZ-xxxxxx\"}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid certificate ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs certificate.manage)"
//	@Failure		404				{object}	models.ErrorResponse	"Certificate not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/certificates/{id}/regenerate [post]
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
