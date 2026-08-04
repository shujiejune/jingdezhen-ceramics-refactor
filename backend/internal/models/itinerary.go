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
