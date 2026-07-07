package ceramicstory

import (
	"jingdezhen-ceramics-backend/internal/models"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for ceramic stories.
type Handler struct {
	service ServiceInterface
	// validate *validator.Validate // Needed for admin C/U/D operations
}

// NewHandler creates a new ceramic story handler.
func NewHandler(service ServiceInterface) *Handler {
	return &Handler{
		service: service,
		// validate: validator.New(),
	}
}

// GetAllDynasties handles the request to get all ceramic stories.
// Corresponds to: csGroup.GET("", csHandler.GetAllDynasties)
func (h *Handler) GetAllDynasties(c *fiber.Ctx) error {
	ctx := c.Context()
	stories, err := h.service.GetAllCeramicStories(ctx)
	if err != nil {
		// In a real app, you'd check the error type for more specific responses
		log.Printf("Handler.GetAllDynasties: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic stories"})
	}

	if len(stories) == 0 {
		return c.Status(fiber.StatusOK).JSON([]models.CeramicStory{})
	}

	return c.Status(fiber.StatusOK).JSON(stories)
}

// GetDynastyDetail handles the request to get details for a specific ceramic story.
// Corresponds to: csGroup.GET("/:dynasty_id_or_slug", csHandler.GetDynastyDetail)
func (h *Handler) GetDynastyDetail(c *fiber.Ctx) error {
	idOrSlug := c.Params("dynasty_id_or_slug")
	if idOrSlug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Dynasty ID or slug parameter is required"})
	}

	ctx := c.Context()
	story, err := h.service.GetCeramicStoryDetail(ctx, idOrSlug)
	if err != nil {
		if err == models.ErrNotFound || strings.Contains(err.Error(), models.ErrNotFound.Error()) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		}
		log.Printf("Handler.GetDynastyDetail: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve ceramic story details"})
	}

	return c.Status(fiber.StatusOK).JSON(story)
}

// --- Admin Handlers (Example - uncomment and complete if you add admin routes) ---
/*
func (h *Handler) CreateCeramicStory(c *fiber.Ctx) error {
	// This route would need admin authentication middleware
	var req models.CreateCeramicStoryData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}
	if err := h.validate.StructCtx(c.Context(), req); err != nil { // Use StructCtx for context-aware validation
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	story, err := h.service.CreateCeramicStory(c.Context(), req)
	if err != nil {
		// Check for specific errors like models.ErrConflict (slug taken)
		// if errors.Is(err, models.ErrConflict) {
		// 	  return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Slug already exists"})
		// }
		log.Printf("Handler.CreateCeramicStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to create ceramic story"})
	}
	return c.Status(fiber.StatusCreated).JSON(story)
}

func (h *Handler) UpdateCeramicStory(c *fiber.Ctx) error {
	// This route would need admin authentication middleware
	idStr := c.Params("id") // Assuming route is /admin/ceramicstory/:id
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid ID parameter"})
	}

	var req models.UpdateCeramicStoryData
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
	}
	if err := h.validate.StructCtx(c.Context(), req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	story, err := h.service.UpdateCeramicStory(c.Context(), id, req)
	if err != nil {
		// if errors.Is(err, models.ErrNotFound) {
		// 	 return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		// }
		// if errors.Is(err, models.ErrConflict) {
		// 	 return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Slug already exists for another story"})
		// }
		log.Printf("Handler.UpdateCeramicStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to update ceramic story"})
	}
	return c.Status(fiber.StatusOK).JSON(story)
}

func (h *Handler) DeleteCeramicStory(c *fiber.Ctx) error {
	// This route would need admin authentication middleware
	idStr := c.Params("id") // Assuming route is /admin/ceramicstory/:id
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid ID parameter"})
	}

	err = h.service.DeleteCeramicStory(c.Context(), id)
	if err != nil {
		// if errors.Is(err, models.ErrNotFound) {
		// 	 return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Ceramic story not found"})
		// }
		log.Printf("Handler.DeleteCeramicStory: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to delete ceramic story"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
*/
