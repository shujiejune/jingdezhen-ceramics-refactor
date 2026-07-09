package models

import "time"

// TwoFARecord is the persisted TOTP enrollment state for a user (TDD §3.4,
// PRD §4.3). The TOTP secret is encrypted at rest (AES-GCM with an app key),
// so this struct exposes only the EncryptedSecret blob — never the raw secret.
type TwoFARecord struct {
	UserID          string     `json:"-" db:"user_id"`
	EncryptedSecret []byte     `json:"-" db:"totp_secret_enc"`
	Enabled         bool       `json:"enabled" db:"enabled"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// TwoFAEnrollResponse is returned by the enroll endpoint. It carries the
// otpauth:// URI (renderable as a QR code) and the raw secret for manual entry.
// The raw secret is shown ONCE here; only the encrypted form is stored.
type TwoFAEnrollResponse struct {
	OTPAuthURI string `json:"otpauth_uri"` // otpauth://totp/... — render as QR
	Secret     string `json:"secret"`      // for manual entry
}

// EnrollTwoFARequest is the body for POST /profile/2fa/enroll (start).
type EnrollTwoFARequest struct {
	// Issuer/account labels for the otpauth URI; defaults applied if empty.
	Issuer  string `json:"issuer,omitempty"`
	Account string `json:"account,omitempty"`
}

// ConfirmTwoFARequest is the body for POST /profile/2fa/confirm — prove the
// user can generate a valid code from the staged secret, which then enables 2FA.
type ConfirmTwoFARequest struct {
	Code string `json:"code" validate:"required,len=6"` // 6-digit TOTP code
}

// VerifyTwoFARequest is the body for POST /auth/2fa/verify — complete a login
// that is pending 2FA, using the pending token issued when 2FA was challenged.
type VerifyTwoFARequest struct {
	PendingToken string `json:"pending_token" validate:"required"`
	Code         string `json:"code" validate:"required,len=6"`
}

// PendingTwoFAEnrollRequest is the body for POST /auth/2fa/pending-enroll —
// the must-enroll flow for super_admin. The pending token (from the blocked
// login) is the credential; optional issuer/account labels the otpauth URI.
type PendingTwoFAEnrollRequest struct {
	PendingToken string `json:"pending_token" validate:"required"`
	Issuer       string `json:"issuer,omitempty"`
	Account      string `json:"account,omitempty"`
}

// PendingTwoFAConfirmRequest is the body for POST /auth/2fa/pending-confirm —
// verify the first code, enable 2FA, and complete login (mint the real JWT).
type PendingTwoFAConfirmRequest struct {
	PendingToken string `json:"pending_token" validate:"required"`
	Code         string `json:"code" validate:"required,len=6"`
}
