package media

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/audit"
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
	audit    *audit.Helper
}

func NewHandler(service ServiceInterface, store storage.Store) *Handler {
	return &Handler{service: service, store: store, validate: validator.New()}
}

// SetAuditLogger injects the audit logger (PRD §3.1.1). Nil = no-op (tests).
func (h *Handler) SetAuditLogger(l audit.Logger) { h.audit = audit.NewHelper(l) }

// PresignUpload: POST /admin/media/presign
// Body: {kind, mime, size}. Returns a presigned PUT URL (OSS) or, in local
// mode, signals the browser to POST the file to /admin/media/upload instead.
//
//	@Summary		Presign a media upload
//	@Description	Returns a presigned PUT URL (OSS mode) or a local upload URL
//	@Description	(local mode) for the browser to upload a media file to.
//	@Description	Access: content_editor (content.write).
//	@Tags			admin,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			body			body		object					true	"{kind: image|video, mime: string, size: int}"
//	@Success		200				{object}	object					"{mode: oss|local, upload_url, oss_key, headers, public_url?}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/media/presign [post]
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
//	@Summary		Upload a media file (local-dev only)
//	@Description	Stores a multipart "file" via the local Store.Put. Returns the
//	@Description	{oss_key, public_url, size} so the caller can register the asset.
//	@Description	Returns 404 in OSS mode (use presign). Access: content_editor.
//	@Tags			admin,media
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			file			formData	file					true	"Media file"
//	@Param			kind			formData	string					false	"image|video (defaults to image)"
//	@Success		200				{object}	object					"{oss_key, public_url, size}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid kind / missing file"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Direct upload is local-dev only; use presign in OSS mode"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/media/upload [post]
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
//	@Summary		Register a media asset
//	@Description	Records an already-uploaded file as a media_assets row (after
//	@Description	the browser has uploaded via presign or local upload). Access: content_editor.
//	@Tags			admin,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string						true	"Bearer <access_token>"
//	@Param			body			body		models.RegisterAssetData	true	"Asset metadata (oss_key, mime, kind, dimensions)"
//	@Success		201				{object}	models.MediaAsset
//	@Failure		400				{object}	models.ErrorResponse	"Invalid body / validation / invalid media kind"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/media/assets [post]
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
//	@Summary		List media assets (admin)
//	@Description	Paginated list of all media_assets, optionally filtered by kind.
//	@Description	Access: content_editor.
//	@Tags			admin,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			kind			query		string					false	"Filter by kind (image|video)"
//	@Param			page			query		int						false	"Page number (1-based)"	default(1)
//	@Param			limit			query		int						false	"Page size (max 100)"	default(20)
//	@Success		200				{object}	object					"{data: []models.MediaAsset, page, limit, total}"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/media/assets [get]
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
//	@Summary		Delete a media asset (admin)
//	@Description	Removes a media_assets row (and the underlying object in OSS mode).
//	@Description	Access: content_editor.
//	@Tags			admin,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Asset ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Asset not found"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/media/assets/{id} [delete]
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
	h.audit.Log(c, models.AuditActionMediaDelete, models.AuditEntityMediaAsset, strconv.FormatInt(id, 10), nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Product gallery ---

// ListProductMedia: GET /admin/products/:id/media
//
//	@Summary		List a product's media gallery (admin)
//	@Description	Returns a product's ordered gallery. Access: ecommerce_operator.
//	@Tags			admin,products,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Product ID"
//	@Success		200				{array}		models.ProductMediaItem
//	@Failure		400				{object}	models.ErrorResponse	"Invalid product id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs product.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/products/{id}/media [get]
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
//	@Summary		Attach a media asset to a product gallery
//	@Description	Attaches a media_assets row to a product's ordered gallery.
//	@Description	sort_order defaults to append-last. Access: ecommerce_operator.
//	@Tags			admin,products,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			id				path		int						true	"Product ID"
//	@Param			body			body		models.AttachMediaData	true	"media_id + optional sort_order + caption"
//	@Success		201				{object}	object					"{ok: true}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid product id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs product.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/products/{id}/media [post]
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
//	@Summary		Detach a media asset from a product gallery
//	@Description	Removes a media_assets row from a product's gallery (does not delete the asset).
//	@Description	Access: ecommerce_operator.
//	@Tags			admin,products,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Product ID"
//	@Param			media_id		path	int		true	"Media asset ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid product/media id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs product.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Media not attached to this product"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/products/{id}/media/{media_id} [delete]
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
	h.audit.Log(c, models.AuditActionMediaDelete, models.AuditEntityMediaAsset, strconv.FormatInt(mediaID, 10), map[string]any{"product_id": productID})
	return c.SendStatus(fiber.StatusNoContent)
}

// ReorderProductMedia: PATCH /admin/products/:id/media/order
//
//	@Summary		Reorder a product's media gallery
//	@Description	Sets the sort_order for each gallery entry. Body is an array of
//	@Description	{product_media_id, sort_order} pairs. Access: ecommerce_operator.
//	@Tags			admin,products,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Product ID"
//	@Param			body			body	[]models.ReorderMediaItem	true	"Ordered list of {product_media_id, sort_order}"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid product id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs product.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/products/{id}/media/order [patch]
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
//	@Summary		List a product's media gallery (public)
//	@Description	Returns a product's ordered gallery for the storefront. No auth.
//	@Tags			catalog,products,media
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Product ID"
//	@Success		200	{array}		models.ProductMediaItem
//	@Failure		400	{object}	models.ErrorResponse	"Invalid product id"
//	@Failure		500	{object}	models.ErrorResponse	"Internal error"
//	@Router			/catalog/products/{id}/media [get]
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

// =============================================================================
// Entity galleries: artist / ceramic-story / activity — mirror the product
// gallery handlers. Same DTOs (AttachMediaData / ReorderMediaItem), same audit
// action (media.delete on detach), same RBAC (content.write via the admin
// group). The :id path param is the entity id (matches product's pattern).
// =============================================================================

// --- Artist gallery ---

// ListArtistMedia: GET /admin/artists/:id/media
//
//	@Summary		List an artist's media gallery (admin)
//	@Description	Returns an artist's ordered gallery. Access: content_editor.
//	@Tags			admin,artists,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Artist ID"
//	@Success		200				{array}		models.GalleryItem
//	@Failure		400				{object}	models.ErrorResponse	"Invalid artist id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/artists/{id}/media [get]
func (h *Handler) ListArtistMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist id"})
	}
	items, err := h.service.ListArtistMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.ListArtistMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list artist media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// AttachToArtist: POST /admin/artists/:id/media
//
//	@Summary		Attach a media asset to an artist gallery
//	@Description	Attaches a media_assets row to an artist's ordered gallery.
//	@Description	sort_order defaults to append-last. Access: content_editor.
//	@Tags			admin,artists,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			id				path		int						true	"Artist ID"
//	@Param			body			body		models.AttachMediaData	true	"media_id + optional sort_order + caption"
//	@Success		201				{object}	object					"{ok: true}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid artist id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/artists/{id}/media [post]
func (h *Handler) AttachToArtist(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist id"})
	}
	var data models.AttachMediaData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.AttachToArtist(c.Context(), id, data.MediaID, data.SortOrder, data.Caption); err != nil {
		log.Printf("Handler.AttachToArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to attach media"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

// DetachFromArtist: DELETE /admin/artists/:id/media/:media_id
//
//	@Summary		Detach a media asset from an artist gallery
//	@Description	Removes a media_assets row from an artist's gallery (does not delete the asset).
//	@Description	Access: content_editor.
//	@Tags			admin,artists,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Artist ID"
//	@Param			media_id		path	int		true	"Media asset ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid artist/media id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Media not attached to this artist"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/artists/{id}/media/{media_id} [delete]
func (h *Handler) DetachFromArtist(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist id"})
	}
	mediaID, err := strconv.ParseInt(c.Params("media_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid media id"})
	}
	if err := h.service.DetachFromArtist(c.Context(), id, mediaID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Media not attached to this artist"})
		}
		log.Printf("Handler.DetachFromArtist: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to detach media"})
	}
	h.audit.Log(c, models.AuditActionMediaDelete, models.AuditEntityMediaAsset, strconv.FormatInt(mediaID, 10), map[string]any{"artist_id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

// ReorderArtistMedia: PATCH /admin/artists/:id/media/order
//
//	@Summary		Reorder an artist's media gallery
//	@Description	Sets the sort_order for each gallery entry. Access: content_editor.
//	@Tags			admin,artists,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Artist ID"
//	@Param			body			body	[]models.ReorderMediaItem	true	"Ordered list of {media_id, sort_order}"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid artist id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/artists/{id}/media/order [patch]
func (h *Handler) ReorderArtistMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist id"})
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
	if err := h.service.ReorderArtistMedia(c.Context(), id, items); err != nil {
		log.Printf("Handler.ReorderArtistMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to reorder media"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PublicListArtistMedia: GET /artists/:id/media (public, no auth)
//
//	@Summary		List an artist's media gallery (public)
//	@Description	Returns an artist's ordered gallery for the storefront. No auth.
//	@Tags			artists,media
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Artist ID"
//	@Success		200	{array}		models.GalleryItem
//	@Failure		400	{object}	models.ErrorResponse	"Invalid artist id"
//	@Failure		500	{object}	models.ErrorResponse	"Internal error"
//	@Router			/artists/{id}/media [get]
func (h *Handler) PublicListArtistMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artist id"})
	}
	items, err := h.service.ListArtistMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.PublicListArtistMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list artist media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// --- Ceramic story gallery ---

// ListStoryMedia: GET /admin/ceramicstory/:id/media
//
//	@Summary		List a ceramic story's media gallery (admin)
//	@Description	Returns a story's ordered gallery. Access: content_editor.
//	@Tags			admin,ceramicstory,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Story ID"
//	@Success		200				{array}		models.GalleryItem
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/media [get]
func (h *Handler) ListStoryMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story id"})
	}
	items, err := h.service.ListStoryMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.ListStoryMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list story media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// AttachToStory: POST /admin/ceramicstory/:id/media
//
//	@Summary		Attach a media asset to a ceramic story gallery
//	@Description	Attaches a media_assets row to a story's ordered gallery.
//	@Description	sort_order defaults to append-last. Access: content_editor.
//	@Tags			admin,ceramicstory,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			id				path		int						true	"Story ID"
//	@Param			body			body		models.AttachMediaData	true	"media_id + optional sort_order + caption"
//	@Success		201				{object}	object					"{ok: true}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/media [post]
func (h *Handler) AttachToStory(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story id"})
	}
	var data models.AttachMediaData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.AttachToStory(c.Context(), id, data.MediaID, data.SortOrder, data.Caption); err != nil {
		log.Printf("Handler.AttachToStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to attach media"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

// DetachFromStory: DELETE /admin/ceramicstory/:id/media/:media_id
//
//	@Summary		Detach a media asset from a ceramic story gallery
//	@Description	Removes a media_assets row from a story's gallery. Access: content_editor.
//	@Tags			admin,ceramicstory,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Story ID"
//	@Param			media_id		path	int		true	"Media asset ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story/media id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Media not attached to this story"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/media/{media_id} [delete]
func (h *Handler) DetachFromStory(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story id"})
	}
	mediaID, err := strconv.ParseInt(c.Params("media_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid media id"})
	}
	if err := h.service.DetachFromStory(c.Context(), id, mediaID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Media not attached to this story"})
		}
		log.Printf("Handler.DetachFromStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to detach media"})
	}
	h.audit.Log(c, models.AuditActionMediaDelete, models.AuditEntityMediaAsset, strconv.FormatInt(mediaID, 10), map[string]any{"story_id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

// ReorderStoryMedia: PATCH /admin/ceramicstory/:id/media/order
//
//	@Summary		Reorder a ceramic story's media gallery
//	@Description	Sets the sort_order for each gallery entry. Access: content_editor.
//	@Tags			admin,ceramicstory,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Story ID"
//	@Param			body			body	[]models.ReorderMediaItem	true	"Ordered list of {media_id, sort_order}"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid story id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/ceramicstory/{id}/media/order [patch]
func (h *Handler) ReorderStoryMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story id"})
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
	if err := h.service.ReorderStoryMedia(c.Context(), id, items); err != nil {
		log.Printf("Handler.ReorderStoryMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to reorder media"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PublicListStoryMedia: GET /ceramicstory/:id/media (public, no auth)
//
//	@Summary		List a ceramic story's media gallery (public)
//	@Description	Returns a story's ordered gallery for the storefront. No auth.
//	@Tags			ceramicstory,media
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Story ID"
//	@Success		200	{array}		models.GalleryItem
//	@Failure		400	{object}	models.ErrorResponse	"Invalid story id"
//	@Failure		500	{object}	models.ErrorResponse	"Internal error"
//	@Router			/ceramicstory/{id}/media [get]
func (h *Handler) PublicListStoryMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid story id"})
	}
	items, err := h.service.ListStoryMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.PublicListStoryMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list story media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// --- Activity gallery ---

// ListActivityMedia: GET /admin/engage/:id/media
//
//	@Summary		List an activity's media gallery (admin)
//	@Description	Returns an activity's ordered gallery. Access: content_editor.
//	@Tags			admin,engage,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			id				path		int		true	"Activity ID"
//	@Success		200				{array}		models.GalleryItem
//	@Failure		400				{object}	models.ErrorResponse	"Invalid activity id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/engage/{id}/media [get]
func (h *Handler) ListActivityMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity id"})
	}
	items, err := h.service.ListActivityMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.ListActivityMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list activity media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

// AttachToActivity: POST /admin/engage/:id/media
//
//	@Summary		Attach a media asset to an activity gallery
//	@Description	Attaches a media_assets row to an activity's ordered gallery.
//	@Description	sort_order defaults to append-last. Access: content_editor.
//	@Tags			admin,engage,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string					true	"Bearer <access_token>"
//	@Param			id				path		int						true	"Activity ID"
//	@Param			body			body		models.AttachMediaData	true	"media_id + optional sort_order + caption"
//	@Success		201				{object}	object					"{ok: true}"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid activity id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/engage/{id}/media [post]
func (h *Handler) AttachToActivity(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity id"})
	}
	var data models.AttachMediaData
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.AttachToActivity(c.Context(), id, data.MediaID, data.SortOrder, data.Caption); err != nil {
		log.Printf("Handler.AttachToActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to attach media"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

// DetachFromActivity: DELETE /admin/engage/:id/media/:media_id
//
//	@Summary		Detach a media asset from an activity gallery
//	@Description	Removes a media_assets row from an activity's gallery. Access: content_editor.
//	@Tags			admin,engage,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string	true	"Bearer <access_token>"
//	@Param			id				path	int		true	"Activity ID"
//	@Param			media_id		path	int		true	"Media asset ID"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid activity/media id"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		404				{object}	models.ErrorResponse	"Media not attached to this activity"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/engage/{id}/media/{media_id} [delete]
func (h *Handler) DetachFromActivity(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity id"})
	}
	mediaID, err := strconv.ParseInt(c.Params("media_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid media id"})
	}
	if err := h.service.DetachFromActivity(c.Context(), id, mediaID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Media not attached to this activity"})
		}
		log.Printf("Handler.DetachFromActivity: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to detach media"})
	}
	h.audit.Log(c, models.AuditActionMediaDelete, models.AuditEntityMediaAsset, strconv.FormatInt(mediaID, 10), map[string]any{"activity_id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

// ReorderActivityMedia: PATCH /admin/engage/:id/media/order
//
//	@Summary		Reorder an activity's media gallery
//	@Description	Sets the sort_order for each gallery entry. Access: content_editor.
//	@Tags			admin,engage,media
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string						true	"Bearer <access_token>"
//	@Param			id				path	int							true	"Activity ID"
//	@Param			body			body	[]models.ReorderMediaItem	true	"Ordered list of {media_id, sort_order}"
//	@Success		204				"No Content (empty body)"
//	@Failure		400				{object}	models.ErrorResponse	"Invalid activity id / body / validation"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs content.write)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/engage/{id}/media/order [patch]
func (h *Handler) ReorderActivityMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity id"})
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
	if err := h.service.ReorderActivityMedia(c.Context(), id, items); err != nil {
		log.Printf("Handler.ReorderActivityMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to reorder media"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PublicListActivityMedia: GET /engage/:id/media (public, no auth)
//
//	@Summary		List an activity's media gallery (public)
//	@Description	Returns an activity's ordered gallery for the storefront. No auth.
//	@Tags			engage,media
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Activity ID"
//	@Success		200	{array}		models.GalleryItem
//	@Failure		400	{object}	models.ErrorResponse	"Invalid activity id"
//	@Failure		500	{object}	models.ErrorResponse	"Internal error"
//	@Router			/engage/{id}/media [get]
func (h *Handler) PublicListActivityMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid activity id"})
	}
	items, err := h.service.ListActivityMedia(c.Context(), id)
	if err != nil {
		log.Printf("Handler.PublicListActivityMedia: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to list activity media"})
	}
	return c.Status(fiber.StatusOK).JSON(items)
}
