package models

// Tag is a product discovery facet (PRD §3.2.1 line 173, TDD §3.2 line 130).
//
// Evolved from the inherited (dead) monolingual `tags(name)` table: the parent
// now holds a language-neutral canonical `key` (lowercase kebab-case, e.g.
// `hand-painted`); per-locale display names live in `tag_translations`. `Name`
// is the locale-resolved display name (joined at read time with an en-US → key
// fallback via COALESCE) — it is NOT a column on the parent `tags` row.
//
// Tags are taxonomy, NOT editorial content: no draft/publish workflow status
// lives on tag_translations (unlike artist/product translations). A tag is
// visible iff it is attached to a published product.
type Tag struct {
	ID   int64  `json:"id" db:"id"`
	Key  string `json:"key" db:"key"`
	Name string `json:"name" db:"name"` // locale-resolved (COALESCE in the JOIN)
}
