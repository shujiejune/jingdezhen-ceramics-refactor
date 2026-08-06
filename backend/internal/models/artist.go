package models

import "time"

// =============================================================================
// Artist: ceramic artist profiles (TDD §3.2 i18n pattern, PRD §3.1.3 / §3.2.1)
//
// The parent row holds non-localized data: avatar/portrait photo, optional link
// to a platform user (UUID), display order, timestamps. Each (artist, locale)
// has a translation row carrying the localized name, slug, biography, meta
// tags, and an independent workflow status (PRD §3.1.1). Artist profiles are
// cross-linked from Art Gallery products (PRD §3.2.1).
//
// The API returns a merged view (Artist) so the frontend sees one flat object
// per locale. The repository JOINs parent + translation at read time.
// =============================================================================

// Artist is the merged public view: parent fields + the requested locale's
// translation. Scanned from a JOIN of artists + artist_translations.
type Artist struct {
	ID           int64   `json:"id" db:"id"`
	AvatarURL    *string `json:"avatar_url,omitempty" db:"avatar_url"` // parent (non-localized media)
	UserID       *string `json:"user_id,omitempty" db:"user_id"`       // parent: link to users.id (UUID)
	DisplayOrder int     `json:"display_order" db:"display_order"`     // parent
	// Translation fields.
	Name            string        `json:"name" db:"name"`         // translation (localized display name)
	Slug            string        `json:"slug" db:"slug"`         // translation (per-locale unique)
	Bio             *string       `json:"bio,omitempty" db:"bio"` // translation
	MetaTitle       *string       `json:"meta_title,omitempty" db:"meta_title"`
	MetaDescription *string       `json:"meta_description,omitempty" db:"meta_description"`
	Locale          string        `json:"locale" db:"locale"`
	Status          ContentStatus `json:"status" db:"status"`
	PublishedAt     *time.Time    `json:"published_at,omitempty" db:"published_at"`
	// Alternates: hreflang map (PRD §4.4), locale → slug for every other
	// published translation of this artist. Populated by the service on the
	// detail view.
	Alternates map[string]string `json:"alternates,omitempty" db:"-"`
	CreatedAt       time.Time     `json:"created_at" db:"created_at"` // parent
	UpdatedAt       time.Time     `json:"updated_at" db:"updated_at"` // parent
}

// --- Admin / CMS DTOs ---

// CreateArtistData creates a new artist + its first translation in one call.
// Locale defaults to en-US if empty. The parent holds the non-localized fields
// (avatar, user link, display order); the translation carries the localized
// text (name, slug, bio, meta).
type CreateArtistData struct {
	Locale          string `json:"locale,omitempty" validate:"omitempty,len=5"`
	Name            string `json:"name" validate:"required,max=255"`
	Slug            string `json:"slug" validate:"required,max=100"`
	Bio             string `json:"bio,omitempty"`
	MetaTitle       string `json:"meta_title,omitempty" validate:"omitempty,max=255"`
	MetaDescription string `json:"meta_description,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty" validate:"omitempty,url"`
	UserID          string `json:"user_id,omitempty"` // optional: link to a platform user
	DisplayOrder    int    `json:"display_order" validate:"gte=0"`
}

// UpdateArtistData updates an artist's translation (localized fields) and/or
// the parent's non-localized fields. A nil pointer = leave unchanged.
type UpdateArtistData struct {
	Name            *string `json:"name,omitempty" validate:"omitempty,max=255"`
	Slug            *string `json:"slug,omitempty" validate:"omitempty,max=100"`
	Bio             *string `json:"bio,omitempty"`
	MetaTitle       *string `json:"meta_title,omitempty" validate:"omitempty,max=255"`
	MetaDescription *string `json:"meta_description,omitempty"`
	AvatarURL       *string `json:"avatar_url,omitempty" validate:"omitempty,url"`
	UserID          *string `json:"user_id,omitempty"`
	DisplayOrder    *int    `json:"display_order,omitempty" validate:"omitempty,gte=0"`
}
