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
//
// @Summary      List the current user's shipping addresses
// @Description  Returns all shipping addresses for the signed-in user.
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {object} object "{data: []models.UserAddress}"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses [get]
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
//
// @Summary      Create a shipping address
// @Description  Adds a shipping address to the signed-in user's address book.
// @Description  Set is_default=true to also make it the default.
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.CreateAddressRequest true "Address to create (country is ISO 3166-1 alpha-2)"
// @Success      201 {object} models.UserAddress
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses [post]
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
//
// @Summary      Get one shipping address
// @Description  Fetches one of the signed-in user's addresses by ID. An address
// @Description  not owned by the user returns 404 (no cross-user access).
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Address ID"
// @Success      200 {object} models.UserAddress
// @Failure      400 {object} models.ErrorResponse "Invalid address ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Address not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses/{id} [get]
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
//
// @Summary      Replace a shipping address (PUT semantics)
// @Description  Replaces all editable fields of an address. is_default is honoured
// @Description  here too (so a full-replace client can set it in one call), though
// @Description  it's also manageable via the set-default endpoint.
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Address ID"
// @Param        body body models.UpdateAddressRequest true "Full replacement of editable fields"
// @Success      200 {object} models.UserAddress
// @Failure      400 {object} models.ErrorResponse "Invalid address ID / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Address not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses/{id} [put]
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
//
// @Summary      Delete a shipping address
// @Description  Removes one of the signed-in user's addresses.
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Address ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid address ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Address not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses/{id} [delete]
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
//
// @Summary      Set the default shipping address
// @Description  Marks one of the signed-in user's addresses as the default
// @Description  (used as the checkout prefill).
// @Tags         profile,addresses
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Address ID"
// @Success      200 {object} object "{message: \"Default address updated\"}"
// @Failure      400 {object} models.ErrorResponse "Invalid address ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Address not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /profile/addresses/{id}/default [post]
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
