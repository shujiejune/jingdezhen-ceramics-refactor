package media

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handler handles media-asset + product-gallery HTTP requests (PRD §3.2.1,
// TDD §3.4). Media never flows through the VPS: in OSS mode the browser uploads
// to a presigned URL directly; in local-dev mode the upload handler Put's the
// file server-side via the LocalStore.
type Handler struct {
	service  ServiceInterface
	store    storage.Store // for Mode() check + PresignUpload
	validate *validator.Validate
}

func NewHandler(service ServiceInterface, store storage.Store) *Handler {
	return &Handler{service: service, store: store, validate: validator.New()}
}

// PresignUpload: POST /admin/media/presign
// Body: {kind, mime, size}. Returns a presigned PUT URL (OSS) or, in local
// mode, signals the browser to POST the file to /admin/media/upload instead.
//
// @Summary      Presign a media upload
// @Description  Returns a presigned PUT URL (OSS mode) or a local upload URL
// @Description  (local mode) for the browser to upload a media file to.
// @Description  Access: content_editor (content.write).
// @Tags         admin,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body object true "{kind: image|video, mime: string, size: int}"
// @Success      200 {object} object "{mode: oss|local, upload_url, oss_key, headers, public_url?}"
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/media/presign [post]
func (h *Handler) PresignUpload(c *fiber.Ctx) error {
	var req struct {
		Kind string `json:"kind" validate:"required,oneof=image video"`
		MIME string `json:"mime" validate:"required"`
		Size int64  `json:"size" validate:"gte=0"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}

	// Derive a storage key for the upload (the client hasn't uploaded yet).
	key, err := h.store.Key(storage.Kind(req.Kind), req.MIME)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}

	// Local mode: no real presign — tell the browser to POST to /upload.
	if h.store.Mode() == "local" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"mode":       "local",
			"upload_url": "/admin/media/upload",
			"oss_key":    key,
			"public_url": h.store.PublicURL(key),
			"headers":    map[string]string{"X-Storage-Mode": "local"},
		})
	}

	// OSS mode: return a presigned PUT URL.
	url, headers, err := h.store.PresignUpload(c.Context(), key, req.MIME, req.Size)
	if err != nil {
		log.Printf("Handler.PresignUpload: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to presign upload"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"mode":       "oss",
		"upload_url": url,
		"oss_key":    key,
		"headers":    headers,
	})
}

// UploadLocal: POST /admin/media/upload (multipart "file") — local-dev only.
// Stores the file via Store.Put + returns {oss_key, public_url} so the caller
// can POST /admin/media/assets to register the media_assets row next.
//
// @Summary      Upload a media file (local-dev only)
// @Description  Stores a multipart "file" via the local Store.Put. Returns the
// @Description  {oss_key, public_url, size} so the caller can register the asset.
// @Description  Returns 404 in OSS mode (use presign). Access: content_editor.
// @Tags         admin,media
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        file formData file true "Media file"
// @Param        kind formData string false "image|video (defaults to image)"
// @Success      200 {object} object "{oss_key, public_url, size}"
// @Failure      400 {object} models.ErrorResponse "Invalid kind / missing file"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      404 {object} models.ErrorResponse "Direct upload is local-dev only; use presign in OSS mode"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/media/upload [post]
func (h *Handler) UploadLocal(c *fiber.Ctx) error {
	if h.store.Mode() != "local" {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Direct upload is local-dev only; use presign in OSS mode"})
	}
	kind := c.FormValue("kind", "image")
	if kind != "image" && kind != "video" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid kind"})
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Missing 'file' field"})
	}
	f, err := fh.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to open upload"})
	}
	defer f.Close()

	uploadedBy, _ := utils.GetUserIDFromContext(c)
	ossKey, publicURL, err := h.service.UploadLocal(c.Context(), storage.Kind(kind), fh.Header.Get("Content-Type"), f, ptrStr(uploadedBy))
	if err != nil {
		log.Printf("Handler.UploadLocal: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to store upload"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"oss_key":    ossKey,
		"public_url": publicURL,
		"size":       fh.Size,
	})
}

// RegisterAsset: POST /admin/media/assets — record an uploaded file.
//
// @Summary      Register a media asset
// @Description  Records an already-uploaded file as a media_assets row (after
// @Description  the browser has uploaded via presign or local upload). Access: content_editor.
// @Tags         admin,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        body body models.RegisterAssetData true "Asset metadata (oss_key, mime, kind, dimensions)"
// @Success      201 {object} models.MediaAsset
// @Failure      400 {object} models.ErrorResponse "Invalid body / validation / invalid media kind"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/media/assets [post]
func (h *Handler) RegisterAsset(c *fiber.Ctx) error {
	var data models.RegisterAssetData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}
	uploadedBy, _ := utils.GetUserIDFromContext(c)
	asset, err := h.service.RegisterAsset(c.Context(), data, ptrStr(uploadedBy))
	if err != nil {
		if errors.Is(err, models.ErrInvalidOperation) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid media kind"})
		}
		log.Printf("Handler.RegisterAsset: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to register asset"})
	}
	return c.Status(fiber.StatusCreated).JSON(asset)
}

// ListAssets: GET /admin/media/assets?kind=&page=&limit=
//
// @Summary      List media assets (admin)
// @Description  Paginated list of all media_assets, optionally filtered by kind.
// @Description  Access: content_editor.
// @Tags         admin,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        kind  query string false "Filter by kind (image|video)"
// @Param        page  query int    false "Page number (1-based)" default(1)
// @Param        limit query int    false "Page size (max 100)" default(20)
// @Success      200 {object} object "{data: []models.MediaAsset, page, limit, total}"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/media/assets [get]
func (h *Handler) ListAssets(c *fiber.Ctx) error {
	kind := c.Query("kind")
	page, limit := utils.GetPageLimit(c)
	assets, total, err := h.service.ListAssets(c.Context(), kind, page, limit)
	if err != nil {
		log.Printf("Handler.ListAssets: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list assets"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": assets, "page": page, "limit": limit, "total": total,
	})
}

// DeleteAsset: DELETE /admin/media/assets/:id
//
// @Summary      Delete a media asset (admin)
// @Description  Removes a media_assets row (and the underlying object in OSS mode).
// @Description  Access: content_editor.
// @Tags         admin,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Asset ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs content.write)"
// @Failure      404 {object} models.ErrorResponse "Asset not found"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/media/assets/{id} [delete]
func (h *Handler) DeleteAsset(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid id"})
	}
	if err := h.service.DeleteAsset(c.Context(), id); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Asset not found"})
		}
		log.Printf("Handler.DeleteAsset: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete asset"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Product gallery ---

// ListProductMedia: GET /admin/products/:id/media
//
// @Summary      List a product's media gallery (admin)
// @Description  Returns a product's ordered gallery. Access: ecommerce_operator.
// @Tags         admin,products,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id path int true "Product ID"
// @Success      200 {array} models.ProductMediaItem
// @Failure      400 {object} models.ErrorResponse "Invalid product id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/media [get]
func (h *Handler) ListProductMedia(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product id"})
	}
	items, err := h.service.ListProductMedia(c.Context(), productID)
	if err != nil {
		log.Printf("Handler.ListProductMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list product media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// AttachToProduct: POST /admin/products/:id/media
//
// @Summary      Attach a media asset to a product gallery
// @Description  Attaches a media_assets row to a product's ordered gallery.
// @Description  sort_order defaults to append-last. Access: ecommerce_operator.
// @Tags         admin,products,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body models.AttachMediaData true "media_id + optional sort_order + caption"
// @Success      201 {object} object "{ok: true}"
// @Failure      400 {object} models.ErrorResponse "Invalid product id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/media [post]
func (h *Handler) AttachToProduct(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product id"})
	}
	var data models.AttachMediaData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.AttachToProduct(c.Context(), productID, data.MediaID, data.SortOrder, data.Caption); err != nil {
		log.Printf("Handler.AttachToProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to attach media"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

// DetachFromProduct: DELETE /admin/products/:id/media/:media_id
//
// @Summary      Detach a media asset from a product gallery
// @Description  Removes a media_assets row from a product's gallery (does not delete the asset).
// @Description  Access: ecommerce_operator.
// @Tags         admin,products,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id        path int true "Product ID"
// @Param        media_id  path int true "Media asset ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid product/media id"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      404 {object} models.ErrorResponse "Media not attached to this product"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/media/{media_id} [delete]
func (h *Handler) DetachFromProduct(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product id"})
	}
	mediaID, err := strconv.ParseInt(c.Params("media_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid media id"})
	}
	if err := h.service.DetachFromProduct(c.Context(), productID, mediaID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Media not attached to this product"})
		}
		log.Printf("Handler.DetachFromProduct: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to detach media"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ReorderProductMedia: PATCH /admin/products/:id/media/order
//
// @Summary      Reorder a product's media gallery
// @Description  Sets the sort_order for each gallery entry. Body is an array of
// @Description  {product_media_id, sort_order} pairs. Access: ecommerce_operator.
// @Tags         admin,products,media
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        id   path int true "Product ID"
// @Param        body body []models.ReorderMediaItem true "Ordered list of {product_media_id, sort_order}"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid product id / body / validation"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs product.write)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/products/{id}/media/order [patch]
func (h *Handler) ReorderProductMedia(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product id"})
	}
	var items []models.ReorderMediaItem
	if err := c.BodyParser(&items); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	for _, it := range items {
		if err := h.validate.Struct(it); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
		}
	}
	if err := h.service.ReorderProductMedia(c.Context(), productID, items); err != nil {
		log.Printf("Handler.ReorderProductMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to reorder media"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PublicListProductMedia: GET /catalog/products/:id/media (public, no auth)
// Exposes a product's ordered gallery to the storefront.
//
// @Summary      List a product's media gallery (public)
// @Description  Returns a product's ordered gallery for the storefront. No auth.
// @Tags         catalog,products,media
// @Accept       json
// @Produce      json
// @Param        id path int true "Product ID"
// @Success      200 {array} models.ProductMediaItem
// @Failure      400 {object} models.ErrorResponse "Invalid product id"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Router       /catalog/products/{id}/media [get]
func (h *Handler) PublicListProductMedia(c *fiber.Ctx) error {
	productID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid product id"})
	}
	items, err := h.service.ListProductMedia(c.Context(), productID)
	if err != nil {
		log.Printf("Handler.PublicListProductMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list product media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// ptrStr returns a *string for the userID, or nil if empty.
func ptrStr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
