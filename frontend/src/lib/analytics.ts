/**
 * Analytics client — sends pageview and custom events to the backend
 * via POST /analytics/events. The backend checks cookie_analytics
 * consent by IP hash and returns 204 (silently dropped) when not
 * granted. This client never retries — 204 means the event was
 * intentionally dropped (PRD §4.3, TDD §4.3).
 *
 * Call `trackPageview(path, locale)` on route change and
 * `trackEvent(name, locale, props)` for custom events.
 * Both are fire-and-forget; errors are swallowed.
 */
import { api } from '~/lib/api'

let pendingPath: string | null = null

export function trackPageview(path: string, locale?: string) {
  pendingPath = path
  // fire-and-forget; 204 = consent-gated drop, 201 = recorded
  void api.trackAnalytics({ kind: 'pageview', path, locale }).catch(() => {
    /* network error — non-fatal, never retry */
  })
}

export function trackEvent(name: string, locale?: string, props?: Record<string, unknown>) {
  void api
    .trackAnalytics({ kind: 'event', path: pendingPath ?? '/', name, locale, props })
    .catch(() => {
      /* non-fatal */
    })
}

/** Track an itinerary form view (funnel contract: pageview → form_view → submit). */
export function trackItineraryFormView(locale?: string) {
  trackEvent('itinerary_form_view', locale)
}
