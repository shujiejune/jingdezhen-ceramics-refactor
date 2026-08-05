package itinerary

import (
	"encoding/csv"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the itinerary wizard (PRD §3.3.2, TDD §5.2).
type Handler struct {
	service  ServiceInterface
	store    storage.Store // resolves quote pdf_key → public URL (local path or CDN)
	validate *validator.Validate
}

func NewHandler(service ServiceInterface, store storage.Store) *Handler {
	return &Handler{service: service, store: store, validate: validator.New()}
}

// requestLocale returns ?locale= or falls back to Accept-Language (mirrors order).
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

// --- Draft ---

// GetDraft: GET /itineraries/draft
//
// @Summary      Get the current user's itinerary draft
// @Description  Returns the signed-in user's saved 4-step wizard draft (one per user).
// @Description  404 if no draft is saved.
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {object} models.ItineraryDraft
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "No saved draft"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/draft [get]
func (h *Handler) GetDraft(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	draft, err := h.service.GetDraft(c.Context(), userID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "No saved draft"})
		}
		log.Printf("Handler.GetDraft: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve draft"})
	}
	return c.Status(fiber.StatusOK).JSON(draft)
}

// SaveDraft: PUT /itineraries/draft
//
// @Summary      Save the itinerary draft
// @Description  Upserts the signed-in user's wizard draft (one per user via UNIQUE).
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.ItineraryDraftData true "Draft data (4-step wizard)"
// @Success      200 {object} models.ItineraryDraft
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / invalid operation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/draft [put]
func (h *Handler) SaveDraft(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.ItineraryDraftData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	draft, err := h.service.SaveDraft(c.Context(), userID, req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidOperation) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.SaveDraft: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to save draft"})
	}
	return c.Status(fiber.StatusOK).JSON(draft)
}

// DeleteDraft: DELETE /itineraries/draft
//
// @Summary      Delete the itinerary draft
// @Description  Removes the signed-in user's saved draft.
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      204 "No Content (empty body)"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/draft [delete]
func (h *Handler) DeleteDraft(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	if err := h.service.DeleteDraft(c.Context(), userID); err != nil {
		log.Printf("Handler.DeleteDraft: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete draft"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Requests ---

// Submit: POST /itineraries (signed-in). Body: the full 4-step wizard.
//
// @Summary      Submit an itinerary request
// @Description  Submits the 4-step wizard as a request (pending → awaiting planner).
// @Description  Requires GDPR consent. Locale resolved from ?locale= / Accept-Language.
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        locale query string false "BCP 47 locale (e.g. en-US)." default("en-US")
// @Param        body body models.ItinerarySubmitRequest true "Full 4-step wizard payload"
// @Success      201 {object} models.ItineraryRequest
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / consent required / invalid operation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries [post]
func (h *Handler) Submit(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.ItinerarySubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	out, err := h.service.Submit(c.Context(), userID, req, requestLocale(c))
	if err != nil {
		if errors.Is(err, models.ErrConsentRequired) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidOperation) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.Submit: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to submit itinerary request"})
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// ListMine: GET /itineraries?page=&limit=
//
// @Summary      List the current user's itinerary requests
// @Description  Paginated list of the signed-in user's itinerary requests.
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        page  query int false "Page number (1-based)" default(1)
// @Param        limit query int false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.ItineraryRequest}
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries [get]
func (h *Handler) ListMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	page, limit := utils.GetPageLimit(c)
	reqs, total, err := h.service.ListMine(c.Context(), userID, page, limit)
	if err != nil {
		log.Printf("Handler.ListMine: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list itinerary requests"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(reqs, page, limit, total))
}

// GetMine: GET /itineraries/:id
//
// @Summary      Get one of the current user's itinerary requests
// @Description  Fetches a single itinerary request owned by the signed-in user.
// @Description  An unowned request returns 404 (no cross-user access).
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      200 {object} models.ItineraryRequest
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/{id} [get]
func (h *Handler) GetMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	req, err := h.service.GetMine(c.Context(), userID, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Itinerary request not found"})
		}
		log.Printf("Handler.GetMine: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve itinerary request"})
	}
	return c.Status(fiber.StatusOK).JSON(req)
}

// CancelMine: POST /itineraries/:id/cancel (pending → cancelled)
//
// @Summary      Cancel an itinerary request (customer)
// @Description  Cancels the signed-in user's pending request. Only pending
// @Description  requests are cancellable; other statuses return 409. Body {reason} optional.
// @Tags         itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.ItineraryCancelRequest false "Optional cancel reason"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found (or not owned by user)"
// @Failure      409 {object} models.ErrorResponse "Itinerary request is not cancellable"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/{id}/cancel [post]
func (h *Handler) CancelMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.ItineraryCancelRequest
	// Body is optional; an empty body is fine.
	_ = c.BodyParser(&req)
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.Cancel(c.Context(), userID, id, req); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Itinerary request not found"})
		}
		if errors.Is(err, models.ErrItineraryNotCancellable) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.CancelMine: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to cancel itinerary request"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// =============================================================================
// Planner CRM handlers (PRD §3.3.2 "Backend/CRM")
// =============================================================================

// mapItinAdminErr maps planner-CRM errors to HTTP statuses.
func mapItinAdminErr(c *fiber.Ctx, err error, op string) error {
	switch {
	case errors.Is(err, models.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Itinerary request not found"})
	case errors.Is(err, models.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Itinerary request is not in a valid state for this operation"})
	case errors.Is(err, models.ErrItineraryNotCancellable):
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
	case errors.Is(err, models.ErrInvalidOperation):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	default:
		log.Printf("Handler.%s: %v", op, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to process itinerary operation"})
	}
}

// AdminList: GET /admin/itineraries?status=&assigned_to=&sla=&page=&limit=
//
// @Summary      List itinerary requests (planner CRM inbox)
// @Description  Paginated list of all itinerary requests, JOINed with the customer's
// @Description  email/nickname. Filterable by status, assignee, and SLA state.
// @Description  Access: travel_planner, customer_service.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        status      query string false "Filter by status"
// @Param        assigned_to query string false "Filter by assignee (user ID, or 'unassigned')"
// @Param        sla         query string false "Filter by SLA state (on_time|approaching|breached|met)"
// @Param        page        query int    false "Page number (1-based)" default(1)
// @Param        limit       query int    false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.ItineraryAdminRow}
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries [get]
func (h *Handler) AdminList(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	status := c.Query("status")
	assignedTo := c.Query("assigned_to")
	sla := c.Query("sla")
	rows, total, err := h.service.ListAdmin(c.Context(), status, assignedTo, sla, page, limit)
	if err != nil {
		log.Printf("Handler.AdminList: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list itinerary requests"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(rows, page, limit, total))
}

// AdminGet: GET /admin/itineraries/:id
//
// @Summary      Get an itinerary request (admin)
// @Description  Fetches a single itinerary request (planner detail view).
// @Description  Access: travel_planner, customer_service.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      200 {object} models.ItineraryAdminRow
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id} [get]
func (h *Handler) AdminGet(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	row_, err := h.service.GetAdmin(c.Context(), id)
	if err != nil {
		return mapItinAdminErr(c, err, "AdminGet")
	}
	return c.Status(fiber.StatusOK).JSON(row_)
}

// AdminListPlanners: GET /admin/itineraries/planners (assignment dropdown)
//
// @Summary      List planners (assignment dropdown)
// @Description  Returns travel_planner-role users for the assignment dropdown.
// @Description  Access: travel_planner, customer_service.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {array} object
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/planners [get]
func (h *Handler) AdminListPlanners(c *fiber.Ctx) error {
	planners, err := h.service.ListPlanners(c.Context())
	if err != nil {
		log.Printf("Handler.AdminListPlanners: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list planners"})
	}
	return c.Status(fiber.StatusOK).JSON(planners)
}

// AdminOpen: POST /admin/itineraries/:id/open (pending → processing)
//
// @Summary      Open an itinerary request (pending → processing)
// @Description  Explicit state transition (not auto-on-view). Access: travel_planner.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/open [post]
func (h *Handler) AdminOpen(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	if err := h.service.Open(c.Context(), id); err != nil {
		return mapItinAdminErr(c, err, "AdminOpen")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AdminClose: POST /admin/itineraries/:id/close ({pending,processing} → closed)
//
// @Summary      Close an itinerary request
// @Description  Transitions a pending/processing request to closed. Body {reason} optional.
// @Description  Access: travel_planner.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.ItineraryReasonRequest false "Optional close reason"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/close [post]
func (h *Handler) AdminClose(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.ItineraryReasonRequest
	_ = c.BodyParser(&req) // optional
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.Close(c.Context(), id, req); err != nil {
		return mapItinAdminErr(c, err, "AdminClose")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AdminCancel: POST /admin/itineraries/:id/cancel ({pending,processing} → cancelled, staff)
//
// @Summary      Cancel an itinerary request (staff)
// @Description  Transitions a pending/processing request to cancelled (by staff).
// @Description  Body {reason} optional. Access: travel_planner.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.ItineraryReasonRequest false "Optional cancel reason"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/cancel [post]
func (h *Handler) AdminCancel(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.ItineraryReasonRequest
	_ = c.BodyParser(&req) // optional
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.CancelByStaff(c.Context(), id, req); err != nil {
		return mapItinAdminErr(c, err, "AdminCancel")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AdminAssign: POST /admin/itineraries/:id/assign
//
// @Summary      Assign an itinerary request to a planner
// @Description  Sets the assigned_to planner. Body {assigned_to: <user_id|nil>}
// @Description  (nil or empty = unassign). Access: travel_planner.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.AssignItineraryRequest true "assigned_to (user ID, or empty to unassign)"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/assign [post]
func (h *Handler) AdminAssign(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.AssignItineraryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.Assign(c.Context(), id, req); err != nil {
		return mapItinAdminErr(c, err, "AdminAssign")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AdminAddNote: POST /admin/itineraries/:id/notes
//
// @Summary      Add a CRM note to an itinerary request
// @Description  Appends an internal CRM note (planner-only, not customer-visible).
// @Description  Access: travel_planner, customer_service.
// @Tags         admin,itineraries,notes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.ItineraryNoteRequest true "Note text"
// @Success      201 {object} models.CRMNote
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/notes [post]
func (h *Handler) AdminAddNote(c *fiber.Ctx) error {
	authorID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.ItineraryNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	note, err := h.service.AddNote(c.Context(), id, authorID, req)
	if err != nil {
		return mapItinAdminErr(c, err, "AdminAddNote")
	}
	return c.Status(fiber.StatusCreated).JSON(note)
}

// AdminListNotes: GET /admin/itineraries/:id/notes
//
// @Summary      List CRM notes for an itinerary request
// @Description  Returns the internal CRM notes (planner-only). Access: itinerary.read.
// @Tags         admin,itineraries,notes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      200 {array} models.CRMNote
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/notes [get]
func (h *Handler) AdminListNotes(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	notes, err := h.service.ListNotes(c.Context(), id)
	if err != nil {
		log.Printf("Handler.AdminListNotes: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list notes"})
	}
	return c.Status(fiber.StatusOK).JSON(notes)
}

// AdminExport: GET /admin/itineraries/export?status=&assigned_to=&sla=
// Streams a CSV of the matching requests (PRD §3.3.2 "Data export (CSV/Excel)"
// — CSV for MVP; Excel is the same data in a richer format later).
//
// @Summary      Export itinerary requests as CSV
// @Description  Streams a CSV of all matching itinerary requests (no pagination;
// @Description  MVP volume is low). Access: travel_planner, customer_service.
// @Tags         admin,itineraries
// @Produce      text/csv
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        status      query string false "Filter by status"
// @Param        assigned_to query string false "Filter by assignee"
// @Param        sla         query string false "Filter by SLA state"
// @Success      200 {file} binary "CSV file (Content-Disposition: attachment)"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/export [get]
func (h *Handler) AdminExport(c *fiber.Ctx) error {
	status := c.Query("status")
	assignedTo := c.Query("assigned_to")
	sla := c.Query("sla")
	// Export ignores pagination — fetch all matching rows (MVP volume is low).
	rows, _, err := h.service.ListAdmin(c.Context(), status, assignedTo, sla, 1, 10000)
	if err != nil {
		log.Printf("Handler.AdminExport: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to export itinerary requests"})
	}
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="itinerary-requests.csv"`)
	w := csv.NewWriter(c.Response().BodyWriter())
	header := []string{"id", "status", "sla_status", "submitted_at", "customer_email",
		"customer_nickname", "arrival_date", "duration_days", "flexible", "adults",
		"children", "pace", "interests", "budget", "services", "contact", "notes",
		"locale", "sla_deadline", "assigned_to"}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		arrival := ""
		if r.ArrivalDate != nil {
			arrival = r.ArrivalDate.Format("2006-01-02")
		}
		assigned := ""
		if r.AssignedTo != nil {
			assigned = *r.AssignedTo
		}
		if err := w.Write([]string{
			strconv.FormatInt(r.ID, 10), string(r.Status), r.SLAStatus,
			r.SubmittedAt.Format(time.RFC3339), r.CustomerEmail, r.CustomerNickname,
			arrival, strconv.Itoa(r.DurationDays), strconv.FormatBool(r.Flexible),
			strconv.Itoa(r.Adults), strconv.Itoa(r.Children), r.Pace,
			string(r.Interests), string(r.Budget), string(r.Services), string(r.Contact),
			derefStr(r.Notes), r.Locale, r.SLADeadline.Format(time.RFC3339), assigned,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return nil
}

// derefStr returns *s or "" for a nil *string (CSV cells must be strings).
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// =============================================================================
// Quote builder + deposit handlers (PRD §3.3.2, TDD §3.4 M3 #3)
// =============================================================================

// --- Customer-facing ---

// PayDeposit: POST /itineraries/:id/pay-deposit (signed-in customer).
//
// @Summary      Pay the itinerary deposit
// @Description  Pays the deposit (30% by default, or full amount with pay_full=true)
// @Description  via the payment gateway. Returns the hosted checkout URL (sandbox/live)
// @Description  or the paid confirmation (mock mode).
// @Tags         itineraries,quotes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.PayDepositRequest true "pay_full toggle + gateway"
// @Success      200 {object} models.DepositPaidResponse
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / no quote"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/{id}/pay-deposit [post]
func (h *Handler) PayDeposit(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.PayDepositRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	out, err := h.service.PayDeposit(c.Context(), userID, id, req)
	if err != nil {
		return mapItinAdminErr(c, err, "PayDeposit")
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

// GetQuote: GET /itineraries/:id/quote (customer reads their own quote).
//
// @Summary      Get the quote for an itinerary request (customer)
// @Description  Fetches the quote for one of the signed-in user's requests.
// @Description  Ownership-checked; an unowned request returns 404.
// @Tags         itineraries,quotes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      200 {object} models.ItineraryQuote
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / no quote"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/{id}/quote [get]
func (h *Handler) GetQuote(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	// Verify ownership (GetByIDForUser scopes to the user) before reading the quote.
	if _, err := h.service.GetMine(c.Context(), userID, id); err != nil {
		return mapItinAdminErr(c, err, "GetQuote.Ownership")
	}
	q, err := h.service.GetQuote(c.Context(), id)
	if err != nil {
		return mapItinAdminErr(c, err, "GetQuote")
	}
	return c.Status(fiber.StatusOK).JSON(q)
}

// --- Planner-facing (admin) ---

// AdminListOptionRates: GET /admin/itineraries/option-rates (the mocked CMS
// rate table — the planner's price book).
//
// @Summary      List option rates (planner price book)
// @Description  Returns the option_rates rows (the planner's price book).
// @Description  Access: travel_planner, customer_service (itinerary.read).
// @Tags         admin,itineraries,option-rates
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {array} models.OptionRate
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/option-rates [get]
func (h *Handler) AdminListOptionRates(c *fiber.Ctx) error {
	rates, err := h.service.ListOptionRates(c.Context())
	if err != nil {
		log.Printf("Handler.AdminListOptionRates: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list option rates"})
	}
	return c.Status(fiber.StatusOK).JSON(rates)
}

// AdminCreateOptionRate: POST /admin/itineraries/option-rates (settings.manage).
// option_key is the canonical immutable identifier (lowercase kebab).
//
// @Summary      Create an option rate (admin)
// @Description  Adds a rate to the planner's price book. option_key is immutable after create.
// @Description  Access: super_admin (settings.manage — pricing is a business decision).
// @Tags         admin,itineraries,option-rates
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.CreateOptionRateRequest true "Rate to create (option_key + rate_cny + unit + label)"
// @Success      201 {object} models.OptionRate
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / bad option_key (lowercase kebab)"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs settings.manage)"
// @Failure      409 {object} models.ErrorResponse "An option rate with that option_key already exists"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/option-rates [post]
func (h *Handler) AdminCreateOptionRate(c *fiber.Ctx) error {
	var req models.CreateOptionRateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	o, err := h.service.CreateOptionRate(c.Context(), req)
	if err != nil {
		return mapOptionRateErr(c, err, "AdminCreateOptionRate")
	}
	return c.Status(fiber.StatusCreated).JSON(o)
}

// AdminUpdateOptionRate: PUT /admin/itineraries/option-rates/:id (settings.manage).
// Mutates rate_cny/unit/display_label only; option_key is immutable.
//
// @Summary      Update an option rate (admin)
// @Description  Updates rate_cny/unit/display_label. option_key is immutable (renames
// @Description  would orphan historical quote snapshots). Access: super_admin.
// @Tags         admin,itineraries,option-rates
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Option rate ID"
// @Param        body body models.UpdateOptionRateRequest true "Fields to update (rate_cny, unit, display_label)"
// @Success      200 {object} models.OptionRate
// @Failure      400 {object} models.ErrorResponse "Invalid id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs settings.manage)"
// @Failure      404 {object} models.ErrorResponse "Option rate not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/option-rates/{id} [put]
func (h *Handler) AdminUpdateOptionRate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid option rate id"})
	}
	var req models.UpdateOptionRateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	o, err := h.service.UpdateOptionRate(c.Context(), id, req)
	if err != nil {
		return mapOptionRateErr(c, err, "AdminUpdateOptionRate")
	}
	return c.Status(fiber.StatusOK).JSON(o)
}

// AdminDeleteOptionRate: DELETE /admin/itineraries/option-rates/:id (settings.manage).
//
// @Summary      Delete an option rate (admin)
// @Description  Removes a rate from the price book. Access: super_admin (settings.manage).
// @Tags         admin,itineraries,option-rates
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Option rate ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid option rate id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs settings.manage)"
// @Failure      404 {object} models.ErrorResponse "Option rate not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/option-rates/{id} [delete]
func (h *Handler) AdminDeleteOptionRate(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid option rate id"})
	}
	if err := h.service.DeleteOptionRate(c.Context(), id); err != nil {
		return mapOptionRateErr(c, err, "AdminDeleteOptionRate")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// mapOptionRateErr maps service errors for the option-rate CRUD. The UNIQUE
// violation on create (duplicate option_key) surfaces as a 409.
func mapOptionRateErr(c *fiber.Ctx, err error, op string) error {
	switch {
	case errors.Is(err, models.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Option rate not found"})
	case errors.Is(err, models.ErrInvalidOperation):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	default:
		// A duplicate option_key surfaces as a pgx unique-violation wrapped error;
		// detect by the constraint name so the planner/operator gets a clear 409.
		if strings.Contains(err.Error(), "option_rates_option_key_key") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "An option rate with that option_key already exists"})
		}
		log.Printf("Handler.%s: %v", op, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to save option rate"})
	}
}

// AdminSendQuote: POST /admin/itineraries/:id/quote
//
// @Summary      Send a quote to the customer
// @Description  Builds + sends a quote (line items priced in CNY; presentment + deposit
// @Description  FX-snapshotted at send time). Re-sending replaces the prior quote via
// @Description  ON CONFLICT (only the latest is payable). Enqueues a PDF render.
// @Description  Access: travel_planner (itinerary.write).
// @Tags         admin,itineraries,quotes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.SendQuoteRequest true "Quote line items + deposit toggle + valid_until"
// @Success      201 {object} models.ItineraryQuote
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/quote [post]
func (h *Handler) AdminSendQuote(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.SendQuoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	q, err := h.service.SendQuote(c.Context(), id, req)
	if err != nil {
		return mapItinAdminErr(c, err, "AdminSendQuote")
	}
	return c.Status(fiber.StatusCreated).JSON(q)
}

// AdminGetQuote: GET /admin/itineraries/:id/quote
//
// @Summary      Get the quote for an itinerary request (planner)
// @Description  Fetches the quote for a request (planner view; no ownership check).
// @Description  Access: travel_planner, customer_service.
// @Tags         admin,itineraries,quotes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      200 {object} models.ItineraryQuote
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / no quote"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/quote [get]
func (h *Handler) AdminGetQuote(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	q, err := h.service.GetQuote(c.Context(), id)
	if err != nil {
		return mapItinAdminErr(c, err, "AdminGetQuote")
	}
	return c.Status(fiber.StatusOK).JSON(q)
}

// QuotePDFDownload: GET /itineraries/:id/quote/pdf (signed-in customer).
// Serves the pre-rendered itinerary-quote PDF. Ownership-checked (mirrors
// GetQuote). 404 "PDF not yet generated" when pdf_key is NULL (local mode or
// render pending). ?download=1 forces a Content-Disposition: attachment.
//
// @Summary      Download the itinerary quote PDF (customer)
// @Description  Serves the pre-rendered quote PDF (302 redirect to the storage/CDN URL).
// @Description  Ownership-checked. 404 when the PDF has not yet been generated.
// @Tags         itineraries,quotes
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id       path int true "Itinerary request ID"
// @Param        download query int false "1 = force Content-Disposition: attachment"
// @Success      302 {string} string "Redirect to the PDF storage URL"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / PDF not yet generated"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /itineraries/{id}/quote/pdf [get]
func (h *Handler) QuotePDFDownload(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	// Verify ownership (GetMine scopes to the user) before reading the quote.
	if _, err := h.service.GetMine(c.Context(), userID, id); err != nil {
		return mapItinAdminErr(c, err, "QuotePDFDownload.Ownership")
	}
	q, err := h.service.GetQuote(c.Context(), id)
	if err != nil {
		return mapItinAdminErr(c, err, "QuotePDFDownload")
	}
	return h.serveQuotePDF(c, q)
}

// AdminQuotePDFDownload: GET /admin/itineraries/:id/quote/pdf (planner).
// No ownership check (planner scope). Same 404 + ?download=1 behavior.
//
// @Summary      Download the itinerary quote PDF (planner)
// @Description  Serves the pre-rendered quote PDF (planner view; no ownership check).
// @Description  404 when the PDF has not yet been generated.
// @Tags         admin,itineraries,quotes
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id       path int true "Itinerary request ID"
// @Param        download query int false "1 = force Content-Disposition: attachment"
// @Success      302 {string} string "Redirect to the PDF storage URL"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.read)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / PDF not yet generated"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/quote/pdf [get]
func (h *Handler) AdminQuotePDFDownload(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	q, err := h.service.GetQuote(c.Context(), id)
	if err != nil {
		return mapItinAdminErr(c, err, "AdminQuotePDFDownload")
	}
	return h.serveQuotePDF(c, q)
}

// serveQuotePDF resolves a quote's pdf_key to its public URL + redirects. The
// filename mirrors the cert convention: itinerary-<request_id>.pdf.
func (h *Handler) serveQuotePDF(c *fiber.Ctx, q *models.ItineraryQuote) error {
	if q.PDFKey == nil || *q.PDFKey == "" {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "PDF not yet generated"})
	}
	url := h.store.PublicURL(*q.PDFKey)
	fname := "itinerary-" + strconv.FormatInt(q.RequestID, 10) + ".pdf"
	if c.Query("download") == "1" {
		c.Set("Content-Disposition", `attachment; filename="`+fname+`"`)
	} else {
		c.Set("Content-Disposition", `inline; filename="`+fname+`"`)
	}
	return c.Redirect(url, fiber.StatusFound)
}

// AdminConfirm: POST /admin/itineraries/:id/confirm (deposit_paid → confirmed)
//
// @Summary      Confirm an itinerary request (deposit_paid → confirmed)
// @Description  Transitions a deposit-paid request to confirmed. Access: travel_planner.
// @Tags         admin,itineraries
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Itinerary request ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/confirm [post]
func (h *Handler) AdminConfirm(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	if err := h.service.Confirm(c.Context(), id); err != nil {
		return mapItinAdminErr(c, err, "AdminConfirm")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AdminRefundDeposit: POST /admin/itineraries/:id/refund-deposit (fail-closed)
//
// @Summary      Refund an itinerary deposit (fail-closed)
// @Description  Issues a full refund of the deposit via the gateway (fail-closed:
// @Description  gateway.Refund is called BEFORE the status transition). Body {reason} optional.
// @Description  Access: travel_planner (itinerary.write).
// @Tags         admin,itineraries,quotes
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Itinerary request ID"
// @Param        body body models.RefundDepositRequest false "Optional refund reason"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid itinerary id / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs itinerary.write)"
// @Failure      404 {object} models.ErrorResponse "Itinerary request not found / no quote"
// @Failure      409 {object} models.ErrorResponse "Request is not in a valid state for this operation"
// @Failure      500 {object} models.ErrorResponse "Internal error (gateway refund failed)"
// @Security     BearerAuth
// @Router       /admin/itineraries/{id}/refund-deposit [post]
func (h *Handler) AdminRefundDeposit(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid itinerary id"})
	}
	var req models.RefundDepositRequest
	_ = c.BodyParser(&req) // optional
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.RefundDeposit(c.Context(), id, req); err != nil {
		return mapItinAdminErr(c, err, "AdminRefundDeposit")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
