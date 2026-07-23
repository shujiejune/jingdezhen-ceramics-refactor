package models

// =============================================================================
// i18n content infrastructure (TDD §3.2, §3.3, PRD §3.5.1)
//
// Shared types for the per-locale translation-table pattern. Every localized
// content entity `X` gets a companion `x_translations` table whose rows carry
// the per-locale title/slug/content/meta + an independent workflow status
// (PRD §3.1.1). These constants and types are used by every content module so
// the workflow, locale, and rich-content shape are consistent across the API.
// =============================================================================

// --- Locales (BCP 47) --------------------------------------------------------

const (
	LocaleEnUS = "en-US" // default, launch
	LocaleZhCN = "zh-CN" // launch
	// Future locales (PRD §3.5.1) — architecture must be locale-extensible, so
	// adding a locale here + a translation row is all that's required.
	LocaleZhHant = "zh-Hant"
	LocaleJa     = "ja"
	LocaleFr     = "fr"
)

// SupportedLocales is the set of locales the platform renders. Querying or
// writing a translation in a locale outside this set returns ErrInvalidLocale.
// (Use LaunchLocales for the narrower "must-have content" set if needed.)
var SupportedLocales = []string{LocaleEnUS, LocaleZhCN}

// IsSupportedLocale reports whether a BCP 47 key is in the supported set.
func IsSupportedLocale(locale string) bool {
	for _, l := range SupportedLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// DefaultLocale is the fallback when a request specifies none (TDD §5.1).
const DefaultLocale = LocaleEnUS

// --- Content workflow status (lives on the translation row) ----------------

// ContentStatus is the editorial workflow state of a single translation
// (PRD §3.1.1). Per-locale independence means the en-US translation can be
// published while the zh-CN translation is still in_review.
type ContentStatus string

const (
	StatusDraft     ContentStatus = "draft"
	StatusInReview  ContentStatus = "in_review"
	StatusPublished ContentStatus = "published"
	StatusRejected  ContentStatus = "rejected"
)

// AllContentStatuses is the exhaustive set, matching the DB CHECK constraint.
var AllContentStatuses = []ContentStatus{StatusDraft, StatusInReview, StatusPublished, StatusRejected}

// IsPublishedStatus reports whether a status is publicly visible.
func IsPublishedStatus(s ContentStatus) bool { return s == StatusPublished }

// --- Rich content blocks (TDD §3.3) -----------------------------------------

// ContentBlock is one element of the ordered `content JSONB` array stored on a
// translation row. Portable-text style: media references are FKs to
// media_assets (integrity + CDN URL resolution at render time), not raw URLs.
// Rendering to HTML/SSR happens in the frontend; the API stores + returns these.
type ContentBlock struct {
	Type     string `json:"type"`               // paragraph | heading | image | video
	Text     string `json:"text,omitempty"`     // paragraph/heading body
	Level    int    `json:"level,omitempty"`    // heading level (1-6)
	AssetID *int64 `json:"asset_id,omitempty"`  // image/video → media_assets.id
	Caption  string `json:"caption,omitempty"`  // image caption
}

// Valid block types.
const (
	BlockTypeParagraph = "paragraph"
	BlockTypeHeading   = "heading"
	BlockTypeImage     = "image"
	BlockTypeVideo     = "video"
)

// IsValidBlockType reports whether a block type is recognised.
func IsValidBlockType(t string) bool {
	switch t {
	case BlockTypeParagraph, BlockTypeHeading, BlockTypeImage, BlockTypeVideo:
		return true
	}
	return false
}

// --- Shared translation query/result types -----------------------------------

// LocaleQuery is the common request shape for public content endpoints: a
// locale plus optional slug. The service validates the locale and filters to
// published translations only (TDD §5.1 — locale from URL prefix / ?locale=).
type LocaleQuery struct {
	Locale string `json:"-"`              // BCP 47; defaults to DefaultLocale if empty
	Slug   string `json:"slug,omitempty"` // per-locale slug (unique within locale)
}

// TranslationMeta is the common localized metadata returned by every content
// endpoint: title, slug, meta tags, and the workflow status (so the frontend
// can show an editor's own draft vs. a reader's published view).
type TranslationMeta struct {
	Locale           string        `json:"locale"`
	Slug             string        `json:"slug"`
	Title            string        `json:"title"`
	MetaTitle        *string       `json:"meta_title,omitempty"`
	MetaDescription  *string       `json:"meta_description,omitempty"`
	Status           ContentStatus `json:"status"`
	PublishedAt      *string       `json:"published_at,omitempty"` // ISO 8601; nil if not published
}

// --- Workflow transition errors ---------------------------------------------
// (ErrInvalidLocale, ErrInvalidWorkflowTransition live in errors.go with the
// other domain errors for centralized HTTP-status mapping.)
