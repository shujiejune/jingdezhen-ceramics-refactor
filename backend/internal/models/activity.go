package models

import "time"

// =============================================================================
// Activity: Destinations & Local Lifestyle (TDD §3.2 i18n pattern, PRD §3.1.2/§3.1.3)
//
// Consolidates the inherited `activities` (card) + `articles` (detail) pair into
// one content entity. The parent holds non-localized data: `type` (Destination
// vs Local-Lifestyle) + destination location fields (lat, lng, address,
// opening_info — NULL for Local-Lifestyle rows). The per-locale translation
// carries title, slug, brief (card), body (full article), meta, + an independent
// workflow status (PRD §3.1.1). The API returns a merged view.
// =============================================================================

// Activity is the merged public view: parent fields + the requested locale's
// translation. Scanned from a JOIN of activities + activity_translations.
type Activity struct {
	ID                int64          `json:"id" db:"id"`
	Type              string         `json:"type" db:"type"` // parent: 'Destination', 'Local Lifestyle', etc.
	// Destination location (parent, non-localized; NULL for non-destination types).
	Lat               *float64       `json:"lat,omitempty" db:"lat"`
	Lng               *float64       `json:"lng,omitempty" db:"lng"`
	Address           *string        `json:"address,omitempty" db:"address"`
	// Translation fields.
	Slug              string         `json:"slug" db:"slug"`
	Title             string         `json:"title" db:"title"`
	BriefIntroduction *string        `json:"brief_introduction,omitempty" db:"brief_introduction"`
	Body              *string        `json:"body,omitempty" db:"body"`
	MetaTitle         *string        `json:"meta_title,omitempty" db:"meta_title"`
	MetaDescription   *string        `json:"meta_description,omitempty" db:"meta_description"`
	Locale            string         `json:"locale" db:"locale"`
	Status            ContentStatus  `json:"status" db:"status"`
	PublishedAt       *time.Time     `json:"published_at,omitempty" db:"published_at"`
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
}

// Article is retained as a type alias to Activity for backward compatibility
// with any callers that reference the old name; the consolidated entity is an
// Activity. (No separate articles table is read at runtime anymore.)
type Article = Activity
