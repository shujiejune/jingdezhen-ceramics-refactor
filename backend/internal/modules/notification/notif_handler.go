package notification

import (
	"errors"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// Handler handles HTTP requests for the notification module.
type Handler struct {
	service ServiceInterface
}

// NewHandler creates a new notification handler.
func NewHandler(service ServiceInterface) *Handler {
	return &Handler{
		service: service,
	}
}

// GetNotifications handles requests from an authenticated user to get their notifications.
func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	// This is a protected route, so a user ID must exist in the context.
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	// Get pagination parameters from the request query string.
	page, limit := utils.GetPageLimit(c)

	notifications, total, err := h.service.GetNotificationsForUser(c.Context(), userID, page, limit)
	if err != nil {
		log.Printf("Handler.GetNotifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve notifications"})
	}

	// Return the data in a standardized paginated response format.
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(notifications, page, limit, total))
}

// GetUnreadNotificationCount handles requests to get the count of a user's unread notifications.
func (h *Handler) GetUnreadNotificationCount(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	count, err := h.service.GetUnreadNotificationCount(c.Context(), userID)
	if err != nil {
		log.Printf("Handler.GetUnreadNotificationCount: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to retrieve unread notification count"})
	}

	// Return the count in a simple JSON object.
	return c.Status(fiber.StatusOK).JSON(map[string]int64{"count": count})
}

// MarkAsRead handles requests to mark a single notification as read.
func (h *Handler) MarkAsRead(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	// Parse the notification ID from the URL path parameter.
	notificationID, err := strconv.ParseInt(c.Params("notification_id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid notification ID"})
	}

	err = h.service.MarkNotificationAsRead(c.Context(), notificationID, userID)
	if err != nil {
		// Check if the error is a "not found" error, which could mean the notification
		// doesn't exist or doesn't belong to the user.
		if errors.Is(err, models.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{Message: "Notification not found"})
		}
		log.Printf("Handler.MarkAsRead: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to mark notification as read"})
	}

	// A 204 No Content response is appropriate for a successful action that doesn't return a body.
	return c.SendStatus(fiber.StatusNoContent)
}

// MarkAllAsRead handles requests to mark all of a user's notifications as read.
func (h *Handler) MarkAllAsRead(c *fiber.Ctx) error {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: err.Error()})
	}

	err = h.service.MarkAllNotificationsAsRead(c.Context(), userID)
	if err != nil {
		log.Printf("Handler.MarkAllAsRead: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to mark all notifications as read"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
