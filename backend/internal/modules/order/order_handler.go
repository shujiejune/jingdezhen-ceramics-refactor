package order

import (
	"context"
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"
	"jingdezhen-ceramics-backend/pkg/utils"

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
	audit    *audit.Helper
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// SetAuditLogger injects the audit logger (PRD §3.1.1). Nil = no-op (tests).
func (h *Handler) SetAuditLogger(l audit.Logger) { h.audit = audit.NewHelper(l) }

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
//
//	@Summary		Place an order from the cart
//	@Description	Converts the signed-in user's cart into an order: authoritative
//	@Description	atomic stock decrement, FX-snapshot of presentment + CNY totals,
//	@Description	address snapshot, shipping calculation. Returns the created
//	@Description	order (with a hosted checkout URL in sandbox/live mode; empty
//	@Description	in mock mode where the order auto-succeeds).
//	@Description	Currency defaults to the user's preferred_currency (or USD).
//	@Description	Gateway required in sandbox/live (airwallex|paypal); ignored in mock.
//	@Tags			orders,checkout
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			body			body		models.CheckoutRequest	true	"Checkout details (address must belong to the user)"
//	@Success		201				{object}	models.Order
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation / empty cart / unshippable / overweight / unsupported currency"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		404				{object}	models.ErrorResponse	"Address not found"
//	@Failure		409				{object}	models.ErrorResponse	"Insufficient stock to place the order"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/checkout [post]
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
//
//	@Summary		List the current user's orders
//	@Description	Paginated list of the signed-in user's orders (header only;
//	@Description	items loaded on detail view).
//	@Tags			orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			page			query		int		false	"Page number (1-based)"	default(1)
//	@Param			limit			query		int		false	"Page size (max 100)"	default(20)
//	@Success		200				{object}	models.PaginatedResponse{data=[]models.Order}
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/orders [get]
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
//
//	@Summary		Get one of the current user's orders
//	@Description	Fetches a single order (with items) owned by the signed-in user.
//	@Description	An order not owned by the user returns 404 (no cross-user access).
//	@Tags			orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Order ID"
//	@Success		200				{object}	models.Order
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found (or not owned by user)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/orders/{id} [get]
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
//
//	@Summary		Cancel an unpaid order
//	@Description	Cancels the signed-in user's order. Only unpaid (status=created)
//	@Description	orders can be cancelled; any other status returns 409.
//	@Description	Body {reason} is optional.
//	@Tags			orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Order ID"
//	@Param			body			body	models.CancelOrderRequest	false	"Optional cancel reason"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found (or not owned by user)"
//	@Failure		409				{object}	models.ErrorResponse	"Only unpaid orders can be cancelled"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/orders/{id}/cancel [post]
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

// ListAdmin: GET /admin/orders?status=&page=&limit= (PermOrderRead)
//
//	@Summary		List all orders (admin)
//	@Description	Paginated list of all orders, optionally filtered by status.
//	@Description	Access: ecommerce_operator, customer_service, travel_planner.
//	@Tags			admin,orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			status			query		string	false	"Filter by order status (created|paid|shipped|completed|cancelled|refunded)"
//	@Param			page			query		int		false	"Page number (1-based)"	default(1)
//	@Param			limit			query		int		false	"Page size (max 100)"	default(20)
//	@Success		200				{object}	models.PaginatedResponse{data=[]models.Order}
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs order.read)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/orders [get]
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

// GetAdmin: GET /admin/orders/:id (PermOrderRead)
//
//	@Summary		Get any order (admin)
//	@Description	Fetches a single order (with items) by ID, regardless of owner.
//	@Description	Access: ecommerce_operator, customer_service, travel_planner.
//	@Tags			admin,orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Order ID"
//	@Success		200				{object}	models.Order
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs order.read)"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/orders/{id} [get]
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

// Ship: POST /admin/orders/:id/ship (PermOrderWrite). Operator enters the
// carrier name + tracking number (PRD §3.2.3: no carrier API integration).
//
//	@Summary		Mark an order as shipped
//	@Description	Transitions a paid order to shipped, recording the carrier +
//	@Description	tracking number (manual entry, no carrier API).
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string					true	"Bearer <access_token>"
//	@Param			id				path	int						true	"Order ID"
//	@Param			body			body	models.ShipOrderRequest	true	"Carrier + tracking number"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs order.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found"
//	@Failure		409				{object}	models.ErrorResponse	"Order is not in the expected state (must be paid)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/orders/{id}/ship [post]
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

// Complete: POST /admin/orders/:id/complete (PermOrderWrite).
//
//	@Summary		Mark an order as completed
//	@Description	Transitions a shipped order to completed.
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Order ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs order.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found"
//	@Failure		409				{object}	models.ErrorResponse	"Order is not in the expected state (must be shipped)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/orders/{id}/complete [post]
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

// Refund: POST /admin/orders/:id/refund (PermOrderRefund). Fail-closed:
// the gateway refund is called BEFORE the status transition; a gateway error
// leaves the order paid (PRD §3.2.3).
//
//	@Summary		Refund an order
//	@Description	Issues a full refund via the payment gateway (fail-closed: the
//	@Description	gateway.Refund is called BEFORE the status transition; a gateway
//	@Description	error leaves the order paid). Tiered partial refunds deferred.
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,orders
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Order ID"
//	@Param			body			body	models.RefundOrderRequest	false	"Optional refund reason"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid order ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs order.refund)"
//	@Failure		404				{object}	models.ErrorResponse	"Order not found"
//	@Failure		409				{object}	models.ErrorResponse	"Order is not in the expected state (must be paid)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error (gateway refund failed)"
//	@Security		BearerAuth
//	@Router			/admin/orders/{id}/refund [post]
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
	h.audit.Log(c, models.AuditActionOrderRefund, models.AuditEntityOrder, strconv.FormatInt(id, 10),
		map[string]any{"reason": req.Reason})
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
