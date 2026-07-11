package product

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the product catalog (PRD §3.2.1).
type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// requestLocale returns the locale from ?locale= query param, falling back to
// Accept-Language header. TDD §5.1: ?locale= overrides Accept-Language.
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

// --- Public reads ---

// GetProducts: GET /catalog/products?locale=en-US&category=&artist=&page=&limit=
func (h *Handler) GetProducts(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	category := c.Query("category")
	artistID, _ := strconv.ParseInt(c.Query("artist"), 10, 64)
	products, total, err := h.service.GetProducts(c.Context(), locale, category, artistID, page, limit)
	if err != nil {
		log.Printf("Handler.GetProducts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve products"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(products, page, limit, total))
}

// GetProductBySlug: GET /catalog/products/:slug?locale=en-US
func (h *Handler) GetProductBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Product slug parameter is required"})
	}
	locale := requestLocale(c)
	product, err := h.service.GetProductBySlug(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Product not found"})
		}
		log.Printf("Handler.GetProductBySlug: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve product"})
	}
	return c.Status(fiber.StatusOK).JSON(product)
}

// =============================================================================
// Admin / CMS handlers — products (PRD §3.1.1 editorial workflow)
// =============================================================================

func adminActor(c *fiber.Ctx) i18ncontent.WorkflowActor {
	roles, _ := c.Locals("userRoles").([]string)
	for _, r := range roles {
		if r == models.RoleSuperAdmin {
			return i18ncontent.ActorSuperAdmin
		}
	}
	return i18ncontent.ActorEditor
}

type localeBody struct {
	Locale string `json:"locale" validate:"required,len=5"`
}

// AdminListProducts: GET /admin/products?locale=&status=&page=&limit=
func (h *Handler) AdminListProducts(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := c.Query("locale")
	status := c.Query("status")
	products, total, err := h.service.AdminListProducts(c.Context(), locale, status, page, limit)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminListProducts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list products"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(products, page, limit, total))
}

// AdminGetProduct: GET /admin/products/:slug?locale=en-US (any status)
func (h *Handler) AdminGetProduct(c *fiber.Ctx) error {
	slug := c.Params("slug")
	locale := c.Query("locale", models.DefaultLocale)
	product, err := h.service.AdminGetProduct(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Product not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminGetProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve product"})
	}
	return c.Status(fiber.StatusOK).JSON(product)
}

// AdminCreateProduct: POST /admin/products
func (h *Handler) AdminCreateProduct(c *fiber.Ctx) error {
	var req models.CreateProductData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	product, err := h.service.AdminCreateProduct(c.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.AdminCreateProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create product"})
	}
	return c.Status(fiber.StatusCreated).JSON(product)
}

// AdminUpdateProduct: PUT /admin/products/:id?locale=en-US
func (h *Handler) AdminUpdateProduct(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product ID"})
	}
	locale := c.Query("locale", models.DefaultLocale)
	var req models.UpdateProductData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	product, err := h.service.AdminUpdateProduct(c.Context(), id, locale, req, adminActor(c))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Product not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "This translation is not editable in its current workflow state"})
		}
		log.Printf("Handler.AdminUpdateProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update product"})
	}
	return c.Status(fiber.StatusOK).JSON(product)
}

func (h *Handler) adminTransition(c *fiber.Ctx, to models.ContentStatus) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product ID"})
	}
	var body localeBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	reviewerID, _ := utils.GetUserIDFromContext(c)
	product, err := h.service.AdminTransitionProduct(c.Context(), id, body.Locale, to, adminActor(c), reviewerID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Product not found"})
		}
		if errors.Is(err, models.ErrInvalidLocale) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
		if errors.Is(err, models.ErrInvalidWorkflowTransition) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: err.Error()})
		}
		log.Printf("Handler.adminTransition: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to transition product"})
	}
	return c.Status(fiber.StatusOK).JSON(product)
}

// AdminSubmitProduct: POST /admin/products/:id/submit (draft → in_review)
func (h *Handler) AdminSubmitProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusInReview)
}

// AdminApproveProduct: POST /admin/products/:id/approve (in_review → published)
func (h *Handler) AdminApproveProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusPublished)
}

// AdminRejectProduct: POST /admin/products/:id/reject (in_review → rejected)
func (h *Handler) AdminRejectProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusRejected)
}

// AdminUnpublishProduct: POST /admin/products/:id/unpublish (published → draft)
func (h *Handler) AdminUnpublishProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusDraft)
}

// AdminDeleteProduct: DELETE /admin/products/:id
func (h *Handler) AdminDeleteProduct(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product ID"})
	}
	if err := h.service.AdminDeleteProduct(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Product not found"})
		}
		log.Printf("Handler.AdminDeleteProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete product"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// =============================================================================
// Admin / CMS handlers — SKUs (PRD §3.2.1)
// =============================================================================

// AdminCreateSKU: POST /admin/products/:id/skus
func (h *Handler) AdminCreateSKU(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product ID"})
	}
	var req models.CreateSKUData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}
	sku, err := h.service.AdminCreateSKU(c.Context(), productID, req)
	if err != nil {
		log.Printf("Handler.AdminCreateSKU: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create SKU"})
	}
	return c.Status(fiber.StatusCreated).JSON(sku)
}

// AdminUpdateSKU: PUT /admin/skus/:id
func (h *Handler) AdminUpdateSKU(c *fiber.Ctx) error {
	skuID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid SKU ID"})
	}
	var req models.UpdateSKUData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	sku, err := h.service.AdminUpdateSKU(c.Context(), skuID, req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "SKU not found"})
		}
		log.Printf("Handler.AdminUpdateSKU: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update SKU"})
	}
	return c.Status(fiber.StatusOK).JSON(sku)
}

// AdminDeleteSKU: DELETE /admin/skus/:id
func (h *Handler) AdminDeleteSKU(c *fiber.Ctx) error {
	skuID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid SKU ID"})
	}
	if err := h.service.AdminDeleteSKU(c.Context(), skuID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "SKU not found"})
		}
		log.Printf("Handler.AdminDeleteSKU: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete SKU"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
