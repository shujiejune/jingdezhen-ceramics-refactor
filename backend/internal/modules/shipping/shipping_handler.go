package shipping

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles shipping endpoints (PRD §3.2.3).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// Quote: GET /shipping/quote?country=US&weight=2500 (public preview, TDD §5.2)
func (h *Handler) Quote(c *fiber.Ctx) error {
	country := c.Query("country")
	if len(country) != 2 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "country must be a 2-letter ISO code"})
	}
	weight, err := strconv.Atoi(c.Query("weight", "0"))
	if err != nil || weight < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "weight must be a non-negative integer (grams)"})
	}
	resp, err := h.service.Quote(c.Context(), country, weight)
	if err != nil {
		log.Printf("Handler.ShippingQuote: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to compute shipping quote"})
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// ListTiers: GET /admin/shipping/tiers
func (h *Handler) ListTiers(c *fiber.Ctx) error {
	tiers, err := h.service.ListAll(c.Context())
	if err != nil {
		log.Printf("Handler.ListTiers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list shipping tiers"})
	}
	return c.Status(fiber.StatusOK).JSON(tiers)
}

// CreateTier: POST /admin/shipping/tiers
func (h *Handler) CreateTier(c *fiber.Ctx) error {
	var req models.CreateShippingTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	t, err := h.service.Create(c.Context(), req)
	if err != nil {
		log.Printf("Handler.CreateTier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create shipping tier"})
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

// UpdateTier: PUT /admin/shipping/tiers/:id
func (h *Handler) UpdateTier(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid tier ID"})
	}
	var req models.UpdateShippingTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	t, err := h.service.Update(c.Context(), id, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Shipping tier not found"})
		}
		log.Printf("Handler.UpdateTier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update shipping tier"})
	}
	return c.Status(fiber.StatusOK).JSON(t)
}

// DeleteTier: DELETE /admin/shipping/tiers/:id
func (h *Handler) DeleteTier(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid tier ID"})
	}
	if err := h.service.Delete(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Shipping tier not found"})
		}
		log.Printf("Handler.DeleteTier: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete shipping tier"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
