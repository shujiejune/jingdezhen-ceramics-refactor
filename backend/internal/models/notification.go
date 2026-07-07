package models

import "time"

// NotificationType defines the type of notification.
type NotificationType string

const (
	// System notifications (PRD-aligned types to be added as modules land:
	// order status, low-stock alerts, itinerary status, chat handoff, content approval)
	NotificationTypeSystem NotificationType = "system"
)

// Notification represents a single notification for a user.
type Notification struct {
	ID               int64            `json:"notification_id" db:"notification_id"`
	RecipientUserID  string           `json:"recipient_user_id" db:"recipient_user_id"`
	ActorUserID      *string          `json:"actor_user_id,omitempty" db:"actor_user_id"`
	ActorUser        *User            `json:"actor_user,omitempty" db:"-"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	EntityType       *string          `json:"entity_type,omitempty" db:"entity_type"`
	EntityID         *int64           `json:"entity_id,omitempty" db:"entity_id"`
	Message          string           `json:"message" db:"message"`
	IsRead           bool             `json:"is_read" db:"is_read"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
}

// CreateNotificationParams holds the data needed to create a notification.
type CreateNotificationParams struct {
	RecipientUserID string
	ActorUserID     string // Use an empty string for system notifications
	Type            NotificationType
	EntityType      string
	EntityID        int64
	ExtraData       map[string]string
}
