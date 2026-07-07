package utils

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// GetUserIDFromContext retrieves userID from Fiber context (set by JWT middleware)
func GetUserIDFromContext(c *fiber.Ctx) (string, error) {
	userIDVal := c.Locals("userID")
	if userIDVal == nil {
		return "", errors.New("userID not found in context, user may not be authenticated")
	}
	userID, ok := userIDVal.(string)
	if !ok {
		return "", errors.New("userID in context is not of type string")
	}
	if userID == "" {
		return "", errors.New("userID in context is empty")
	}
	return userID, nil
}

// GetPageLimit extracts page and limit query params for pagination
func GetPageLimit(c *fiber.Ctx) (page, limit int) {
	page = c.QueryInt("page")
	if page < 1 {
		page = 1
	}

	limit = c.QueryInt("limit")
	if limit < 1 {
		limit = 20 // Default limit
	}
	if limit > 100 { // Max limit
		limit = 100
	}
	return page, limit
}
