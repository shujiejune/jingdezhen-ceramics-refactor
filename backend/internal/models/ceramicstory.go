package models

import "time"

// =============================================================================
// CeramicStory: History & Heritage (TDD §3.2 i18n pattern)
//
// The parent row holds non-localized data (start/end year, image, display
// order, timestamps). Each (story, locale) has a translation row carrying the
// localized dynasty name, slug, period label, description, characteristics,
// takeaways, meta tags, and an independent workflow status (PRD §3.1.1).
//
// The API returns a merged view (CeramicStory) so the frontend sees one flat
// object per locale. The repository JOINs parent + translation at read time.
// =============================================================================

// CeramicStory is the merged public view: parent fields + the requested
// locale's translation. Scanned from a JOIN of ceramic_stories +
// ceramic_story_translations.
type CeramicStory struct {
	ID                   int64          `json:"id" db:"id"`
	DynastyName          string         `json:"dynasty_name" db:"dynasty_name"`          // translation
	Slug                 string         `json:"slug" db:"slug"`                          // translation (per-locale unique)
	Period               *string        `json:"period,omitempty" db:"period"`            // translation
	StartYear            *int           `json:"start_year,omitempty" db:"start_year"`     // parent
	EndYear              *int           `json:"end_year,omitempty" db:"end_year"`         // parent
	Description          string         `json:"description" db:"description"`            // translation
	CharacteristicsCraft *string        `json:"characteristics_craft,omitempty" db:"characteristics_craft"`
	CharacteristicsArt   *string        `json:"characteristics_art,omitempty" db:"characteristics_art"`
	ImageURL             *string        `json:"image_url,omitempty" db:"image_url"`        // parent (non-localized media)
	Takeaways            *string        `json:"takeaways,omitempty" db:"takeaways"`
	DisplayOrder         int            `json:"display_order" db:"display_order"`         // parent
	MetaTitle            *string        `json:"meta_title,omitempty" db:"meta_title"`
	MetaDescription      *string        `json:"meta_description,omitempty" db:"meta_description"`
	Locale               string         `json:"locale" db:"locale"`                        // translation
	Status               ContentStatus  `json:"status" db:"status"`                         // translation
	PublishedAt          *time.Time     `json:"published_at,omitempty" db:"published_at"`
	CreatedAt            time.Time      `json:"created_at" db:"created_at"`                 // parent
	UpdatedAt           time.Time      `json:"updated_at" db:"updated_at"`                 // parent
}

// --- Admin / CMS DTOs (for future admin endpoints) ---------------------------

// CreateCeramicStoryData creates a new story + its first translation in one
// call. Locale defaults to en-US if empty. The parent holds the non-localized
// fields; the translation carries the localized text.
type CreateCeramicStoryData struct {
	Locale               string `json:"locale,omitempty" validate:"omitempty,len=5"`
	DynastyName          string `json:"dynasty_name" validate:"required,max=100"`
	Slug                 string `json:"slug" validate:"required,max=100"`
	Period               string `json:"period,omitempty" validate:"omitempty,max=100"`
	StartYear            *int   `json:"start_year,omitempty"`
	EndYear              *int   `json:"end_year,omitempty"`
	Description          string `json:"description" validate:"required"`
	CharacteristicsCraft string `json:"characteristics_craft,omitempty"`
	CharacteristicsArt   string `json:"characteristics_art,omitempty"`
	ImageURL             string `json:"image_url,omitempty" validate:"omitempty,url"`
	Takeaways            string `json:"takeaways,omitempty"`
	DisplayOrder         int    `json:"display_order" validate:"gte=0"`
}

// UpdateCeramicStoryData updates a story's translation (localized fields) and/or
// the parent's non-localized fields. A nil pointer = leave unchanged.
type UpdateCeramicStoryData struct {
	DynastyName          *string `json:"dynasty_name,omitempty" validate:"omitempty,max=100"`
	Slug                 *string `json:"slug,omitempty" validate:"omitempty,max=100"`
	Period               *string `json:"period,omitempty" validate:"omitempty,max=100"`
	StartYear            *int    `json:"start_year,omitempty"`
	EndYear              *int    `json:"end_year,omitempty"`
	Description          *string `json:"description,omitempty"`
	CharacteristicsCraft *string `json:"characteristics_craft,omitempty"`
	CharacteristicsArt   *string `json:"characteristics_art,omitempty"`
	ImageURL             *string `json:"image_url,omitempty" validate:"omitempty,url"`
	Takeaways            *string `json:"takeaways,omitempty"`
	DisplayOrder         *int    `json:"display_order,omitempty" validate:"omitempty,gte=0"`
}
