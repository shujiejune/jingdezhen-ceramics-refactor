package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	// pubSubChannelPrefix is the Redis pub/sub channel name prefix for
	// per-user notification delivery. Each user gets a dedicated channel
	// so that only the instance(s) with that user connected receive the
	// message — no global fan-out to all instances.
	pubSubChannelPrefix = "ws:user:"
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

// pubSubMessage is the envelope published to Redis pub/sub. It carries
// the target userID (so the subscriber knows which local client to deliver
// to) and the marshalled notification payload (already-JSON, ready to
// write to the socket).
type pubSubMessage struct {
	UserID  string `json:"user_id"`
	Payload []byte `json:"payload"` // JSON-marshalled models.Notification
}

// Hub maintains the set of active local clients and uses Redis pub/sub
// for cross-instance fan-out. When SendToUser is called, it publishes to
// a per-user Redis channel; the subscriber goroutine on whichever instance
// has that user connected picks it up and writes to the local socket.
//
// The rdb field may be nil — in that case SendToUser falls back to direct
// local delivery (single-instance dev mode). This keeps the worker (which
// has no WebSocket clients but needs to publish notifications) able to
// construct a publisher-only Hub: it sets rdb, leaves the subscriber
// goroutine off, and SendToUser publishes to Redis for a serve instance
// to deliver.
type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex

	rdb *redis.Client
}

// NewHub creates a new Hub with Redis pub/sub fan-out. Pass a non-nil
// rdb to enable cross-instance delivery; pass nil for single-instance
// local-only mode (dev without Redis, or tests).
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rdb:        rdb,
	}
}

// Run starts the hub's event loop. It must be run in a separate goroutine.
// If a Redis client is configured, it also starts the pub/sub subscriber
// goroutine for cross-instance fan-out.
func (h *Hub) Run(ctx context.Context) {
	// Start the Redis pub/sub subscriber alongside the event loop when a
	// Redis client is available. This runs only on serve instances (the
	// worker doesn't serve WebSockets, so it doesn't subscribe).
	if h.rdb != nil {
		go h.runSubscriber(ctx)
	}

	for {
		select {
		case <-ctx.Done():
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

// runSubscriber listens on the per-user Redis pub/sub channel pattern
// and delivers messages to local WebSocket clients. When a notification
// is published (by this instance or any other), the subscriber on the
// instance that has the user connected writes it to the local socket.
func (h *Hub) runSubscriber(ctx context.Context) {
	pubsub := h.rdb.PSubscribe(ctx, pubSubChannelPrefix+"*")
	defer pubsub.Close()

	// Wait for the subscription to be confirmed before entering the
	// receive loop (avoids a race where a message is published before
	// the subscription is active).
	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("ws.Hub.runSubscriber: PSubscribe confirm: %v", err)
		return
	}

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var pm pubSubMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pm); err != nil {
				log.Printf("ws.Hub.runSubscriber: unmarshal: %v", err)
				continue
			}
			h.deliverLocal(pm.UserID, pm.Payload)
		}
	}
}

// deliverLocal writes a pre-marshalled payload to the local client's
// send channel if one exists. Non-blocking: a full channel drops the
// message (same behavior as the original in-memory hub).
//
// The send happens under h.mu.RLock so that Run()'s close(client.send)
// (which holds h.mu.Lock) can't race with the send here. A non-blocking
// select (default: drop) prevents deadlock if the channel is full.
func (h *Hub) deliverLocal(userID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.clients[userID]
	if !ok {
		return
	}
	select {
	case client.send <- payload:
	default:
		log.Printf("ws.Hub.deliverLocal: client %s send channel full, dropping message", userID)
	}
}

// SendToUser publishes a notification to the per-user Redis pub/sub
// channel. The instance with the user's WebSocket connection will
// receive it and deliver to the local socket. Falls back to direct
// local delivery when no Redis client is configured (single-instance).
//
// This satisfies notification.WebSocketService so the notification service
// can call it without knowing about Redis.
func (h *Hub) SendToUser(userID string, notification *models.Notification) {
	payload, err := json.Marshal(notification)
	if err != nil {
		log.Printf("ws.Hub.SendToUser: marshal notification: %v", err)
		return
	}

	// No Redis → single-instance dev mode: deliver locally.
	if h.rdb == nil {
		h.deliverLocal(userID, payload)
		return
	}

	// Publish to Redis; the subscriber on the instance with the user
	// connected will deliver. We also deliver locally as a fast-path
	// (avoids a Redis round-trip when the user is on this instance).
	h.deliverLocal(userID, payload)

	pm := pubSubMessage{UserID: userID, Payload: payload}
	data, err := json.Marshal(pm)
	if err != nil {
		log.Printf("ws.Hub.SendToUser: marshal envelope: %v", err)
		return
	}
	channel := pubSubChannelPrefix + userID
	if err := h.rdb.Publish(context.Background(), channel, data).Err(); err != nil {
		log.Printf("ws.Hub.SendToUser: publish to %s: %v", channel, err)
	}
}

// IsUserOnline checks if a user has an active WebSocket connection on
// this instance. Note: this is local-only — a user connected on another
// instance will report false. The notification service uses this as a
// hint to skip the real-time push when no connection is likely; the
// Redis publish path is still the reliable delivery mechanism.
func (h *Hub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}
