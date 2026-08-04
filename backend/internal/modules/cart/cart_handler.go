package cart

import (
	"context"
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// PriceConverter converts a CNY minor-unit price to a presentment currency.
// Implemented by platform/fx.Service; re-declared here (same shape as the
// product handler's) to avoid a cross-module import + keep the FX package out
// of this module's dependency graph. nil means no conversion (CNY-only).
type PriceConverter interface {
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
}

// Handler handles HTTP requests for the cart (PRD §3.2.3, TDD §3.4).
type Handler struct {
	service        ServiceInterface
	validate       *validator.Validate
	priceConverter PriceConverter // optional; nil => no ?currency= conversion
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// SetPriceConverter injects the FX converter (called in main.go after both the
// cart + FX services are constructed).
func (h *Handler) SetPriceConverter(pc PriceConverter) {
	h.priceConverter = pc
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

func requestCurrency(c *fiber.Ctx) string {
	return c.Query("currency")
}

// GetCart: GET /cart?locale=en-US&currency=USD
func (h *Handler) GetCart(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	locale := requestLocale(c)
	cart, err := h.service.GetCart(c.Context(), userID, locale)
	if err != nil {
		log.Printf("Handler.GetCart: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve cart"})
	}
	h.applyPresentment(c, cart)
	return c.Status(fiber.StatusOK).JSON(cart)
}

// AddItem: POST /cart/items (body: {"sku_id":1,"qty":2}; qty defaults to 1, additive)
func (h *Handler) AddItem(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.AddCartItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if req.Qty == 0 {
		req.Qty = 1 // default
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.AddItem(c.Context(), userID, req.SkuID, req.Qty); err != nil {
		return mapCartErr(c, err, "add item")
	}
	return c.SendStatus(fiber.StatusCreated)
}

// UpdateItemQty: PATCH /cart/items/:sku_id (body: {"qty":3}; absolute set)
func (h *Handler) UpdateItemQty(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	skuID, err := strconv.ParseInt(c.Params("sku_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid SKU ID"})
	}
	var req models.UpdateCartItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	if err := h.service.SetItemQty(c.Context(), userID, skuID, req.Qty); err != nil {
		return mapCartErr(c, err, "update quantity")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveItem: DELETE /cart/items/:sku_id
func (h *Handler) RemoveItem(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	skuID, err := strconv.ParseInt(c.Params("sku_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid SKU ID"})
	}
	if err := h.service.RemoveItem(c.Context(), userID, skuID); err != nil {
		return mapCartErr(c, err, "remove item")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// BulkRemove: DELETE /cart/items (body: {"sku_ids":[1,2,3]})
func (h *Handler) BulkRemove(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.BulkRemoveRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	removed, err := h.service.BulkRemove(c.Context(), userID, req.SkuIDs)
	if err != nil {
		log.Printf("Handler.BulkRemove: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to remove items"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"removed": removed})
}

// MergeCart: POST /cart/merge (body: {"items":[{"sku_id":1,"qty":2}]}). Merges a
// guest localStorage cart into the server cart on login. Returns the merged cart.
func (h *Handler) MergeCart(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Authentication required"})
	}
	var req models.MergeCartRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	locale := requestLocale(c)
	cart, err := h.service.Merge(c.Context(), userID, req.Items, locale)
	if err != nil {
		log.Printf("Handler.MergeCart: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to merge cart"})
	}
	h.applyPresentment(c, cart)
	return c.Status(fiber.StatusOK).JSON(cart)
}

// applyPresentment enriches the cart + each line with presentment-currency
// amounts when ?currency= is supplied and an FX converter is wired. The cart
// fully reconciles: each line's presentment = unit_presentment × qty, and the
// cart total = Σ line. This matches the checkout snapshot model (TDD §7: order
// total = Σ item unit_price_minor × qty) so the customer pays exactly what the
// displayed unit prices imply — no lump-sum conversion that disagrees with the
// per-line display. On any FX error it degrades gracefully (CNY-only, logged).
func (h *Handler) applyPresentment(c *fiber.Ctx, cart *models.Cart) {
	applyPresentmentConv(c.Context(), requestCurrency(c), h.priceConverter, cart)
}

// applyPresentmentConv is the testable core of applyPresentment (no fiber.Ctx).
// All-or-nothing: presentment fields are only set if every line converts, so a
// mid-cart FX failure leaves the cart CNY-only rather than half-converted.
func applyPresentmentConv(ctx context.Context, cur string, conv PriceConverter, cart *models.Cart) {
	if cur == "" || conv == nil || !isSupportedCurrency(cur) {
		return
	}
	// Convert each unit price (PRD §3.2.3 rounding applies to the unit price),
	// then derive line = unit × qty. Compute into temps first so a mid-loop
	// error leaves the cart CNY-only (all-or-nothing).
	units := make([]int64, len(cart.Items))
	lines := make([]int64, len(cart.Items))
	var sum int64
	for i := range cart.Items {
		unit, err := conv.Convert(ctx, cart.Items[i].UnitPriceCNY, cur)
		if err != nil {
			log.Printf("applyPresentment.Convert(sku=%d): %v", cart.Items[i].SkuID, err)
			return
		}
		line := unit * int64(cart.Items[i].Qty)
		units[i] = unit
		lines[i] = line
		sum += line
	}
	for i := range cart.Items {
		cart.Items[i].UnitPrice = &units[i]
		cart.Items[i].LineTotal = &lines[i]
	}
	curCopy := cur
	cart.Total = &sum
	cart.Currency = &curCopy
}

// isSupportedCurrency checks the presentment-currency set (USD/EUR/GBP).
// Mirrors the product handler's local check to keep this module decoupled from
// the FX package.
func isSupportedCurrency(code string) bool {
	switch code {
	case "USD", "EUR", "GBP":
		return true
	}
	return false
}

// mapCartErr maps service errors to HTTP responses for the mutation endpoints.
func mapCartErr(c *fiber.Ctx, err error, action string) error {
	switch {
	case errors.Is(err, models.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "SKU not in cart or does not exist"})
	case errors.Is(err, models.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Quantity exceeds available stock"})
	case errors.Is(err, models.ErrInvalidOperation):
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Quantity must be at least 1"})
	default:
		log.Printf("Handler.%s: %v", action, err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to " + action})
	}
}
