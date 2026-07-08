package models

import "time"

// UserAddress is a single shipping address belonging to a user (PRD §3.5).
// `country` is ISO 3166-1 alpha-2 and drives the shipping-fee calculator
// (PRD §3.2.3). At most one address per user may be the default.
type UserAddress struct {
	ID         int64     `json:"id" db:"id"`
	UserID     string    `json:"-" db:"user_id"` // derived from the auth token, never client-supplied
	Recipient  string    `json:"recipient" db:"recipient"`
	Line1      string    `json:"line1" db:"line1"`
	Line2      *string   `json:"line2,omitempty" db:"line2"`
	City       string    `json:"city" db:"city"`
	Region     *string   `json:"region,omitempty" db:"region"`
	PostalCode *string   `json:"postal_code,omitempty" db:"postal_code"`
	Country    string    `json:"country" db:"country"` // ISO 3166-1 alpha-2
	Phone      *string   `json:"phone,omitempty" db:"phone"`
	IsDefault  bool      `json:"is_default" db:"is_default"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// CreateAddressRequest is the body for creating a new shipping address.
type CreateAddressRequest struct {
	Recipient  string `json:"recipient" validate:"required,max=255"`
	Line1      string `json:"line1" validate:"required,max=255"`
	Line2      string `json:"line2,omitempty" validate:"omitempty,max=255"`
	City       string `json:"city" validate:"required,max=100"`
	Region     string `json:"region,omitempty" validate:"omitempty,max=100"`
	PostalCode string `json:"postal_code,omitempty" validate:"omitempty,max=30"`
	Country    string `json:"country" validate:"required,len=2"` // alpha-2, e.g. "US"
	Phone      string `json:"phone,omitempty" validate:"omitempty,max=30"`
	IsDefault  bool   `json:"is_default"`
}

// UpdateAddressRequest replaces all editable fields of an address (PUT semantics).
// `is_default` is managed separately via the set-default endpoint, but is honoured
// here so a full-replace client can set it in one call.
type UpdateAddressRequest struct {
	Recipient  string `json:"recipient" validate:"required,max=255"`
	Line1      string `json:"line1" validate:"required,max=255"`
	Line2      string `json:"line2,omitempty" validate:"omitempty,max=255"`
	City       string `json:"city" validate:"required,max=100"`
	Region     string `json:"region,omitempty" validate:"omitempty,max=100"`
	PostalCode string `json:"postal_code,omitempty" validate:"omitempty,max=30"`
	Country    string `json:"country" validate:"required,len=2"`
	Phone      string `json:"phone,omitempty" validate:"omitempty,max=30"`
	IsDefault  bool   `json:"is_default"`
}
