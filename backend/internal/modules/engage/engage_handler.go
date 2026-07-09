package engage

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service}
}

// requestLocale returns the locale from the ?locale= query param, falling back
// to the Accept-Language header. TDD §5.1: ?locale= overrides Accept-Language.
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

// GetActivities: GET /engage?locale=en-US&type=Destination&page=1&limit=20
// Returns published activities for a locale, optionally filtered by the parent
// `type` (e.g. Destination vs Local Lifestyle), paginated.
func (h *Handler) GetActivities(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	locale := requestLocale(c)
	typeFilter := c.Query("type") // optional: "Destination", "Local Lifestyle", etc.

	activities, total, err := h.service.GetActivities(c.Context(), locale, typeFilter, page, limit)
	if err != nil {
		log.Printf("Handler.GetActivities: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve activities"})
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(activities, page, limit, total))
}

// GetActivityArticle: GET /engage/:slug?locale=en-US
func (h *Handler) GetActivityArticle(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Activity slug parameter is required"})
	}
	locale := requestLocale(c)
	article, err := h.service.GetActivityArticle(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Article not found"})
		}
		log.Printf("Handler.GetActivityArticle: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve article"})
	}
	return c.Status(fiber.StatusOK).JSON(article)
}
