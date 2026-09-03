/**
 * Consent provider — tracks the visitor's cookie-consent choices in
 * localStorage and records them via POST /consent (public, no auth).
 * The consent state gates analytics events: pageview/event calls are
 * silently dropped (204 from the backend) when cookie_analytics is not
 * granted (PRD §3.5, TDD §4.3).
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { api } from '~/lib/api'
import type { ConsentKind } from '~/lib/types'

const CONSENT_KEY = 'jdz.consent'
const DOC_VERSION = '1.0'

export interface ConsentChoices {
  privacy_policy: boolean
  tos: boolean
  cookie_analytics: boolean
  cookie_marketing: boolean
}

type StoredConsent = ConsentChoices & { recorded_at: string }

const DEFAULTS: ConsentChoices = {
  privacy_policy: false,
  tos: false,
  cookie_analytics: false,
  cookie_marketing: false,
}

const ALL_KINDS: ConsentKind[] = ['privacy_policy', 'tos', 'cookie_analytics', 'cookie_marketing']

export interface ConsentValue {
  /** whether the banner should be shown (no choice recorded yet) */
  needsBanner: boolean
  choices: ConsentChoices
  /** true when the visitor has granted analytics cookies */
  analyticsGranted: boolean
  /** accept all + dismiss the banner */
  acceptAll: () => void
  /** accept only essential (privacy + tos) + dismiss */
  acceptEssential: () => void
  /** re-open the banner from the footer "Cookie preferences" link */
  reopen: () => void
}

const ConsentContext = createContext<ConsentValue | null>(null)

function read(): StoredConsent | null {
  if (typeof localStorage === 'undefined') return null
  try {
    const raw = localStorage.getItem(CONSENT_KEY)
    return raw ? (JSON.parse(raw) as StoredConsent) : null
  } catch {
    return null
  }
}

export function ConsentProvider({ children }: { children: React.ReactNode }) {
  const [stored, setStored] = useState<StoredConsent | null>(null)
  const [showBanner, setShowBanner] = useState(false)

  useEffect(() => {
    const s = read()
    setStored(s)
    setShowBanner(s === null)
  }, [])

  const record = useCallback((choices: ConsentChoices) => {
    const entry: StoredConsent = { ...choices, recorded_at: new Date().toISOString() }
    try {
      localStorage.setItem(CONSENT_KEY, JSON.stringify(entry))
    } catch {
      /* storage blocked — non-fatal */
    }
    setStored(entry)
    setShowBanner(false)
    // fire-and-forget: record each kind via the public POST /consent
    for (const kind of ALL_KINDS) {
      void api
        .recordConsent({ kind, doc_version: DOC_VERSION, granted: choices[kind] })
        .catch(() => {
          /* 204 = silently dropped, non-fatal */
        })
    }
  }, [])

  const acceptAll = useCallback(
    () =>
      record({ privacy_policy: true, tos: true, cookie_analytics: true, cookie_marketing: true }),
    [record],
  )

  const acceptEssential = useCallback(
    () => record({ ...DEFAULTS, privacy_policy: true, tos: true }),
    [record],
  )

  const reopen = useCallback(() => setShowBanner(true), [])

  const value = useMemo<ConsentValue>(
    () => ({
      needsBanner: showBanner,
      choices: stored ?? DEFAULTS,
      analyticsGranted: stored?.cookie_analytics ?? false,
      acceptAll,
      acceptEssential,
      reopen,
    }),
    [stored, showBanner, acceptAll, acceptEssential, reopen],
  )

  return <ConsentContext.Provider value={value}>{children}</ConsentContext.Provider>
}

export function useConsent(): ConsentValue {
  const ctx = useContext(ConsentContext)
  if (!ctx) throw new Error('useConsent must be used inside ConsentProvider')
  return ctx
}
