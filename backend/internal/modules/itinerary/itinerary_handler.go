package itinerary

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

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
