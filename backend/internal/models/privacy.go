package models

import "time"

// UserDataExport is the machine-readable GDPR data-portability package assembled
// for `GET /profile/export` (PRD §4.3). It bundles every piece of personal data
// the platform holds about the requesting user. Fields are omitted from the JSON
// when empty (omitempty) so a customer with no favorites doesn't get `null`.
//
// Future commerce/travel modules (orders, cart, itineraries, certificates) will
// add their sections here as they land (M2/M3).
type UserDataExport struct {
	// Request metadata.
	ExportedAt time.Time `json:"exported_at"`
	UserID     string    `json:"user_id"`
	Locale     string    `json:"locale,omitempty"`

	// Profile (the users row minus secrets).
	Profile *User `json:"profile"`

	// Shipping address book.
	Addresses []UserAddress `json:"addresses,omitempty"`

	// Consent ledger (privacy_policy / tos / cookie_* history).
	ConsentRecords []ConsentRecord `json:"consent_records,omitempty"`

	// 2FA enrollment metadata (the encrypted TOTP secret is NOT exported; only
	// the enabled flag + confirmation timestamp are surfaced for transparency).
	TwoFA *TwoFAExport `json:"two_fa,omitempty"`

	// Wishlist (favorited SKUs; was user_favorite_artworks, now keyed on SKU
	// since the 000013 evolve-artworks migration).
	Wishlist []FavoriteExport `json:"wishlist,omitempty"`

	// Notifications received.
	Notifications []Notification `json:"notifications,omitempty"`
}

// TwoFAExport is the non-secret 2FA summary included in a data export.
type TwoFAExport struct {
	Enabled              bool       `json:"enabled"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	BackupCodesRemaining int        `json:"backup_codes_remaining"`
}

// FavoriteExport is one row of the user's wishlist (the SKU detail is joined
// for readability; the favorited_at timestamp is the personal datum).
type FavoriteExport struct {
	SKUID       int64     `json:"sku_id"`
	SKUCode     string    `json:"sku_code"`
	FavoritedAt time.Time `json:"favorited_at"`
}

// DeleteAccountRequest is the body for POST /privacy/delete-account. The
// `confirm` field requires a deliberate typed string to guard against accidental
// erasure (the action is irreversible).
type DeleteAccountRequest struct {
	Confirm string `json:"confirm" validate:"required,eq=DELETE"`
}
