package artist

import (
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for artist profiles (PRD §3.1.3 / §3.2.1).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// requestLocale returns the locale from ?locale= query param, falling back to
// Accept-Language header. TDD §5.1: ?locale= overrides Accept-Language.
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

// --- Public reads ---

// GetArtists: GET /artists?locale=en-US&page=&limit=
func (h *Handler) GetArtists(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	artists, total, err := h.service.GetArtists(c.Context(), locale, page, limit)
	if err != nil {
		log.Printf("Handler.GetArtists: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve artists"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(artists, page, limit, total))
}

// GetArtistBySlug: GET /artists/:slug?locale=en-US
func (h *Handler) GetArtistBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Artist slug parameter is required"})
	}
	locale := requestLocale(c)
	artist, err := h.service.GetArtistBySlug(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artist not found"})
		}
		log.Printf("Handler.GetArtistBySlug: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve artist"})
	}
	return c.Status(fiber.StatusOK).JSON(artist)
}

// =============================================================================
// Admin / CMS handlers (PRD §3.1.1 editorial workflow)
// =============================================================================

func adminActor(c *fiber.Ctx) i18ncontent.WorkflowActor {
	roles, _ := c.Locals("userRoles").([]string)
	for _, r := range roles {
		if r == models.RoleSuperAdmin {
			return i18ncontent.ActorSuperAdmin
		}
	}
	return i18ncontent.ActorEditor
}

type localeBody struct {
	Locale string `json:"locale" validate:"required,len=5"`
}

// AdminListArtists: GET /admin/artists?locale=&status=&page=&limit=
func (h *Handler) AdminListArtists(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := c.Query("locale")
	status := c.Query("status")
	artists, total, err := h.service.AdminListArtists(c.Context(), locale, status, page, limit)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminListArtists: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list artists"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(artists, page, limit, total))
}

// AdminGetArtist: GET /admin/artists/:slug?locale=en-US (any status)
func (h *Handler) AdminGetArtist(c *fiber.Ctx) error {
	slug := c.Params("slug")
	locale := c.Query("locale", models.DefaultLocale)
	artist, err := h.service.AdminGetArtist(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artist not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminGetArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve artist"})
	}
	return c.Status(fiber.StatusOK).JSON(artist)
}

// AdminCreateArtist: POST /admin/artists
func (h *Handler) AdminCreateArtist(c *fiber.Ctx) error {
	var req models.CreateArtistData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	artist, err := h.service.AdminCreateArtist(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminCreateArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create artist"})
	}
	return c.Status(fiber.StatusCreated).JSON(artist)
}

// AdminUpdateArtist: PUT /admin/artists/:id?locale=en-US
func (h *Handler) AdminUpdateArtist(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist ID"})
	}
	locale := c.Query("locale", models.DefaultLocale)
	var req models.UpdateArtistData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	artist, err := h.service.AdminUpdateArtist(c.Context(), id, locale, req, adminActor(c))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artist not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "This translation is not editable in its current workflow state"})
		}
		log.Printf("Handler.AdminUpdateArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update artist"})
	}
	return c.Status(fiber.StatusOK).JSON(artist)
}

func (h *Handler) adminTransition(c *fiber.Ctx, to models.ContentStatus) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist ID"})
	}
	var body localeBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	reviewerID, _ := utils.GetUserIDFromContext(c)
	artist, err := h.service.AdminTransitionArtist(c.Context(), id, body.Locale, to, adminActor(c), reviewerID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artist not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.adminTransition: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to transition artist"})
	}
	return c.Status(fiber.StatusOK).JSON(artist)
}

// AdminSubmitArtist: POST /admin/artists/:id/submit (draft → in_review)
func (h *Handler) AdminSubmitArtist(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusInReview)
}

// AdminApproveArtist: POST /admin/artists/:id/approve (in_review → published)
func (h *Handler) AdminApproveArtist(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusPublished)
}

// AdminRejectArtist: POST /admin/artists/:id/reject (in_review → rejected)
func (h *Handler) AdminRejectArtist(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusRejected)
}

// AdminUnpublishArtist: POST /admin/artists/:id/unpublish (published → draft)
func (h *Handler) AdminUnpublishArtist(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusDraft)
}

// AdminDeleteArtist: DELETE /admin/artists/:id
func (h *Handler) AdminDeleteArtist(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist ID"})
	}
	if err := h.service.AdminDeleteArtist(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artist not found"})
		}
		log.Printf("Handler.AdminDeleteArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete artist"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
