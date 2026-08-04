package models

import "time"

// ConsentKind enumerates the consent categories the platform records (TDD §3.4,
// PRD §4.3). `privacy_policy` and `tos` are mandatory for registration/checkout;
// the cookie kinds gate analytics/marketing respectively.
type ConsentKind string

const (
	ConsentKindPrivacyPolicy   ConsentKind = "privacy_policy"
	ConsentKindToS             ConsentKind = "tos"
	ConsentKindCookieAnalytics ConsentKind = "cookie_analytics"
	ConsentKindCookieMarketing ConsentKind = "cookie_marketing"
)

// ConsentRecord is an immutable append-only row in the GDPR consent ledger.
// A `granted=false` row records a withdrawal/refusal (GDPR requires both).
// `UserID` is nil for anonymous visitor consent (pre-signup cookie banner).
// `IPHash` is an HMAC of the visitor's IP (no raw IPs stored — GDPR
// minimisation, TDD §11); the HMAC key rotates daily so the hash is only
// useful for short-term dedup/audit, not re-identification.
type ConsentRecord struct {
	ID         int64       `json:"id" db:"id"`
	UserID     *string     `json:"user_id,omitempty" db:"user_id"` // NULL = anonymous visitor
	Kind       ConsentKind `json:"kind" db:"kind"`
	DocVersion string      `json:"doc_version" db:"doc_version"`
	Granted    bool        `json:"granted" db:"granted"`
	IPHash     *string     `json:"-" db:"ip_hash"` // never exposed in JSON (minimisation)
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
}

// RecordConsentRequest is the body for POST /consent. `user_id` is never
// client-supplied — it is derived from the auth token (or left null for an
// anonymous visitor). `ip_hash` is computed server-side from the request IP.
type RecordConsentRequest struct {
	Kind       ConsentKind `json:"kind" validate:"required,oneof=privacy_policy tos cookie_analytics cookie_marketing"`
	DocVersion string      `json:"doc_version" validate:"required,max=20"`
	Granted    bool        `json:"granted"`
}
