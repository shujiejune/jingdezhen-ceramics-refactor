package ceramicstory

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for ceramic stories (History & Heritage).
type Handler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service}
}

// requestLocale returns the locale from the ?locale= query param, falling back
// to the Accept-Language header (first entry, lowercased). TDD §5.1: ?locale=
// overrides Accept-Language; default en-US.
func requestLocale(c *fiber.Ctx) string {
	loc := c.Query("locale")
	if loc != "" {
		return loc
	}
	// Accept-Language header fallback (simple first-token extraction).
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

// GetAllDynasties: GET /ceramicstory?locale=en-US
func (h *Handler) GetAllDynasties(c *fiber.Ctx) error {
	locale := requestLocale(c)
	stories, err := h.service.GetAllCeramicStories(c.Context(), locale)
	if err != nil {
		log.Printf("Handler.GetAllDynasties: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic stories"})
	}
	if len(stories) == 0 {
		return c.Status(fiber.StatusOK).JSON([]models.CeramicStory{})
	}
	return c.Status(fiber.StatusOK).JSON(stories)
}

// GetDynastyDetail: GET /ceramicstory/:slug?locale=en-US
func (h *Handler) GetDynastyDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Dynasty slug parameter is required"})
	}
	locale := requestLocale(c)
	story, err := h.service.GetCeramicStoryDetail(c.Context(), slug, locale)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		log.Printf("Handler.GetDynastyDetail: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic story details"})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}
