package engage

import (
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
	audit    *audit.Helper
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// SetAuditLogger injects the audit logger (PRD §3.1.1). Nil = no-op (tests).
func (h *Handler) SetAuditLogger(l audit.Logger) { h.audit = audit.NewHelper(l) }

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
//
// @Summary      List published activities
// @Description  Paginated list of published activities for a locale, optionally filtered by type.
// @Tags         engage
// @Accept       json
// @Produce      json
// @Param        locale query string false "BCP 47 locale (e.g. en-US). Overrides Accept-Language." default("en-US")
// @Param        type   query string false "Activity type filter (e.g. Destination, Local Lifestyle)"
// @Param        page   query int    false "Page number (1-based)" default(1)
// @Param        limit  query int    false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.Activity}
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /engage [get]
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
//
// @Summary      Get an activity by slug
// @Description  Fetches a single published activity by its locale-specific slug.
// @Tags         engage
// @Accept       json
// @Produce      json
// @Param        slug   path string true "Activity slug (locale-specific)"
// @Param        locale query string false "BCP 47 locale (e.g. en-US). Overrides Accept-Language." default("en-US")
// @Success      200 {object} models.Activity
// @Failure      400 {object} models.ErrorResponse "Missing slug"
// @Failure      404 {object} models.ErrorResponse "Article not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /engage/{slug} [get]
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
//
// @Summary      List activities (admin, any status)
// @Description  Paginated list of activities filtered by locale, status, type. Access: content_editor.
// @Tags         admin,engage
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        locale query string false "BCP 47 locale"
// @Param        status query string false "Filter by workflow status"
// @Param        type   query string false "Filter by activity type"
// @Param        page   query int    false "Page number (1-based)" default(1)
// @Param        limit  query int    false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.Activity}
// @Failure      400 {object} models.ErrorResponse "Invalid locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/engage [get]
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
//
// @Summary      Get an activity by slug (admin, any status)
// @Description  Fetches a single activity by slug in any workflow status. Access: content_editor.
// @Tags         admin,engage
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        slug   path string  true "Activity slug"
// @Param        locale query string false "BCP 47 locale" default(en-US)
// @Success      200 {object} models.Activity
// @Failure      400 {object} models.ErrorResponse "Invalid locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      404 {object} models.ErrorResponse "Activity not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/engage/{slug} [get]
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
//
// @Summary      Create an activity
// @Description  Creates an activity + its first translation. Access: content_editor.
// @Tags         admin,engage
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.CreateActivityData true "Activity to create"
// @Success      201 {object} models.Activity
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/engage [post]
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
//
// @Summary      Update an activity
// @Description  Updates an activity's translation + parent fields (nil = unchanged). Access: content_editor.
// @Tags         admin,engage
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id     path int true "Activity ID"
// @Param        locale query string false "BCP 47 locale" default(en-US)
// @Param        body   body models.UpdateActivityData true "Fields to update (nil pointers = unchanged)"
// @Success      200 {object} models.Activity
// @Failure      400 {object} models.ErrorResponse "Invalid activity ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      404 {object} models.ErrorResponse "Activity not found"
// @Failure      409 {object} models.ErrorResponse "Translation not editable in its current workflow state"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/engage/{id} [put]
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
	if act := audit.ActionForTransition(to); act != "" {
		h.audit.Log(c, act, models.AuditEntityActivity, strconv.FormatInt(id, 10), map[string]any{"locale": body.Locale, "to": string(to)})
	}
	return c.Status(fiber.StatusOK).JSON(activity)
}

func (h *Handler) AdminSubmitActivity(c *fiber.Ctx) error {
	//
	// @Summary      Submit an activity for review
	// @Description  Transitions a draft activity translation to in_review. Body: {locale}.
	// @Description  Access: content_editor.
	// @Tags         admin,engage,workflow
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        id   path int true "Activity ID"
	// @Param        body body object true "{locale: en-US}"
	// @Success      200 {object} models.Activity
	// @Failure      400 {object} models.ErrorResponse "Invalid activity ID / body / bad locale"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
	// @Failure      404 {object} models.ErrorResponse "Activity not found"
	// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/engage/{id}/submit [post]
	return h.adminTransition(c, models.StatusInReview)
}

func (h *Handler) AdminApproveActivity(c *fiber.Ctx) error {
	//
	// @Summary      Approve + publish an activity
	// @Description  Transitions an in_review activity translation to published. Access: super_admin ONLY.
	// @Tags         admin,engage,workflow
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        id   path int true "Activity ID"
	// @Param        body body object true "{locale: en-US}"
	// @Success      200 {object} models.Activity
	// @Failure      400 {object} models.ErrorResponse "Invalid activity ID / body / bad locale"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.publish — super_admin only)"
	// @Failure      404 {object} models.ErrorResponse "Activity not found"
	// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/engage/{id}/approve [post]
	return h.adminTransition(c, models.StatusPublished)
}

func (h *Handler) AdminRejectActivity(c *fiber.Ctx) error {
	//
	// @Summary      Reject an activity (in_review → rejected)
	// @Description  Transitions an in_review activity translation to rejected. Access: super_admin ONLY.
	// @Tags         admin,engage,workflow
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        id   path int true "Activity ID"
	// @Param        body body object true "{locale: en-US}"
	// @Success      200 {object} models.Activity
	// @Failure      400 {object} models.ErrorResponse "Invalid activity ID / body / bad locale"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.publish — super_admin only)"
	// @Failure      404 {object} models.ErrorResponse "Activity not found"
	// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/engage/{id}/reject [post]
	return h.adminTransition(c, models.StatusRejected)
}

func (h *Handler) AdminUnpublishActivity(c *fiber.Ctx) error {
	//
	// @Summary      Unpublish an activity (published → draft)
	// @Description  Transitions a published activity translation back to draft. Access: super_admin ONLY.
	// @Tags         admin,engage,workflow
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        id   path int true "Activity ID"
	// @Param        body body object true "{locale: en-US}"
	// @Success      200 {object} models.Activity
	// @Failure      400 {object} models.ErrorResponse "Invalid activity ID / body / bad locale"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.publish — super_admin only)"
	// @Failure      404 {object} models.ErrorResponse "Activity not found"
	// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/engage/{id}/unpublish [post]
	return h.adminTransition(c, models.StatusDraft)
}

func (h *Handler) AdminDeleteActivity(c *fiber.Ctx) error {
	//
	// @Summary      Delete an activity
	// @Description  Removes an activity (parent + all translations). Access: content_editor.
	// @Tags         admin,engage
	// @Accept       json
	// @Produce      json
	// @Param        Authorization header string true "Bearer <access_token>"
	// @Param        id path int true "Activity ID"
	// @Success      204 "No Content (empty body)"
	// @Failure      400 {object} models.ErrorResponse "Invalid activity ID"
	// @Failure      401 {object} models.ErrorResponse "Authentication required"
	// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
	// @Failure      404 {object} models.ErrorResponse "Activity not found"
	// @Failure      500 {object} models.ErrorResponse "Internal error"
	// @Security     BearerAuth
	// @Router       /admin/engage/{id} [delete]
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
	h.audit.Log(c, models.AuditActivityDelete, models.AuditEntityActivity, strconv.FormatInt(id, 10), nil)
	return c.SendStatus(fiber.StatusNoContent)
}
