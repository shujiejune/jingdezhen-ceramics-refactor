package engage

import (
	"errors"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service}
}

// GetActivities handles the request to get a paginated list of activities.
func (h *Handler) GetActivities(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)
	ctx := c.Context()

	activities, total, err := h.service.GetActivities(ctx, page, limit)
	if err != nil {
		log.Printf("Handler.GetActivities: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve activities"})
	}

	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(activities, page, limit, total))
}

// GetActivityArticle handles the request to get a detailed article.
func (h *Handler) GetActivityArticle(c *fiber.Ctx) error {
	idOrSlug := c.Params("activity_id_or_slug")
	if idOrSlug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Activity ID or slug parameter is required"})
	}

	ctx := c.Context()
	article, err := h.service.GetActivityArticle(ctx, idOrSlug)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) || strings.Contains(err.Error(), models.ErrNotFound.Error()) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Article not found"})
		}
		log.Printf("Handler.GetActivityArticle: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve article"})
	}

	return c.Status(fiber.StatusOK).JSON(article)
}
