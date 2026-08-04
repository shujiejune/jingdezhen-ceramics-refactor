package itinerary

import (
	"encoding/csv"
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the itinerary wizard (PRD §3.3.2, TDD §5.2).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
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
func (h *Handler) AdminListPlanners(c *fiber.Ctx) error {
	planners, err := h.service.ListPlanners(c.Context())
	if err != nil {
		log.Printf("Handler.AdminListPlanners: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list planners"})
	}
	return c.Status(fiber.StatusOK).JSON(planners)
}

// AdminOpen: POST /admin/itineraries/:id/open (pending → processing)
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
