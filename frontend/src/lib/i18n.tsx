/**
 * I18n + currency context (TDD §3.2 / §6).
 *
 * - Locale comes from the $locale URL segment (validated at the route);
 *   this provider is instantiated by the locale layout with that locale.
 * - Currency (USD/EUR/GBP presentment) is a UI preference persisted in a
 *   cookie. SSR renders with the default (USD — loaders also default to
 *   USD server-side, so server and first client render match); on mount
 *   the cookie is read and, when different, loaders are invalidated so
 *   client-side refetches carry the preferred currency.
 * - Prices themselves are always computed by the API (TDD §7).
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { useRouter } from '@tanstack/react-router'

import { enUS, type Catalog, type CatalogKey } from '~/i18n/en-US'
import { zhCN } from '~/i18n/zh-CN'

const CATALOGS: Record<string, Catalog> = { 'en-US': enUS, 'zh-CN': zhCN }

export interface I18nValue {
  locale: string
  currency: 'USD' | 'EUR' | 'GBP'
  setCurrency: (c: 'USD' | 'EUR' | 'GBP') => void
  t: (key: CatalogKey, vars?: Record<string, string | number>) => string
  /** format a presentment minor-unit amount in the active currency */
  price: (minor?: number, currency?: string) => string
}

const I18nContext = createContext<I18nValue | null>(null)

export function I18nProvider({ locale, children }: { locale: string; children: React.ReactNode }) {
  const [currency, setCurrencyState] = useState<'USD' | 'EUR' | 'GBP'>('USD')
  const router = useRouter()

  // after mount, adopt the stored preference (keeps SSR output stable)
  useEffect(() => {
    const stored = document.cookie.match(/(?:^|;\s*)jdz-currency=(USD|EUR|GBP)/)?.[1] as
      'USD' | 'EUR' | 'GBP' | undefined
    if (stored && stored !== 'USD') {
      setCurrencyState(stored)
      void router.invalidate()
    }
  }, [router])

  const setCurrency = useCallback(
    (c: 'USD' | 'EUR' | 'GBP') => {
      setCurrencyState(c)
      document.cookie = `jdz-currency=${c}; path=/; max-age=31536000; samesite=lax`
      void router.invalidate() // loaders refetch client-side with ?currency=
    },
    [router],
  )

  const t = useCallback(
    (key: CatalogKey, vars?: Record<string, string | number>) => {
      let s: string = CATALOGS[locale]?.[key] ?? enUS[key] ?? String(key)
      if (vars) {
        for (const [k, v] of Object.entries(vars)) {
          s = s.replaceAll(`{${k}}`, String(v))
        }
      }
      return s
    },
    [locale],
  )

  const price = useCallback(
    (minor?: number, cur?: string) =>
      new Intl.NumberFormat(locale, {
        style: 'currency',
        currency: cur ?? currency,
        minimumFractionDigits: minor != null && (minor / 100) % 1 !== 0 ? 2 : 0,
        maximumFractionDigits: 2,
      }).format(minor == null ? 0 : minor / 100),
    [currency, locale],
  )

  const value = useMemo<I18nValue>(
    () => ({ locale, currency, setCurrency, t, price }),
    [locale, currency, setCurrency, t, price],
  )

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used inside I18nProvider')
  return ctx
}
