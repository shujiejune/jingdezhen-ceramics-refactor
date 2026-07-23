package order

import (
	"context"
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// PriceConverter converts a CNY minor-unit price to a presentment currency, for
// read-time display on GET /orders (order totals are already snapshots, so this
// is only used to enrich the order item presentment if a different currency is
// requested — currently the snapshot currency is authoritative; this is a hook).
type PriceConverter interface {
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
}

// Handler handles order HTTP requests (PRD §3.2.3, TDD §8).
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

// Checkout: POST /checkout (signed-in). Body: {"address_id":1,"currency":"USD"}
func (h *Handler) Checkout(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	o, err := h.service.Checkout(c.Context(), userID, req, requestLocale(c))
	if err != nil {
		return mapCheckoutErr(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(o)
}

// ListMine: GET /orders?page=&limit=
func (h *Handler) ListMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	page, limit := utils.GetPageLimit(c)
	orders, total, err := h.service.ListMine(c.Context(), userID, page, limit)
	if err != nil {
		log.Printf("Handler.ListMine: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list orders"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(orders, page, limit, total))
}

// GetMine: GET /orders/:id
func (h *Handler) GetMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	o, err := h.service.GetMine(c.Context(), userID, id)
	if err != nil {
		return mapOrderErr(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(o)
}

// CancelMine: POST /orders/:id/cancel (customer, before shipment)
func (h *Handler) CancelMine(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	var req models.CancelOrderRequest
	_ = c.BodyParser(&req) // body optional
	o, err := h.service.GetMine(c.Context(), userID, id)
	if err != nil {
		return mapOrderErr(c, err)
	}
	if o.Status != models.StatusCreated {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Only unpaid orders can be cancelled"})
	}
	if err := h.service.Cancel(c.Context(), userID, id, req); err != nil {
		return mapOrderErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Admin ---

func (h *Handler) ListAdmin(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	status := c.Query("status")
	orders, total, err := h.service.ListAdmin(c.Context(), status, page, limit)
	if err != nil {
		log.Printf("Handler.ListAdmin: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list orders"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(orders, page, limit, total))
}

func (h *Handler) GetAdmin(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	o, err := h.service.GetAdmin(c.Context(), id)
	if err != nil {
		return mapOrderErr(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(o)
}

func (h *Handler) Ship(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	var req models.ShipOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.Ship(c.Context(), id, req); err != nil {
		return mapOrderErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Complete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	if err := h.service.Complete(c.Context(), id); err != nil {
		return mapOrderErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Refund(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid order ID"})
	}
	var req models.RefundOrderRequest
	_ = c.BodyParser(&req) // optional
	if err := h.service.Refund(c.Context(), id, req); err != nil {
		return mapOrderErr(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// mapCheckoutErr maps checkout-specific errors.
func mapCheckoutErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, models.ErrCartEmpty):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Cart is empty; add items before checking out"})
	case errors.Is(err, models.ErrUnshippable):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Shipping is not available for the selected destination country"})
	case errors.Is(err, models.ErrOverweight):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Your order exceeds the maximum shipping weight for the destination. Please contact us to arrange personalized shipping."})
	case errors.Is(err, models.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Insufficient stock to place the order"})
	case errors.Is(err, models.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Address not found"})
	case errors.Is(err, models.ErrInvalidOperation):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Unsupported currency"})
	default:
		log.Printf("Handler.Checkout: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to place order"})
	}
}

// mapOrderErr maps order-lifecycle errors.
func mapOrderErr(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, models.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Order not found"})
	case errors.Is(err, models.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Order is not in the expected state for this action"})
	default:
		log.Printf("Handler.Order: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to process order"})
	}
}
