// Package i18ncontent holds the cross-cutting helpers for the per-locale
// translation-table pattern (TDD §3.2, §3.3). Content modules (history,
// destinations, local lifestyle, artist profiles, products) depend on these
// so the content workflow, locale handling, and slug resolution are consistent
// across every localized endpoint.
//
// The translation-table pattern: every localized entity X has a parent row
// (non-localized data: media, coordinates, order) and an x_translations row
// per (entity, locale) carrying title/slug/content/meta + an independent
// workflow status (PRD §3.1.1).
package i18ncontent

import (
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
)

// --- Locale handling ---------------------------------------------------------

// NormalizeLocale returns the locale to use for a request: the requested locale
// if supported, otherwise the default. Returns an error (ErrInvalidLocale) only
// when the caller passes validate=true and the requested locale is non-empty
// but unsupported — use this for admin/CMS writes where an unknown locale is a
// client error, not a silent fallback. For public reads, pass validate=false
// so a typo falls back to the default locale rather than 404.
func NormalizeLocale(requested string, validate bool) (string, error) {
	if requested == "" {
		return models.DefaultLocale, nil
	}
	if models.IsSupportedLocale(requested) {
		return requested, nil
	}
	if validate {
		return "", models.ErrInvalidLocale
	}
	return models.DefaultLocale, nil
}

// --- Content workflow state machine (TDD §4.2, §3.2) -------------------------
//
// draft  --(editor submits)-->    in_review
// in_review --(super_admin approves)--> published   (fires sitemap rebuild)
// in_review --(super_admin rejects, w/ comments)--> rejected
// rejected --(editor edits)--> draft
// published --(super_admin unpublish)--> draft       (PRD: unpublish = Super Admin)
//
// Only the Super Administrator may approve/publish or unpublish (PRD §3.1.1).
// Editors may submit (draft→in_review) and edit rejected content back to draft.

// WorkflowActor identifies who is acting on a translation, which constrains
// the allowed transitions. SuperAdminRole is the models.RoleSuperAdmin key.
type WorkflowActor string

const (
	ActorEditor     WorkflowActor = "editor"
	ActorSuperAdmin WorkflowActor = "super_admin"
)

// Transition validates + applies a content-workflow state change for the
// given actor. It returns the new status, or ErrInvalidWorkflowTransition if
// the (from → to) move is not permitted for that actor. The service layer calls
// this before UPDATEing the translation row.
//
// Callers pass the super_admin role explicitly (the RBAC middleware already
// gates the route; this is the in-service enforcement so a logic bug in a
// handler can't bypass the publish gate).
func Transition(from, to models.ContentStatus, actor WorkflowActor) (models.ContentStatus, error) {
	if !allowed(from, to, actor) {
		return from, fmt.Errorf("%w: %s → %s by %s",
			models.ErrInvalidWorkflowTransition, from, to, actor)
	}
	return to, nil
}

// allowed is the transition matrix. Defined here (not in the DB) because it
// encodes actor rules that the DB CHECK constraint can't express.
func allowed(from, to models.ContentStatus, actor WorkflowActor) bool {
	switch {
	// Editors: submit a draft for review, or move rejected back to draft.
	case actor == ActorEditor && from == models.StatusDraft && to == models.StatusInReview:
		return true
	case actor == ActorEditor && from == models.StatusRejected && to == models.StatusDraft:
		return true

	// Super admin: approve or reject an in_review translation.
	case actor == ActorSuperAdmin && from == models.StatusInReview && to == models.StatusPublished:
		return true
	case actor == ActorSuperAdmin && from == models.StatusInReview && to == models.StatusRejected:
		return true

	// Super admin: unpublish (published → draft). Editors cannot unpublish.
	case actor == ActorSuperAdmin && from == models.StatusPublished && to == models.StatusDraft:
		return true

	// Super admin: also allowed to submit/withdraw drafts (can act as editor).
	case actor == ActorSuperAdmin && from == models.StatusDraft && to == models.StatusInReview:
		return true
	case actor == ActorSuperAdmin && from == models.StatusRejected && to == models.StatusDraft:
		return true
	}
	return false
}

// CanEdit reports whether a translation in the given status is editable by the
// actor (i.e. the title/slug/content/meta may be changed). A published
// translation is only editable by the super admin (who must unpublish first
// in a separate transition, but the CMS may allow direct edits by super admin
// that implicitly republish). Editors may edit drafts and rejected rows.
func CanEdit(status models.ContentStatus, actor WorkflowActor) bool {
	if actor == ActorSuperAdmin {
		return true // super admin can edit anything
	}
	return status == models.StatusDraft || status == models.StatusRejected
}

// --- Public query filter ------------------------------------------------------

// PublishedFilter is the WHERE clause fragment every public content endpoint
// appends so only published translations are visible to readers. Content
// modules interpolate this into their translation SELECTs (e.g. joining
// x_translations WHERE status = 'published' AND locale = $1).
const PublishedFilter = "status = 'published'"

// --- Slug resolution ---------------------------------------------------------

// ResolveSlug picks the best slug for a (entity, locale) lookup: the requested
// slug as-is (slugs are unique per locale, so a hit is authoritative). The
// service layer uses this for clarity — there's no fallback across locales
// because a missing translation should 404, not silently serve another
// language (PRD §3.5.1: content is bilingual, not auto-translated).
func ResolveSlug(slug string) string {
	return slug
}
