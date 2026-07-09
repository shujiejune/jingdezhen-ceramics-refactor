package engage

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

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

// requestLocale returns the locale from the ?locale= query param, falling back
// to the Accept-Language header. TDD §5.1: ?locale= overrides Accept-Language.
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

// GetActivities: GET /engage?locale=en-US&type=Destination&page=1&limit=20
// Returns published activities for a locale, optionally filtered by the parent
// `type` (e.g. Destination vs Local Lifestyle), paginated.
func (h *Handler) GetActivities(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	typeFilter := c.Query("type") // optional: "Destination", "Local Lifestyle", etc.

	activities, total, err := h.service.GetActivities(c.Context(), locale, typeFilter, page, limit)
	if err != nil {
		log.Printf("Handler.GetActivities: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve activities"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(activities, page, limit, total))
}

// GetActivityArticle: GET /engage/:slug?locale=en-US
func (h *Handler) GetActivityArticle(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Activity slug parameter is required"})
	}
	locale := requestLocale(c)
	article, err := h.service.GetActivityArticle(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Article not found"})
		}
		log.Printf("Handler.GetActivityArticle: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve article"})
	}
	return c.Status(fiber.StatusOK).JSON(article)
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

// AdminListActivities: GET /admin/engage?locale=&status=&type=&page=&limit=
func (h *Handler) AdminListActivities(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := c.Query("locale")
	status := c.Query("status")
	typeFilter := c.Query("type")
	activities, total, err := h.service.AdminListActivities(c.Context(), locale, status, typeFilter, page, limit)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminListActivities: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list activities"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(activities, page, limit, total))
}

// AdminGetActivity: GET /admin/engage/:slug?locale=en-US (any status)
func (h *Handler) AdminGetActivity(c *fiber.Ctx) error {
	slug := c.Params("slug")
	locale := c.Query("locale", models.DefaultLocale)
	activity, err := h.service.AdminGetActivity(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Activity not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminGetActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve activity"})
	}
	return c.Status(fiber.StatusOK).JSON(activity)
}

// AdminCreateActivity: POST /admin/engage
func (h *Handler) AdminCreateActivity(c *fiber.Ctx) error {
	var req models.CreateActivityData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	activity, err := h.service.AdminCreateActivity(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminCreateActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create activity"})
	}
	return c.Status(fiber.StatusCreated).JSON(activity)
}

// AdminUpdateActivity: PUT /admin/engage/:id?locale=en-US
func (h *Handler) AdminUpdateActivity(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity ID"})
	}
	locale := c.Query("locale", models.DefaultLocale)
	var req models.UpdateActivityData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	activity, err := h.service.AdminUpdateActivity(c.Context(), id, locale, req, adminActor(c))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Activity not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "This translation is not editable in its current workflow state"})
		}
		log.Printf("Handler.AdminUpdateActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update activity"})
	}
	return c.Status(fiber.StatusOK).JSON(activity)
}

func (h *Handler) adminTransition(c *fiber.Ctx, to models.ContentStatus) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity ID"})
	}
	var body localeBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	reviewerID, _ := utils.GetUserIDFromContext(c)
	activity, err := h.service.AdminTransitionActivity(c.Context(), id, body.Locale, to, adminActor(c), reviewerID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Activity not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.adminTransition: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to transition activity"})
	}
	return c.Status(fiber.StatusOK).JSON(activity)
}

func (h *Handler) AdminSubmitActivity(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusInReview)
}

func (h *Handler) AdminApproveActivity(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusPublished)
}

func (h *Handler) AdminRejectActivity(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusRejected)
}

func (h *Handler) AdminUnpublishActivity(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusDraft)
}

func (h *Handler) AdminDeleteActivity(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity ID"})
	}
	if err := h.service.AdminDeleteActivity(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Activity not found"})
		}
		log.Printf("Handler.AdminDeleteActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete activity"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
