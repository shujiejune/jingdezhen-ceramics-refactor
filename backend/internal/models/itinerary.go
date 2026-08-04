package models

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Itinerary Request + Draft (PRD §3.3.2, TDD §3.4 M3)
//
// The Custom Itinerary Builder is a 4-step customer-facing wizard (trip basics
// → preferences → services → contact & consent). Submission requires sign-in.
// A submitted request is an immutable form snapshot carrying the customer's
// preferences + a 24h-SLA deadline; planners work it through the CRM lifecycle
// (pending → processing → quoted → deposit_paid → confirmed | cancelled/closed)
// in a follow-up sub-track.
//
// Money (budget) is BIGINT minor units everywhere (TDD §7); never floats.
// user_id FK = NO ACTION (mirrors orders): requests survive GDPR erasure.
// =============================================================================

// ItineraryStatus is the request state-machine value (PRD §3.3.2).
type ItineraryStatus string

const (
	StatusItineraryPending     ItineraryStatus = "pending"
	StatusItineraryProcessing  ItineraryStatus = "processing"
	StatusItineraryQuoted      ItineraryStatus = "quoted"
	StatusItineraryDepositPaid ItineraryStatus = "deposit_paid"
	StatusItineraryConfirmed   ItineraryStatus = "confirmed"
	StatusItineraryCancelled   ItineraryStatus = "cancelled"
	StatusItineraryClosed      ItineraryStatus = "closed"
)

// ItineraryRequest is a submitted itinerary request (the immutable snapshot).
type ItineraryRequest struct {
	ID            int64           `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	Status        ItineraryStatus `json:"status" db:"status"`
	// Step 1 — Trip basics
	ArrivalDate   *time.Time      `json:"arrival_date,omitempty" db:"arrival_date"`
	DurationDays  int             `json:"duration_days" db:"duration_days"`
	Flexible      bool            `json:"flexible" db:"flexible"`
	Adults        int             `json:"adults" db:"adults"`
	Children      int             `json:"children" db:"children"`
	// Step 2 — Preferences
	Interests     json.RawMessage `json:"interests" db:"interests"` // []string
	Budget        json.RawMessage `json:"budget,omitempty" db:"budget"`
	Pace          string          `json:"pace" db:"pace"`
	// Step 3 — Services
	Services      json.RawMessage `json:"services" db:"services"`
	// Step 4 — Contact & notes
	Contact       json.RawMessage `json:"contact" db:"contact"`
	Notes         *string         `json:"notes,omitempty" db:"notes"`
	// Auto-attached
	Locale        string          `json:"locale" db:"locale"`
	SLADeadline   time.Time       `json:"sla_deadline" db:"sla_deadline"`
	AssignedTo    *string         `json:"assigned_to,omitempty" db:"assigned_to"`
	SubmittedAt   time.Time       `json:"submitted_at" db:"submitted_at"`
	CancelReason  *string         `json:"cancel_reason,omitempty" db:"cancel_reason"`
	CancelledAt   *time.Time      `json:"cancelled_at,omitempty" db:"cancelled_at"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}

// ItineraryDraft is the save-resume state for a signed-in user (one per user).
type ItineraryDraft struct {
	ID        int64           `json:"id" db:"id"`
	UserID    string          `json:"user_id" db:"user_id"`
	FormState json.RawMessage `json:"form_state" db:"form_state"`
	Step      int             `json:"step" db:"step"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// --- Request DTOs ---

// ItineraryDraftData is the body for PUT /itineraries/draft.
type ItineraryDraftData struct {
	Step      int             `json:"step" validate:"required,min=1,max=4"`
	FormState json.RawMessage `json:"form_state"`
}

// BudgetInput is the per-person budget range (presentment currency, minor units).
type BudgetInput struct {
	Currency  string `json:"currency" validate:"required,oneof=USD EUR GBP"`
	MinMinor  int64  `json:"min_minor" validate:"gte=0"`
	MaxMinor  int64  `json:"max_minor" validate:"gte=0"`
}

// ServicesInput captures step 3 (tour guide, hotel, pickup, experience, dietary).
type ServicesInput struct {
	Guide                 string `json:"guide" validate:"required,oneof=none english other"`
	Hotel                 bool   `json:"hotel"`
	HotelLevel            string `json:"hotel_level,omitempty" validate:"omitempty,oneof=budget comfort luxury"`
	Pickup                bool   `json:"pickup"`
	Experience            bool   `json:"experience"`
	DietaryAccessibility  string `json:"dietary_accessibility,omitempty"`
}

// ContactInput captures step 4 (preferred channel + free-text notes).
type ContactInput struct {
	Channel        string `json:"channel" validate:"required,oneof=email whatsapp"`
	WhatsAppNumber string `json:"whatsapp_number,omitempty"`
	Notes          string `json:"notes,omitempty"` // "anything else we should know" (step 4 free text)
}

// ItinerarySubmitRequest is the full 4-step body for POST /itineraries.
// Auto-attached fields (user_id, username, email, locale) are NOT in the body;
// the service fills them from the authenticated user + the request locale.
type ItinerarySubmitRequest struct {
	// Step 1 — Trip basics
	ArrivalDate  string `json:"arrival_date" validate:"required"` // YYYY-MM-DD
	DurationDays int    `json:"duration_days" validate:"required,gt=0"`
	Flexible     bool   `json:"flexible"`
	Adults       int    `json:"adults" validate:"required,gte=1"`
	Children     int    `json:"children" validate:"gte=0"`
	// Step 2 — Preferences
	Interests []string     `json:"interests"`
	Budget    *BudgetInput `json:"budget,omitempty"`
	Pace      string       `json:"pace" validate:"required,oneof=relaxed balanced packed"`
	// Step 3 — Services
	Services *ServicesInput `json:"services" validate:"required"`
	// Step 4 — Contact & consent
	Contact *ContactInput `json:"contact" validate:"required"`
	Notes   string        `json:"notes,omitempty"` // top-level free-text (PRD step 4 "anything else")
	Consent bool          `json:"consent" validate:"required,eq=true"` // GDPR Privacy Policy checkbox
}

// ItineraryCancelRequest is the optional body for POST /itineraries/:id/cancel.
type ItineraryCancelRequest struct {
	Reason string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// =============================================================================
// Planner CRM (PRD §3.3.2 "Backend/CRM (Travel Planner role)", TDD §3.4 M3 #2)
//
// The planner-facing counterpart to the customer wizard. Planners work a
// request through the lifecycle: pending → processing → (quoted/deposit/…
// are a later sub-track) → confirmed | cancelled | closed. This sub-track adds
// inbox + SLA flagging, assignment, internal notes (contact history), and CSV
// export. The quote builder + deposit + confirm transitions land with the
// quote object (next sub-track).
// =============================================================================

// CRMNote is an internal planner note attached to a request (contact history).
// author_id FK = NO ACTION (notes survive the author's GDPR erasure).
type CRMNote struct {
	ID         int64     `json:"id" db:"id"`
	RequestID  int64     `json:"request_id" db:"request_id"`
	AuthorID   string    `json:"author_id" db:"author_id"`
	AuthorName string    `json:"author_name" db:"author_name"` // joined from users (nickname, fall back to email)
	Body       string    `json:"body" db:"body"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// ItineraryAdminRow is an itinerary request enriched for the planner inbox:
// the customer's email/nickname (planners need to contact them, PRD §3.3.2)
// plus a derived SLA status. Embeds ItineraryRequest so the admin handlers
// share the customer scan + add the joined columns.
type ItineraryAdminRow struct {
	ItineraryRequest `json:",inline"`            // flattened into the JSON object
	CustomerEmail     string `json:"customer_email" db:"customer_email"`
	CustomerNickname  string `json:"customer_nickname" db:"customer_nickname"`
	SLAStatus         string `json:"sla_status" db:"sla_status"` // sla_on_time|sla_approaching|sla_breached|sla_met
}

// SLA status values (derived, not stored) for the planner inbox badge.
const (
	SLAOnTime     = "sla_on_time"      // deadline > now + approaching window
	SLAApproaching = "sla_approaching" // deadline within the approaching window
	SLABreached   = "sla_breached"     // deadline < now and not yet confirmed/closed
	SLAMet        = "sla_met"          // status is past the SLA scope (confirmed/cancelled/closed)
)

// slaApproachingWindow is how far before the deadline a request is flagged
// "approaching" (PRD §3.3.2: the inbox flags requests approaching/past SLA).
// 4 hours — a single planner's notice window at MVP scale.
const SLAApproachingWindow = 4 * time.Hour

// ComputeSLAStatus derives the inbox SLA badge from status + deadline. A
// request past the SLA scope (confirmed/cancelled/closed) is "met" regardless
// of the deadline clock — the SLA only governs the pending→confirmed path.
func ComputeSLAStatus(status ItineraryStatus, slaDeadline, now time.Time) string {
	switch status {
	case StatusItineraryConfirmed, StatusItineraryCancelled, StatusItineraryClosed:
		return SLAMet
	}
	if now.After(slaDeadline) {
		return SLABreached
	}
	if slaDeadline.Sub(now) <= SLAApproachingWindow {
		return SLAApproaching
	}
	return SLAOnTime
}

// --- Planner CRM request DTOs ---

// AssignItineraryRequest is the body for POST /admin/itineraries/:id/assign.
// A nil/empty AssigneeID unassigns (PRD §3.3.2 "assignment of requests").
type AssignItineraryRequest struct {
	AssigneeID *string `json:"assignee_id" validate:"omitempty,uuid4"`
}

// ItineraryNoteRequest is the body for POST /admin/itineraries/:id/notes.
type ItineraryNoteRequest struct {
	Body string `json:"body" validate:"required,max=5000"`
}

// ItineraryReasonRequest is the optional body for the planner close/cancel
// transitions (carries the reason that becomes the cancel_reason on cancel).
type ItineraryReasonRequest struct {
	Reason string `json:"reason,omitempty" validate:"omitempty,max=500"`
}

// =============================================================================
// Quote builder + deposit payment (PRD §3.3.2, TDD §3.4 M3 #3)
//
// A planner builds a quote from the mocked option_rates CMS table (CNY line
// items → presentment via the FX pipeline + 30% deposit). The customer pays
// the deposit through the SAME payment stack as e-commerce (TDD §3.4: "Itinerary
// deposits reuse payments with order_id NULL + itinerary_quote_id"). The quote
// lifecycle: sent →(deposit webhook)→ deposit_paid|fully_paid →(planner refund)→
// cancelled. The request lifecycle rides along: processing→quoted→deposit_paid
// →confirmed (or cancelled).
//
// Money is BIGINT minor units (TDD §7). Line items are priced in CNY; the
// presentment total + deposit are FX-snapshotted at send time (like orders).
// =============================================================================

// OptionRate is one row of the mocked CMS rate table (PRD §3.3.2). The planner
// picks option_keys + qtys; the service prices each from this table.
type OptionRate struct {
	ID           int64  `json:"id" db:"id"`
	OptionKey    string `json:"option_key" db:"option_key"`
	RateCNY      int64  `json:"rate_cny" db:"rate_cny"` // fen
	Unit         string `json:"unit" db:"unit"`         // per_person|per_day|flat
	DisplayLabel string `json:"display_label" db:"display_label"`
}

// QuoteStatus is the itinerary_quotes state machine.
type QuoteStatus string

const (
	QuoteSent         QuoteStatus = "sent"
	QuoteDepositPaid  QuoteStatus = "deposit_paid"
	QuoteFullyPaid    QuoteStatus = "fully_paid"
	QuoteCancelled    QuoteStatus = "cancelled"
)

// ItineraryQuote is one active quote for a request (UNIQUE request_id; a
// re-quote replaces). Carries the immutable CNY line_items + the FX-snapshotted
// presentment total + deposit.
type ItineraryQuote struct {
	ID           int64           `json:"id" db:"id"`
	RequestID    int64           `json:"request_id" db:"request_id"`
	LineItems    json.RawMessage `json:"line_items" db:"line_items"` // []QuoteLineItem
	TotalCNY     int64           `json:"total_cny" db:"total_cny"`
	Currency     string          `json:"currency" db:"currency"`
	TotalMinor   int64           `json:"total_minor" db:"total_minor"`
	DepositMinor int64           `json:"deposit_minor" db:"deposit_minor"`
	FxRateUsed   *string         `json:"fx_rate_used,omitempty" db:"fx_rate_used"` // rendered from numeric
	Status       QuoteStatus     `json:"status" db:"status"`
	SentAt       time.Time       `json:"sent_at" db:"sent_at"`
	PaidAt       *time.Time      `json:"paid_at,omitempty" db:"paid_at"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// QuoteLineItem is one priced line of a quote (stored in itinerary_quotes.
// line_items JSONB). The planner supplies OptionKey + Qty; the service fills
// RateCNY/Unit/Label/LineCNY from option_rates.
type QuoteLineItem struct {
	OptionKey string `json:"option_key"` // canonical key (matches option_rates.option_key)
	Qty       int    `json:"qty"`        // per_person × (adults+children), per_day × duration_days, flat × 1
	RateCNY   int64  `json:"rate_cny"`   // fen (snapshot from option_rates at send time)
	Unit      string `json:"unit"`       // per_person|per_day|flat
	Label     string `json:"label"`      // display_label snapshot
	LineCNY   int64  `json:"line_cny"`   // rate_cny × qty (fen)
}

// --- Quote request DTOs ---

// QuoteLineItemInput is the planner's selection for one line (option_key + qty).
type QuoteLineItemInput struct {
	OptionKey string `json:"option_key" validate:"required"`
	Qty       int    `json:"qty" validate:"required,min=1"`
}

// SendQuoteRequest is the body for POST /admin/itineraries/:id/quote. The
// planner picks option_keys + qtys + the presentment currency. PayFull=true
// requests full payment upfront (PRD §3.3.2: "deposit or full amount").
type SendQuoteRequest struct {
	LineItems []QuoteLineItemInput `json:"line_items" validate:"required,min=1,dive"`
	Currency  string               `json:"currency" validate:"required,oneof=USD EUR GBP"`
	PayFull   bool                 `json:"pay_full"` // false → 30% deposit; true → full amount
}

// PayDepositRequest is the body for POST /itineraries/:id/pay-deposit (customer).
// PayFull=true pays the full quoted amount (matches the quote's pay_full flag).
type PayDepositRequest struct {
	Gateway string `json:"gateway" validate:"required,oneof=airwallex paypal mock"`
}

// DepositPaidResponse is the customer-facing result of pay-deposit: the gateway
// hosted checkout URL (empty in mock mode, which auto-finalizes).
type DepositPaidResponse struct {
	QuoteID   int64  `json:"quote_id"`
	HostedURL string `json:"hosted_url,omitempty"`
}

// RefundDepositRequest is the optional body for POST /admin/itineraries/:id/refund-deposit.
type RefundDepositRequest struct {
	Reason string `json:"reason,omitempty" validate:"omitempty,max=500"`
}
