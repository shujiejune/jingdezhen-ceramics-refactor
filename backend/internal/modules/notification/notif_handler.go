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
//
// @Summary      List the user's notifications
// @Description  Paginated list of the signed-in user's notifications.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        page  query int false "Page number (1-based)" default(1)
// @Param        limit query int false "Page size (max 100)" default(20)
// @Success      200 {object} models.PaginatedResponse{data=[]models.Notification}
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /notifications [get]
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
//
// @Summary      Get the unread notification count
// @Description  Returns the count of the signed-in user's unread notifications.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      200 {object} object "{count: int}"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /notifications/unread-count [get]
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
//
// @Summary      Mark a notification as read
// @Description  Marks one of the signed-in user's notifications as read.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        notification_id path int true "Notification ID"
// @Success      204 "No Content (empty body)"
// @Failure      400 {object} models.ErrorResponse "Invalid notification ID"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      404 {object} models.ErrorResponse "Notification not found (or not owned by user)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /notifications/{notification_id}/mark-read [post]
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
//
// @Summary      Mark all notifications as read
// @Description  Marks all of the signed-in user's notifications as read.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      204 "No Content (empty body)"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /notifications/mark-all-read [post]
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
