package address

import (
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

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

// ListAddresses: GET /profile/addresses
func (h *Handler) ListAddresses(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	addresses, err := h.service.ListAddresses(c.Context(), userID)
	if err != nil {
		log.Printf("Handler.ListAddresses: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list addresses"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": addresses})
}

// CreateAddress: POST /profile/addresses
func (h *Handler) CreateAddress(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	var req models.CreateAddressRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	address, err := h.service.CreateAddress(c.Context(), userID, req)
	if err != nil {
		log.Printf("Handler.CreateAddress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create address"})
	}
	return c.Status(fiber.StatusCreated).JSON(address)
}

// GetAddress: GET /profile/addresses/:id
func (h *Handler) GetAddress(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid address ID"})
	}
	address, err := h.service.GetAddress(c.Context(), userID, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Address not found"})
		}
		log.Printf("Handler.GetAddress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to get address"})
	}
	return c.Status(fiber.StatusOK).JSON(address)
}

// UpdateAddress: PUT /profile/addresses/:id
func (h *Handler) UpdateAddress(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid address ID"})
	}
	var req models.UpdateAddressRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	address, err := h.service.UpdateAddress(c.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Address not found"})
		}
		log.Printf("Handler.UpdateAddress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update address"})
	}
	return c.Status(fiber.StatusOK).JSON(address)
}

// DeleteAddress: DELETE /profile/addresses/:id
func (h *Handler) DeleteAddress(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid address ID"})
	}
	if err := h.service.DeleteAddress(c.Context(), userID, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Address not found"})
		}
		log.Printf("Handler.DeleteAddress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete address"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// SetDefaultAddress: POST /profile/addresses/:id/default
func (h *Handler) SetDefaultAddress(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid address ID"})
	}
	if err := h.service.SetDefaultAddress(c.Context(), userID, id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Address not found"})
		}
		log.Printf("Handler.SetDefaultAddress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to set default address"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Default address updated"})
}
