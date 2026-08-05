package product

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// PriceConverter converts a CNY minor-unit price to a presentment currency.
// Implemented by platform/fx.Service; nil means no conversion (backward-compat).
type PriceConverter interface {
	Convert(ctx context.Context, cnyMinor int64, currency string) (int64, error)
}

// Handler handles HTTP requests for the product catalog (PRD §3.2.1).
type Handler struct {
	service        ServiceInterface
	validate       *validator.Validate
	priceConverter PriceConverter // optional; nil => no ?currency= conversion
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service, validate: validator.New()}
}

// SetPriceConverter injects the FX converter (called in main.go after both
// the product and FX services are built). Kept separate from NewHandler so the
// converter stays optional (tests + modules that don't need FX skip it).
func (h *Handler) SetPriceConverter(pc PriceConverter) {
	h.priceConverter = pc
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

// GetProducts: GET /catalog/products?locale=en-US&category=&artist=&tag=&page=&limit=
// `tag` is a comma-separated list of canonical tag keys (ANY-match).
//
// @Summary      List published products
// @Description  Paginated list of published products in the requested locale.
// @Description  Supports filtering by category, artist, and tags (ANY-match).
// @Description  Locale resolution: ?locale= overrides Accept-Language.
// @Tags         catalog,products
// @Accept       json
// @Produce      json
// @Param        locale   query string            false "BCP 47 locale (e.g. en-US). Overrides Accept-Language." default("en-US")
// @Param        category query string            false "Category name (exact match)"
// @Param        artist   query int               false "Artist ID"
// @Param        tag      query string            false "Comma-separated canonical tag keys (ANY-match, e.g. hand-painted,celadon-glaze)"
// @Param        page     query int               false "Page number (1-based)" default(1)
// @Param        limit    query int               false "Page size (max 100)" default(20)
// @Success      200      {object} models.PaginatedResponse{data=[]models.Product}
// @Failure      500      {object} models.ErrorResponse "Internal error"
// @Router       /catalog/products [get]
func (h *Handler) GetProducts(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	category := c.Query("category")
	artistID, _ := strconv.ParseInt(c.Query("artist"), 10, 64)
	tags := parseTagKeys(c.Query("tag"))
	products, total, err := h.service.GetProducts(c.Context(), locale, category, artistID, tags, page, limit)
	if err != nil {
		log.Printf("Handler.GetProducts: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve products"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(products, page, limit, total))
}

// GetProductBySlug: GET /catalog/products/:slug?locale=en-US&currency=USD
// When ?currency= is a supported presentment currency (USD/EUR/GBP) and an FX
// converter is wired, each SKU's response gains `price` + `price_currency`
// (presentment minor units, PRD rounding applied). price_cny is always present.
//
// @Summary      Get a product by slug
// @Description  Fetches a single published product by its locale-specific slug,
// @Description  with SKUs, gallery, and tags loaded. Optional ?currency= adds
// @Description  presentment pricing to each SKU (USD/EUR/GBP; FX-snapshotted).
// @Tags         catalog,products
// @Accept       json
// @Produce      json
// @Param        slug     path string  true "Product slug (locale-specific)"
// @Param        locale   query string false "BCP 47 locale (e.g. en-US). Overrides Accept-Language." default("en-US")
// @Param        currency query string false "Presentment currency (USD/EUR/GBP). Adds `price` + `price_currency` to each SKU."
// @Success      200      {object} models.Product
// @Failure      400      {object} models.ErrorResponse "Missing slug"
// @Failure      404      {object} models.ErrorResponse "Product not found (or not published in this locale)"
// @Failure      500      {object} models.ErrorResponse "Internal error"
// @Router       /catalog/products/{slug} [get]
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

	// Optional presentment-currency conversion (TDD §7). Failures here degrade
	// gracefully: log + return the product with CNY-only pricing rather than a
	// 500, so the catalog stays browseable when FX rates are stale/missing.
	if cur := c.Query("currency"); cur != "" && h.priceConverter != nil && isSupportedCurrency(cur) {
		for i := range product.SKUs {
			p, convErr := h.priceConverter.Convert(c.Context(), product.SKUs[i].PriceCNY, cur)
			if convErr != nil {
				log.Printf("Handler.GetProductBySlug.Convert(sku=%d): %v", product.SKUs[i].ID, convErr)
				continue
			}
			product.SKUs[i].Price = &p
			product.SKUs[i].PriceCurrency = &cur
		}
	}

	return c.Status(fiber.StatusOK).JSON(product)
}

// isSupportedCurrency checks the presentment-currency set (USD/EUR/GBP).
// Duplicated locally to avoid importing platform/fx from the product module
// (which would create a layering smell: a feature module reaching into a
// platform package for a constant). The set is fixed for MVP (PRD §3.2.3).
func isSupportedCurrency(code string) bool {
	switch code {
	case "USD", "EUR", "GBP":
		return true
	}
	return false
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

// AdminListProducts: GET /admin/products?locale=&status=&tag=&page=&limit=
// `tag` is a comma-separated list of canonical tag keys (ANY-match).
// AdminListProducts: GET /admin/products?locale=&status=&tag=&page=&limit=
// `tag` is a comma-separated list of canonical tag keys (ANY-match).
//
// @Summary      List products (admin, any status)
// @Description  Paginated list of products filtered by locale, status, and tags.
// @Description  Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        locale query string false "BCP 47 locale"
// @Param        status query string false "Filter by workflow status (draft|in_review|published|rejected)"
// @Param        tag    query string false "Comma-separated canonical tag keys (ANY-match)"
// @Param        page   query int    false "Page number (1-based)" default(1)
// @Param        limit  query int    false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.Product}
// @Failure      400 {object} models.ErrorResponse "Invalid locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products [get]
func (h *Handler) AdminListProducts(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := c.Query("locale")
	status := c.Query("status")
	tags := parseTagKeys(c.Query("tag"))
	products, total, err := h.service.AdminListProducts(c.Context(), locale, status, tags, page, limit)
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
//
// @Summary      Get a product by slug (admin, any status)
// @Description  Fetches a single product by slug in any workflow status.
// @Description  Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        slug   path string  true "Product slug"
// @Param        locale query string false "BCP 47 locale" default(en-US)
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{slug} [get]
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
//
// @Summary      Create a product
// @Description  Creates a product + optional first SKU. Locale defaults to en-US.
// @Description  A certificate is auto-issued (fail-soft). Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.CreateProductData true "Product to create"
// @Success      201 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products [post]
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

// AdminBulkImport: POST /admin/products/import (multipart "file" CSV, or raw CSV body)
// PRD §3.4.1 line 175: bulk upload CSV import. One product per row (+ optional
// first SKU). Returns a per-row summary (imported / failed / errors).
//
// @Summary      Bulk-import products via CSV
// @Description  Imports products (+ optional first SKU each) from a CSV file
// @Description  (multipart "file" upload, or raw CSV body). Returns a per-row
// @Description  summary (imported / failed / errors). Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        file formData file false "CSV file (multipart upload)"
// @Success      200 {object} object "{imported: int, failed: int, errors: []string}"
// @Failure      400 {object} models.ErrorResponse "Invalid CSV / no rows"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/import [post]
func (h *Handler) AdminBulkImport(c *fiber.Ctx) error {
	var reader io.Reader
	// Prefer a multipart "file" upload; fall back to raw CSV body.
	if fh, err := c.FormFile("file"); err == nil {
		f, err := fh.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Failed to open upload"})
		}
		defer f.Close()
		reader = f
	} else if body := c.Body(); len(body) > 0 {
		reader = strings.NewReader(string(body))
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Missing CSV file (multipart 'file' field or raw body)"})
	}

	rows, err := parseCSV(reader)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "CSV parse: " + err.Error()})
	}
	if len(rows) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "CSV has no data rows"})
	}
	summary, err := h.service.AdminBulkImport(c.Context(), rows)
	if err != nil {
		log.Printf("Handler.AdminBulkImport: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Import failed"})
	}
	return c.Status(fiber.StatusOK).JSON(summary)
}

// parseCSVTags splits a semicolon-separated CSV cell into normalized tag keys
// (lowercase, trimmed; empty entries dropped). Returns nil when the cell is
// empty — the repo treats nil as "no tags to set".
func parseCSVTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	out := []string{}
	for _, p := range parts {
		k := strings.ToLower(strings.TrimSpace(p))
		if k != "" {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseCSV maps CSV columns → BulkImportRow. Header row required; columns:
// title,slug,category,artist_id,thumbnail_url,display_order,description,locale,
// sku_code,price_cny,stock,weight_grams,low_stock_threshold,attributes,tags
func parseCSV(r io.Reader) ([]models.BulkImportRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	out := []models.BulkImportRow{}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := models.BulkImportRow{}
		get := func(col string) string {
			if i, ok := idx[col]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		row.Title = get("title")
		row.Slug = get("slug")
		row.Category = get("category")
		if v := get("artist_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				row.ArtistID = &id
			}
		}
		row.ThumbnailURL = get("thumbnail_url")
		if v := get("display_order"); v != "" {
			n, _ := strconv.Atoi(v)
			row.DisplayOrder = n
		}
		row.Description = get("description")
		row.Locale = get("locale")
		row.SKUCode = get("sku_code")
		if v := get("price_cny"); v != "" {
			n, _ := strconv.ParseInt(v, 10, 64)
			row.PriceCNY = n
		}
		if v := get("stock"); v != "" {
			n, _ := strconv.Atoi(v)
			row.Stock = n
		}
		if v := get("weight_grams"); v != "" {
			n, _ := strconv.Atoi(v)
			row.WeightGrams = n
		}
		if v := get("low_stock_threshold"); v != "" {
			n, _ := strconv.Atoi(v)
			row.LowStockThreshold = &n
		}
		row.Attributes = get("attributes")
		// Tags: semicolon-separated canonical keys within the cell (e.g.
		// `hand-painted;cobalt-blue`). Empty → no tags.
		row.Tags = parseCSVTags(get("tags"))
		out = append(out, row)
	}
	return out, nil
}

// AdminUpdateProduct: PUT /admin/products/:id?locale=en-US
// AdminUpdateProduct: PUT /admin/products/:id?locale=en-US
//
// @Summary      Update a product
// @Description  Updates a product's translation + parent fields (nil = unchanged).
// @Description  May return 409 if the translation is not editable in its workflow state.
// @Description  Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id     path int true "Product ID"
// @Param        locale query string false "BCP 47 locale" default(en-US)
// @Param        body   body models.UpdateProductData true "Fields to update (nil pointers = unchanged)"
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      409 {object} models.ErrorResponse "Translation not editable in its current workflow state"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id} [put]
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
//
// @Summary      Submit a product for review
// @Description  Transitions a draft product translation to in_review. Body: {locale}.
// @Description  Access: ecommerce_operator.
// @Tags         admin,products,workflow
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body object true "{locale: en-US}"
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/submit [post]
func (h *Handler) AdminSubmitProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusInReview)
}

// AdminApproveProduct: POST /admin/products/:id/approve (in_review → published)
//
// @Summary      Approve + publish a product
// @Description  Transitions an in_review product translation to published.
// @Description  Access: super_admin ONLY (product.publish). Body: {locale}.
// @Tags         admin,products,workflow
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body object true "{locale: en-US}"
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.publish — super_admin only)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/approve [post]
func (h *Handler) AdminApproveProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusPublished)
}

// AdminRejectProduct: POST /admin/products/:id/reject (in_review → rejected)
//
// @Summary      Reject a product (in_review → rejected)
// @Description  Transitions an in_review product translation to rejected.
// @Description  Access: super_admin ONLY (product.publish). Body: {locale}.
// @Tags         admin,products,workflow
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body object true "{locale: en-US}"
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.publish — super_admin only)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/reject [post]
func (h *Handler) AdminRejectProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusRejected)
}

// AdminUnpublishProduct: POST /admin/products/:id/unpublish (published → draft)
//
// @Summary      Unpublish a product (published → draft)
// @Description  Transitions a published product translation back to draft.
// @Description  Access: super_admin ONLY (product.publish). Body: {locale}.
// @Tags         admin,products,workflow
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body object true "{locale: en-US}"
// @Success      200 {object} models.Product
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / bad locale"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.publish — super_admin only)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      409 {object} models.ErrorResponse "Invalid workflow transition"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/unpublish [post]
func (h *Handler) AdminUnpublishProduct(c *fiber.Ctx) error {
	return h.adminTransition(c, models.StatusDraft)
}

// AdminDeleteProduct: DELETE /admin/products/:id
//
// @Summary      Delete a product
// @Description  Removes a product (parent + all translations + SKUs). Access: ecommerce_operator.
// @Tags         admin,products
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Product ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid product ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "Product not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id} [delete]
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
//
// @Summary      Create a SKU on a product
// @Description  Adds a purchasable variant (price in CNY minor units, stock, weight,
// @Description  attribute map). Access: ecommerce_operator.
// @Tags         admin,products,skus
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body models.CreateSKUData true "SKU to create"
// @Success      201 {object} models.SKU
// @Failure      400 {object} models.ErrorResponse "Invalid product ID / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/skus [post]
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
//
// @Summary      Update a SKU
// @Description  Updates a SKU (nil pointers = unchanged). Access: ecommerce_operator.
// @Tags         admin,skus
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "SKU ID"
// @Param        body body models.UpdateSKUData true "Fields to update (nil pointers = unchanged)"
// @Success      200 {object} models.SKU
// @Failure      400 {object} models.ErrorResponse "Invalid SKU ID / body"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "SKU not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/skus/{id} [put]
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
//
// @Summary      Delete a SKU
// @Description  Removes a SKU. Access: ecommerce_operator.
// @Tags         admin,skus
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "SKU ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid SKU ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "SKU not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/skus/{id} [delete]
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

// --- Catalog helpers ---

// GetCategories: GET /catalog/categories
//
// @Summary      List product categories
// @Description  Distinct categories across published products (bare strings for MVP).
// @Tags         catalog,categories
// @Produce      json
// @Success      200  {array}  string
// @Failure      500  {object} models.ErrorResponse "Internal error"
// @Router       /catalog/categories [get]
func (h *Handler) GetCategories(c *fiber.Ctx) error {
	categories, err := h.service.GetCategories(c.Context())
	if err != nil {
		log.Printf("Handler.GetCategories: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve categories"})
	}
	return c.Status(fiber.StatusOK).JSON(categories)
}

// GetTags: GET /catalog/tags?locale=en-US
// Lists tags attached to ≥1 published product, with the locale-resolved display
// name + a product count (public facet list, PRD §3.2.1 line 173).
//
// @Summary      List product tags (public facet)
// @Description  Tags attached to at least one published product, with the
// @Description  locale-resolved display name + a product count.
// @Tags         catalog,tags
// @Produce      json
// @Param        locale query string false "BCP 47 locale (e.g. en-US). Overrides Accept-Language." default("en-US")
// @Success      200  {array}  models.TagWithCount
// @Failure      500  {object} models.ErrorResponse "Internal error"
// @Router       /catalog/tags [get]
func (h *Handler) GetTags(c *fiber.Ctx) error {
	locale := requestLocale(c)
	tags, err := h.service.GetTags(c.Context(), locale)
	if err != nil {
		log.Printf("Handler.GetTags: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve tags"})
	}
	return c.Status(fiber.StatusOK).JSON(tags)
}

// parseTagKeys splits a comma-separated `?tag=` query value into normalized tag
// keys (lowercase, trimmed; empty entries dropped). Returns nil when the param
// is absent or empty — the repo treats nil as "no tag filter".
func parseTagKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, p := range parts {
		k := strings.ToLower(strings.TrimSpace(p))
		if k != "" {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
