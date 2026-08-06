package ceramicstory

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

// Handler handles HTTP requests for ceramic stories (History & Heritage).
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
// to the Accept-Language header (first entry, lowercased). TDD §5.1: ?locale=
// overrides Accept-Language; default en-US.
func requestLocale(c *fiber.Ctx) string {
	loc := c.Query("locale")
	if loc != "" {
		return loc
	}
	// Accept-Language header fallback (simple first-token extraction).
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

// GetAllDynasties: GET /ceramicstory?locale=en-US
//
//	@Summary		List ceramic stories
//	@Description	Returns all published ceramic-story/dynasty entries for the requested locale.
//	@Tags			ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			locale	query		string	false	"BCP 47 locale (e.g. en-US). Overrides Accept-Language."	default("en-US")
//	@Success		200		{array}		models.CeramicStory
//	@Failure		500		{object}	models.ErrorResponse	"Internal error"
//	@Router			/ceramicstory [get]
func (h *Handler) GetAllDynasties(c *fiber.Ctx) error {
	locale := requestLocale(c)
	stories, err := h.service.GetAllCeramicStories(c.Context(), locale)
	if err != nil {
		log.Printf("Handler.GetAllDynasties: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic stories"})
	}
	if len(stories) == 0 {
		return c.Status(fiber.StatusOK).JSON([]models.CeramicStory{})
	}
	return c.Status(fiber.StatusOK).JSON(stories)
}

// GetDynastyDetail: GET /ceramicstory/:slug?locale=en-US
//
//	@Summary		Get a ceramic story by slug
//	@Description	Fetches a single published ceramic story by its locale-specific slug.
//	@Tags			ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			slug	path		string	true	"Story slug (locale-specific)"
//	@Param			locale	query		string	false	"BCP 47 locale (e.g. en-US). Overrides Accept-Language."	default("en-US")
//	@Success		200		{object}	models.CeramicStory
//	@Failure		400		{object}	models.ErrorResponse	"Missing slug"
//	@Failure		404		{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		500		{object}	models.ErrorResponse	"Internal error"
//	@Router			/ceramicstory/{slug} [get]
func (h *Handler) GetDynastyDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Dynasty slug parameter is required"})
	}
	locale := requestLocale(c)
	story, err := h.service.GetCeramicStoryDetail(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		log.Printf("Handler.GetDynastyDetail: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic story details"})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}

// =============================================================================
// Admin / CMS handlers (PRD §3.1.1 editorial workflow)
// =============================================================================

// adminActor derives the workflow actor from the JWT roles. A super admin can
// act as editor + has exclusive powers; an editor is the default staff role.
func adminActor(c *fiber.Ctx) i18ncontent.WorkflowActor {
	roles, _ := c.Locals("userRoles").([]string)
	for _, r := range roles {
		if r == models.RoleSuperAdmin {
			return i18ncontent.ActorSuperAdmin
		}
	}
	return i18ncontent.ActorEditor
}

// localeBody is the common body for transition endpoints.
type localeBody struct {
	Locale string `json:"locale" validate:"required,len=5"`
}

// AdminListStories: GET /admin/ceramicstory?locale=&status=&page=&limit=
//
//	@Summary		List ceramic stories (admin, any status)
//	@Description	Paginated list of stories filtered by locale + status. Access: content_editor.
//	@Tags			admin,ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			locale			query		string	false	"BCP 47 locale"
//	@Param			status			query		string	false	"Filter by workflow status"
//	@Param			page			query		int		false	"Page number (1-based)"	default(1)
//	@Param			limit			query		int		false	"Page size (max 100)"	default(20)
//	@Success		200				{object}	models.PaginatedResponse{data=[]models.CeramicStory}
//	@Failure		400				{object}	models.ErrorResponse	"Invalid locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory [get]
func (h *Handler) AdminListStories(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := c.Query("locale") // optional filter
	status := c.Query("status") // optional filter: draft|in_review|published|rejected
	stories, total, err := h.service.AdminListStories(c.Context(), locale, status, page, limit)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminListStories: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list stories"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(stories, page, limit, total))
}

// AdminGetStory: GET /admin/ceramicstory/:slug?locale=en-US (any status)
//
//	@Summary		Get a ceramic story by slug (admin, any status)
//	@Description	Fetches a single story by slug in any workflow status. Access: content_editor.
//	@Tags			admin,ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			slug			path		string	true	"Story slug"
//	@Param			locale			query		string	false	"BCP 47 locale"	default(en-US)
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{slug} [get]
func (h *Handler) AdminGetStory(c *fiber.Ctx) error {
	slug := c.Params("slug")
	locale := c.Query("locale", models.DefaultLocale)
	story, err := h.service.AdminGetStory(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminGetStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve story"})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}

// AdminCreateStory: POST /admin/ceramicstory
//
//	@Summary		Create a ceramic story
//	@Description	Creates a story + its first translation. Access: content_editor.
//	@Tags			admin,ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string							true	"Bearer <access_token>"
//	@Param			body			body		models.CreateCeramicStoryData	true	"Story to create"
//	@Success		201				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory [post]
func (h *Handler) AdminCreateStory(c *fiber.Ctx) error {
	var req models.CreateCeramicStoryData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	story, err := h.service.AdminCreateStory(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminCreateStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create story"})
	}
	return c.Status(fiber.StatusCreated).JSON(story)
}

// AdminUpdateStory: PUT /admin/ceramicstory/:id?locale=en-US
//
//	@Summary		Update a ceramic story
//	@Description	Updates a story's translation + parent fields (nil = unchanged). Access: content_editor.
//	@Tags			admin,ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string							true	"Bearer <access_token>"
//	@Param			id				path		int								true	"Story ID"
//	@Param			locale			query		string							false	"BCP 47 locale"	default(en-US)
//	@Param			body			body		models.UpdateCeramicStoryData	true	"Fields to update (nil pointers = unchanged)"
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID / body / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		409				{object}	models.ErrorResponse	"Translation not editable in its current workflow state"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id} [put]
func (h *Handler) AdminUpdateStory(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story ID"})
	}
	locale := c.Query("locale", models.DefaultLocale)
	var req models.UpdateCeramicStoryData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	story, err := h.service.AdminUpdateStory(c.Context(), id, locale, req, adminActor(c))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "This translation is not editable in its current workflow state"})
		}
		log.Printf("Handler.AdminUpdateStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update story"})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}

// adminTransition is the shared handler for submit/approve/reject/unpublish.
func (h *Handler) adminTransition(c *fiber.Ctx, to models.ContentStatus) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story ID"})
	}
	var body localeBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	reviewerID, _ := utils.GetUserIDFromContext(c)
	story, err := h.service.AdminTransitionStory(c.Context(), id, body.Locale, to, adminActor(c), reviewerID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.adminTransition: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to transition story"})
	}
	if act := audit.ActionForTransition(to); act != "" {
		h.audit.Log(c, act, models.AuditEntityCeramicStory, strconv.FormatInt(id, 10), map[string]any{"locale": body.Locale, "to": string(to)})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}

// AdminSubmitStory: POST /admin/ceramicstory/:id/submit (draft → in_review)
//
//	@Summary		Submit a ceramic story for review
//	@Description	Transitions a draft story translation to in_review. Body: {locale}. Access: content_editor.
//	@Tags			admin,ceramicstory,workflow
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Story ID"
//	@Param			body			body		object	true	"{locale: en-US}"
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID / body / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		409				{object}	models.ErrorResponse	"Invalid workflow transition"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/submit [post]
func (h *Handler) AdminSubmitStory(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusInReview)
}

// AdminApproveStory: POST /admin/ceramicstory/:id/approve (in_review → published)
//
//	@Summary		Approve + publish a ceramic story
//	@Description	Transitions an in_review story translation to published. Access: super_admin ONLY.
//	@Tags			admin,ceramicstory,workflow
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Story ID"
//	@Param			body			body		object	true	"{locale: en-US}"
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID / body / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.publish — super_admin only)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		409				{object}	models.ErrorResponse	"Invalid workflow transition"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/approve [post]
func (h *Handler) AdminApproveStory(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusPublished)
}

// AdminRejectStory: POST /admin/ceramicstory/:id/reject (in_review → rejected)
//
//	@Summary		Reject a ceramic story (in_review → rejected)
//	@Description	Transitions an in_review story translation to rejected. Access: super_admin ONLY.
//	@Tags			admin,ceramicstory,workflow
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Story ID"
//	@Param			body			body		object	true	"{locale: en-US}"
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID / body / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.publish — super_admin only)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		409				{object}	models.ErrorResponse	"Invalid workflow transition"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/reject [post]
func (h *Handler) AdminRejectStory(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusRejected)
}

// AdminUnpublishStory: POST /admin/ceramicstory/:id/unpublish (published → draft)
//
//	@Summary		Unpublish a ceramic story (published → draft)
//	@Description	Transitions a published story translation back to draft. Access: super_admin ONLY.
//	@Tags			admin,ceramicstory,workflow
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Story ID"
//	@Param			body			body		object	true	"{locale: en-US}"
//	@Success		200				{object}	models.CeramicStory
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID / body / bad locale"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.publish — super_admin only)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		409				{object}	models.ErrorResponse	"Invalid workflow transition"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/unpublish [post]
func (h *Handler) AdminUnpublishStory(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusDraft)
}

// AdminDeleteStory: DELETE /admin/ceramicstory/:id
//
//	@Summary		Delete a ceramic story
//	@Description	Removes a story (parent + all translations). Access: content_editor.
//	@Tags			admin,ceramicstory
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Story ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Ceramic story not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id} [delete]
func (h *Handler) AdminDeleteStory(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story ID"})
	}
	if err := h.service.AdminDeleteStory(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		log.Printf("Handler.AdminDeleteStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete story"})
	}
	h.audit.Log(c, models.AuditActionCeramicStoryDelete, models.AuditEntityCeramicStory, strconv.FormatInt(id, 10), nil)
	return c.SendStatus(fiber.StatusNoContent)
}
