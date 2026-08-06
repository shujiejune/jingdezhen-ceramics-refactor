package shipping

import (
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles shipping endpoints (PRD §3.2.3).
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

// Quote: GET /shipping/quote?country=US&weight=2500 (public preview, TDD §5.2)
//
//	@Summary		Get a shipping quote
//	@Description	Public preview of the shipping fee for a country + packed weight,
//	@Description	computed from the shipping_fee_tiers table. Returns shippable=false
//	@Description	(with a reason) for unshippable countries or overweight packages.
//	@Tags			shipping
//	@Accept			json
//	@Produce		json
//	@Param			country	query		string	true	"Destination country (ISO 3166-1 alpha-2)"
//	@Param			weight	query		int		true	"Packed weight in grams"
//	@Success		200		{object}	models.ShippingQuoteResponse
//	@Failure		400		{object}	models.ErrorResponse	"Invalid country (must be 2 letters) / weight (must be non-negative grams)"
//	@Failure		500		{object}	models.ErrorResponse	"Internal error"
//	@Router			/shipping/quote [get]
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
//
//	@Summary		List shipping fee tiers (admin)
//	@Description	Returns all shipping_fee_tiers rows (the shipping-calculator config).
//	@Description	Access: super_admin (settings.manage).
//	@Tags			admin,shipping
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Success		200				{array}		models.ShippingFeeTier
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs settings.manage)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/shipping/tiers [get]
func (h *Handler) ListTiers(c *fiber.Ctx) error {
	tiers, err := h.service.ListAll(c.Context())
	if err != nil {
		log.Printf("Handler.ListTiers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list shipping tiers"})
	}
	return c.Status(fiber.StatusOK).JSON(tiers)
}

// CreateTier: POST /admin/shipping/tiers
//
//	@Summary		Create a shipping fee tier (admin)
//	@Description	Adds a (country, max_weight_grams) → fee_cny tier. Access: super_admin.
//	@Tags			admin,shipping
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string								true	"Bearer <access_token>"
//	@Param			body			body		models.CreateShippingTierRequest	true	"Tier to create"
//	@Success		201				{object}	models.ShippingFeeTier
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs settings.manage)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/shipping/tiers [post]
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
//
//	@Summary		Update a shipping fee tier (admin)
//	@Description	Replaces the (country, max_weight_grams, fee_cny) of a tier.
//	@Description	Access: super_admin (settings.manage).
//	@Tags			admin,shipping
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string								true	"Bearer <access_token>"
//	@Param			id				path		int									true	"Tier ID"
//	@Param			body			body		models.UpdateShippingTierRequest	true	"Full replacement of tier fields"
//	@Success		200				{object}	models.ShippingFeeTier
//	@Failure		400				{object}	models.ErrorResponse	"Invalid tier ID / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs settings.manage)"
//	@Failure		404				{object}	models.ErrorResponse	"Shipping tier not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/shipping/tiers/{id} [put]
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
//
//	@Summary		Delete a shipping fee tier (admin)
//	@Description	Removes a shipping tier. Access: super_admin (settings.manage).
//	@Tags			admin,shipping
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Tier ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid tier ID"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs settings.manage)"
//	@Failure		404				{object}	models.ErrorResponse	"Shipping tier not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/shipping/tiers/{id} [delete]
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
	h.audit.Log(c, models.AuditActionShippingTierDelete, models.AuditEntityShippingFeeTier, strconv.FormatInt(id, 10), nil)
	return c.SendStatus(fiber.StatusNoContent)
}
