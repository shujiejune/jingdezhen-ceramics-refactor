package models

import "time"

// AnalyticsEventKind is the row `kind` in analytics_events (TDD §194).
// `pageview` is a content/SPA view (name is NULL); `event` is a named client
// action (name set) — used for the itinerary funnel (form_view → started →
// submitted) and any other tracked interaction.
type AnalyticsEventKind string

const (
	AnalyticsKindPageview AnalyticsEventKind = "pageview"
	AnalyticsKindEvent    AnalyticsEventKind = "event"
)

// AnalyticsEvent is one pseudonymous analytics row. `Country` is the ISO
// 3166-1 alpha-2 code resolved at ingest via GeoLite2 ('ZZ' = unknown).
// `VisitorHash` is hex(HMAC(daily-rotating key, IP+UA)) — no raw IP is ever
// stored (GDPR minimisation, TDD §11). `Props` is an opaque client JSON blob
// (event-specific payload); the service does not introspect it.
type AnalyticsEvent struct {
	ID          int64              `json:"id" db:"id"`
	Ts          time.Time          `json:"ts" db:"ts"`
	Kind        AnalyticsEventKind `json:"kind" db:"kind"`
	Path        string             `json:"path" db:"path"`
	Name        *string            `json:"name,omitempty" db:"name"` // NULL for pageview
	Country     string             `json:"country" db:"country"`     // CHAR(2)
	Locale      *string            `json:"locale,omitempty" db:"locale"`
	VisitorHash string             `json:"-" db:"visitor_hash"` // never exposed (minimisation)
	Props       map[string]any     `json:"props,omitempty" db:"props"`
}

// AnalyticsEventRequest is the body for POST /analytics/events (TDD §263,
// consent-gated). `props` is an arbitrary object; validated as a JSON object
// (non-array) by the service — kept loose here so the schema can evolve
// without a DTO change per event.
type AnalyticsEventRequest struct {
	Kind   AnalyticsEventKind `json:"kind" validate:"required,oneof=pageview event"`
	Path   string             `json:"path" validate:"required,max=2048"`
	Name   string             `json:"name,omitempty" validate:"omitempty,max=100"`
	Locale string             `json:"locale,omitempty" validate:"omitempty,max=10"`
	Props  map[string]any     `json:"props,omitempty"`
}

// AnalyticsDaily is one rolled-up aggregate row (analytics:rollup job, TDD
// §4.2). `Metric` is one of pageviews|events|visitors; `Dims` is a JSONB
// object of the grouping keys (e.g. {"path":"/products","country":"US"}).
type AnalyticsDaily struct {
	ID     int64          `json:"id" db:"id"`
	Date   string         `json:"date" db:"date"` // YYYY-MM-DD
	Metric string         `json:"metric" db:"metric"`
	Dims   map[string]any `json:"dims" db:"dims"`
	Value  int64          `json:"value" db:"value"`
}
