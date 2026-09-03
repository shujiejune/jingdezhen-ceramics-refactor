package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/testutil"

	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client with a buffered send channel and a fake
// userID. No real websocket.Conn is needed for these tests — we only verify
// that the Hub routes messages to the correct client's send channel.
func newTestClient(hub *Hub, userID string) *Client {
	return &Client{
		hub:    hub,
		userID: userID,
		send:   make(chan []byte, 8),
	}
}

// TestHubLocalDelivery_NilRedis verifies the single-instance fallback path:
// when no Redis client is configured, SendToUser delivers directly to the
// local client map.
func TestHubLocalDelivery_NilRedis(t *testing.T) {
	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := newTestClient(hub, "user-1")
	hub.register <- client

	// Give the event loop a moment to process the registration.
	time.Sleep(50 * time.Millisecond)

	require.True(t, hub.IsUserOnline("user-1"))
	require.False(t, hub.IsUserOnline("user-2"))

	notif := &models.Notification{
		ID:               42,
		NotificationType: models.NotificationTypeSystem,
		Message:          "hello",
	}
	hub.SendToUser("user-1", notif)

	select {
	case msg := <-client.send:
		var got models.Notification
		require.NoError(t, json.Unmarshal(msg, &got))
		require.Equal(t, int64(42), got.ID)
		require.Equal(t, "hello", got.Message)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for local delivery")
	}
}

// TestHubRedisPubSub verifies cross-instance fan-out: SendToUser publishes to
// Redis pub/sub, and the subscriber goroutine delivers to the local client.
//
// This is an integration test — it needs a real Redis container.
func TestHubRedisPubSub(t *testing.T) {
	rdb := testutil.NewRedisClient(t)

	hub := NewHub(rdb)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Give the PSubscribe goroutine time to confirm the subscription.
	time.Sleep(200 * time.Millisecond)

	client := newTestClient(hub, "user-pubsub")
	hub.register <- client
	time.Sleep(50 * time.Millisecond)

	require.True(t, hub.IsUserOnline("user-pubsub"))

	notif := &models.Notification{
		ID:               99,
		NotificationType: models.NotificationTypeSystem,
		Message:          "from redis",
	}
	hub.SendToUser("user-pubsub", notif)

	select {
	case msg := <-client.send:
		var got models.Notification
		require.NoError(t, json.Unmarshal(msg, &got))
		require.Equal(t, int64(99), got.ID)
		require.Equal(t, "from redis", got.Message)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Redis pub/sub delivery")
	}
}

// TestHubRedisPubSub_CrossInstance verifies that a notification published
// from a "publisher-only" hub (no local clients, no subscriber — simulating
// the worker) is delivered by a "serve" hub that has the user connected.
func TestHubRedisPubSub_CrossInstance(t *testing.T) {
	rdb := testutil.NewRedisClient(t)

	// "Serve" hub: has local clients + subscribes to Redis.
	serveHub := NewHub(rdb)
	serveCtx, serveCancel := context.WithCancel(context.Background())
	defer serveCancel()
	go serveHub.Run(serveCtx)
	time.Sleep(200 * time.Millisecond) // let PSubscribe confirm

	// "Worker" hub: publisher-only (no Run(), no local clients, no subscriber).
	workerHub := NewHub(rdb)

	// Register the user on the serve instance.
	client := newTestClient(serveHub, "user-cross")
	serveHub.register <- client
	time.Sleep(50 * time.Millisecond)

	require.True(t, serveHub.IsUserOnline("user-cross"))
	require.False(t, workerHub.IsUserOnline("user-cross"))

	// Worker publishes a notification — it has no local client, but the
	// serve instance's subscriber should pick it up from Redis.
	notif := &models.Notification{
		ID:               777,
		NotificationType: models.NotificationTypeLowStock,
		Message:          "low stock from worker",
	}
	workerHub.SendToUser("user-cross", notif)

	select {
	case msg := <-client.send:
		var got models.Notification
		require.NoError(t, json.Unmarshal(msg, &got))
		require.Equal(t, int64(777), got.ID)
		require.Equal(t, "low stock from worker", got.Message)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cross-instance delivery")
	}
}
