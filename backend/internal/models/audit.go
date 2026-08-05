package models

import "time"

// AuditAction is the stable identifier for a sensitive action recorded in
// audit_log (PRD §3.1.1, TDD §135). Lowercase kebab-case; the dot groups by
// domain (content.*, product.*, order.*, itinerary.*, user.*, privacy.*,
// media.*, shipping-tier.*, option-rate.*).
type AuditAction string

const (
	// Content workflow transitions (ceramicstory / engage / artist / product).
	AuditActionContentApprove   AuditAction = "content.approve"
	AuditActionContentReject    AuditAction = "content.reject"
	AuditActionContentUnpublish AuditAction = "content.unpublish"
	// Deletes (per-entity so the UI can group).
	AuditActionProductDelete      AuditAction = "product.delete"
	AuditActionSKUDelete          AuditAction = "sku.delete"
	AuditActionCeramicStoryDelete AuditAction = "ceramic-story.delete"
	AuditActivityDelete           AuditAction = "activity.delete"
	AuditActionArtistDelete       AuditAction = "artist.delete"
	AuditActionMediaDelete        AuditAction = "media.delete"
	AuditActionShippingTierDelete AuditAction = "shipping-tier.delete"
	AuditActionOptionRateDelete   AuditAction = "option-rate.delete"
	// User management.
	AuditActionRoleAssign AuditAction = "user.role.assign"
	// Order.
	AuditActionOrderRefund AuditAction = "order.refund"
	// Itinerary transitions.
	AuditActionItineraryCancel        AuditAction = "itinerary.cancel"
	AuditActionItineraryAssign        AuditAction = "itinerary.assign"
	AuditActionItineraryConfirm       AuditAction = "itinerary.confirm"
	AuditActionItineraryRefundDeposit AuditAction = "itinerary.refund-deposit"
	// GDPR.
	AuditActionPrivacyDeleteAccount AuditAction = "privacy.delete-account"
)

// AuditEntityType is the entity-kind dimension on audit_log.entity_type.
type AuditEntityType string

const (
	AuditEntityProduct          AuditEntityType = "product"
	AuditEntitySKU              AuditEntityType = "sku"
	AuditEntityCeramicStory     AuditEntityType = "ceramic_story"
	AuditEntityActivity         AuditEntityType = "activity"
	AuditEntityArtist           AuditEntityType = "artist"
	AuditEntityUser             AuditEntityType = "user"
	AuditEntityOrder            AuditEntityType = "order"
	AuditEntityItineraryRequest AuditEntityType = "itinerary_request"
	AuditEntityMediaAsset       AuditEntityType = "media_asset"
	AuditEntityShippingFeeTier  AuditEntityType = "shipping_fee_tier"
	AuditEntityOptionRate       AuditEntityType = "option_rate"
	AuditEntityAccount          AuditEntityType = "account" // GDPR self-erasure
)

// AuditLog is one accountability row (TDD §135). `ActorID` is NULL when the
// actor has since been GDPR-erased (ON DELETE SET NULL). `ActorIPHash` is
// hex(HMAC(key, IP)) — no raw IP stored. `Detail` is an opaque action-specific
// JSONB payload (locale, reason, role, etc.).
type AuditLog struct {
	ID          int64           `json:"id" db:"id"`
	ActorID     *string         `json:"actor_id,omitempty" db:"actor_id"` // NULL if erased
	ActorIPHash *string         `json:"-" db:"actor_ip_hash"`             // never exposed (minimisation)
	Action      AuditAction     `json:"action" db:"action"`
	EntityType  AuditEntityType `json:"entity_type" db:"entity_type"`
	EntityID    *string         `json:"entity_id,omitempty" db:"entity_id"` // NULL for self-erasure
	Detail      map[string]any  `json:"detail,omitempty" db:"detail"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// AuditLogFilter is the query param set for GET /admin/audit-log. Zero values
// are ignored (no filter). From/To are UTC day boundaries (from inclusive, to
// exclusive) — reused from the analytics dashboard range helper.
type AuditLogFilter struct {
	ActorID    *string
	Action     AuditAction
	EntityType AuditEntityType
	EntityID   *string
	From       time.Time
	To         time.Time
}
