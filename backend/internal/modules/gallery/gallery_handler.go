package gallery

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service  ServiceInterface
	validate *validator.Validate
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
	}
}

func (h *Handler) GetArtworks(c *fiber.Ctx) error {
	userID, _ := utils.GetUserIDFromContext(c) // It's ok if this fails for a guest
	page, limit := utils.GetPageLimit(c)
	category := c.Query("category")
	artistID, _ := strconv.ParseInt(c.Query("artist"), 10, 64)

	filters := models.ArtworkFilters{
		Category: category,
		ArtistID: artistID,
		Page:     page,
		Limit:    limit,
	}

	artworks, total, err := h.service.GetArtworks(c.Context(), userID, filters)
	if err != nil {
		log.Printf("Handler.GetArtworks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve artworks"})
	}

	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(artworks, page, limit, total))
}

func (h *Handler) GetArtworkByID(c *fiber.Ctx) error {
	userID, _ := utils.GetUserIDFromContext(c)
	artworkID, err := strconv.ParseInt(c.Params("artwork_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artwork ID"})
	}

	artwork, err := h.service.GetArtworkByID(c.Context(), userID, artworkID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Artwork not found"})
		}
		log.Printf("Handler.GetArtworkByID: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve artwork"})
	}

	return c.Status(fiber.StatusOK).JSON(artwork)
}

func (h *Handler) GetGalleryCategories(c *fiber.Ctx) error {
	categories, err := h.service.GetGalleryCategories(c.Context())
	if err != nil {
		log.Printf("Handler.GetGalleryCategories: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve categories"})
	}
	return c.Status(fiber.StatusOK).JSON(categories)
}

// --- Protected Handlers ---

func (h *Handler) GetFavoriteArtworks(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	page, limit := utils.GetPageLimit(c)
	favArtworks, total, err := h.service.GetFavArtworks(c.Context(), userID, page, limit)
	if err != nil {
		log.Printf("Handler.GetFavArtworks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve favorite artworks"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(favArtworks, page, limit, total))
}

func (h *Handler) MarkAsFavorite(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	artworkID, err := strconv.ParseInt(c.Params("artwork_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artwork ID"})
	}

	if err := h.service.MarkAsFavorite(c.Context(), userID, artworkID); err != nil {
		log.Printf("Handler.MarkAsFavorite: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to mark as favorite"})
	}

	return c.SendStatus(fiber.StatusOK) // Or http.StatusCreated
}

func (h *Handler) UnmarkAsFavorite(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}
	artworkID, err := strconv.ParseInt(c.Params("artwork_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid artwork ID"})
	}

	if err := h.service.UnmarkAsFavorite(c.Context(), userID, artworkID); err != nil {
		log.Printf("Handler.UnmarkAsFavorite: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to unmark as favorite"})
	}

	return c.SendStatus(http.StatusNoContent)
}


