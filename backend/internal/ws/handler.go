package ws

import (
	"log"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// Handler handles the WebSocket connection lifecycle.
type Handler struct {
	hub *Hub
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// UpgradeConnection is the main handler for the /ws route.
// It upgrades the HTTP connection to a WebSocket and manages the client's lifecycle.
func (h *Handler) UpgradeConnection(c *websocket.Conn) {
	// The userID is passed from the middleware to the connection's Locals.
	userID, ok := c.Locals("userID").(string)
	if !ok || userID == "" {
		log.Println("ws.Handler.UpgradeConnection: Failed to get userID from locals")
		c.WriteJSON(models.ErrorResponse{Message: "Unauthorized"})
		c.Close()
		return
	}

	// Create a new Client for this connection.
	client := &Client{
		hub:    h.hub,
		userID: userID,
		conn:   c,
		send:   make(chan []byte, 256),
	}

	// Register the new client with the hub.
	client.hub.register <- client

	// Start the read and write pumps in separate goroutines.
	// These will run for the lifetime of the connection.
	go client.writePump()
	// The readPump is blocking, so it must be called last.
	// When it returns, it means the connection is closed.
	client.readPump()
}

// WsUpgradeMiddleware is a Fiber middleware that checks if the request is a WebSocket upgrade request.
// It also extracts the userID from the JWT middleware's locals and passes it to the WebSocket connection.
func WsUpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			userID, ok := c.Locals("userID").(string)
			if !ok || userID == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Unauthorized for WebSocket"})
			}
			// Store the userID in the connection's locals for the handler to access.
			c.Locals("userID", userID)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}
