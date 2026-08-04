package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/contrib/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub    *Hub
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		// The server doesn't need to read any messages from the client for this feature,
		// but this loop is necessary to detect when the client closes the connection.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws.Client.readPump: error: %v", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop. It must be run in a separate goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// If the context is cancelled, close all client connections.
			h.mu.Lock()
			for _, client := range h.clients {
				close(client.send)
			}
			h.clients = make(map[string]*Client)
			h.mu.Unlock()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.userID] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; ok {
				delete(h.clients, client.userID)
				close(client.send)
			}
			h.mu.Unlock()
		}
	}
}

// SendToUser sends a notification to a specific user if they are online.
func (h *Hub) SendToUser(userID string, notification *models.Notification) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if ok {
		message, err := json.Marshal(notification)
		if err != nil {
			log.Printf("ws.Hub.SendToUser: error marshalling notification: %v", err)
			return
		}
		select {
		case client.send <- message:
		default:
			// If the send channel is full, it means the client is lagging.
			// We can choose to drop the message to prevent the server from blocking.
			log.Printf("ws.Hub.SendToUser: client %s send channel full, dropping message", userID)
		}
	}
}

// IsUserOnline checks if a user has an active WebSocket connection.
func (h *Hub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}
