package wishlist

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the wishlist (PRD §3.5).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

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

// GetWishlist: GET /wishlist?locale=en-US&page=&limit=
func (h *Handler) GetWishlist(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	items, total, err := h.service.List(c.Context(), userID, locale, page, limit)
	if err != nil {
		log.Printf("Handler.GetWishlist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve wishlist"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(items, page, limit, total))
}

// AddToWishlist: POST /wishlist (body: {"sku_id": 123})
func (h *Handler) AddToWishlist(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.AddWishlistRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.Add(c.Context(), userID, req.SkuID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "SKU not found"})
		}
		log.Printf("Handler.AddToWishlist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to add to wishlist"})
	}
	return c.SendStatus(fiber.StatusCreated)
}

// RemoveFromWishlist: DELETE /wishlist/:sku_id
func (h *Handler) RemoveFromWishlist(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	skuID, err := strconv.ParseInt(c.Params("sku_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid SKU ID"})
	}
	if err := h.service.Remove(c.Context(), userID, skuID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Wishlist item not found"})
		}
		log.Printf("Handler.RemoveFromWishlist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to remove from wishlist"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
