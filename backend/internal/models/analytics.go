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
// NOTE: the dashboard read endpoints (Phase B) query the live source tables
// (analytics_events / orders / order_items / itinerary_requests) directly for
// MVP-volume always-correctness; analytics_daily stays wired + forward-looking
// and the reads can switch to it when volume forces it (TDD §12).
type AnalyticsDaily struct {
	ID     int64          `json:"id" db:"id"`
	Date   string         `json:"date" db:"date"` // YYYY-MM-DD
	Metric string         `json:"metric" db:"metric"`
	Dims   map[string]any `json:"dims" db:"dims"`
	Value  int64          `json:"value" db:"value"`
}

// --- Dashboard report DTOs (PRD §3.4.2, Phase B) -----------------------------
// All three reports share a date range (inclusive, UTC days) and a zero-filled
// daily Series so the chart never has gaps. Breakdown lists are capped at 100
// rows by the repository.

// TrafficReport covers PRD §3.4.2 "Traffic analysis": visits by visitor
// geolocation (country), page views, top content. All counts come from
// analytics_events in the range.
type TrafficReport struct {
	From      time.Time          `json:"from"`
	To        time.Time          `json:"to"`
	Totals    TrafficTotals      `json:"totals"`
	Series    []DailyPoint       `json:"series"`     // one per day, zero-filled
	ByCountry []TrafficByCountry `json:"by_country"` // top 100
	ByPath    []TrafficByPath    `json:"by_path"`    // top 100 pageviews
	ByLocale  []TrafficByLocale  `json:"by_locale"`  // top 100
}

type TrafficTotals struct {
	Pageviews int64 `json:"pageviews"`
	Visitors  int64 `json:"visitors"` // distinct visitor_hash
	Events    int64 `json:"events"`   // named events (kind='event')
}

type TrafficByCountry struct {
	Country   string `json:"country"` // ISO 3166-1 alpha-2; 'ZZ' = unknown
	Pageviews int64  `json:"pageviews"`
	Visitors  int64  `json:"visitors"`
}

type TrafficByPath struct {
	Path      string `json:"path"`
	Pageviews int64  `json:"pageviews"`
}

type TrafficByLocale struct {
	Locale    string `json:"locale"`
	Pageviews int64  `json:"pageviews"`
	Visitors  int64  `json:"visitors"`
}

// SalesReport covers PRD §3.4.2 "Sales analysis": GMV, orders, by
// currency/region/product/artist, time series. GMV = SUM(subtotal_cny)
// (merchandise, excludes shipping) over realized orders only
// (status IN paid|shipped|completed — cancelled/refunded excluded).
type SalesReport struct {
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	Totals     SalesTotals       `json:"totals"`
	Series     []SalesDailyPoint `json:"series"` // one per day, zero-filled
	ByCurrency []SalesByCurrency `json:"by_currency"`
	ByCountry  []SalesByCountry  `json:"by_country"`
	ByProduct  []SalesByProduct  `json:"by_product"`
	ByArtist   []SalesByArtist   `json:"by_artist"`
}

type SalesTotals struct {
	GMVCny int64 `json:"gmv_cny"` // Σ subtotal_cny over realized orders
	Orders int64 `json:"orders"`  // count of realized orders
}

type SalesDailyPoint struct {
	Date   string `json:"date"` // YYYY-MM-DD
	GMVCny int64  `json:"gmv_cny"`
	Orders int64  `json:"orders"`
}

type SalesByCurrency struct {
	Currency string `json:"currency"` // CHAR(3) presentment
	GMVCny   int64  `json:"gmv_cny"`
	Orders   int64  `json:"orders"`
}

type SalesByCountry struct {
	Country string `json:"country"` // from address->>'country'
	GMVCny  int64  `json:"gmv_cny"`
	Orders  int64  `json:"orders"`
}

type SalesByProduct struct {
	ProductID int64  `json:"product_id"`
	Title     string `json:"title"` // en-US product title fallback ("Unknown" if missing)
	GMVCny    int64  `json:"gmv_cny"`
	Qty       int64  `json:"qty"`
}

type SalesByArtist struct {
	ArtistID int64  `json:"artist_id"`
	Name     string `json:"name"` // artist name ("Unknown" if artist_id NULL)
	GMVCny   int64  `json:"gmv_cny"`
	Qty      int64  `json:"qty"`
}

// FunnelReport covers PRD §3.4.2 "Itinerary conversion": form views →
// submissions → confirmed, conversion rates over time. Views come from
// analytics_events named `itinerary_form_view`; submitted/confirmed come from
// itinerary_requests by submitted_at (cohort semantics: submitted-in-range ∩
// status='confirmed').
type FunnelReport struct {
	From       time.Time          `json:"from"`
	To         time.Time          `json:"to"`
	Totals     FunnelTotals       `json:"totals"`
	Conversion FunnelConversion   `json:"conversion"`
	Series     []FunnelDailyPoint `json:"series"` // one per day, zero-filled
}

type FunnelTotals struct {
	Views     int64 `json:"views"`     // analytics_events name=itinerary_form_view
	Submitted int64 `json:"submitted"` // itinerary_requests submitted_at in range
	Confirmed int64 `json:"confirmed"` // subset with status='confirmed'
}

// FunnelConversion rates are percentages 0–100, rounded to 2 dp. A zero
// denominator yields 0 (no NaN in JSON).
type FunnelConversion struct {
	ViewToSubmit    float64 `json:"view_to_submit_pct"`    // submitted/views × 100
	SubmitToConfirm float64 `json:"submit_to_confirm_pct"` // confirmed/submitted × 100
}

type FunnelDailyPoint struct {
	Date      string `json:"date"` // YYYY-MM-DD
	Views     int64  `json:"views"`
	Submitted int64  `json:"submitted"`
	Confirmed int64  `json:"confirmed"`
}

// DailyPoint is the shared daily-series point for the traffic report
// (pageviews/visitors/events). Dates are YYYY-MM-DD (UTC day buckets).
type DailyPoint struct {
	Date      string `json:"date"`
	Pageviews int64  `json:"pageviews"`
	Visitors  int64  `json:"visitors"`
	Events    int64  `json:"events"`
}
