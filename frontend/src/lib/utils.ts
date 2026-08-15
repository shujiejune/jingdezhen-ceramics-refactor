/** Join class names, dropping falsy values. */
export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}

/** "en-US" → "en" (for <html lang>). */
export function localeToHtmlLang(locale: string): string {
  return locale.split('-')[0] ?? locale
}

export const SUPPORTED_LOCALES = ['en-US', 'zh-CN'] as const
export type Locale = (typeof SUPPORTED_LOCALES)[number]

export const SUPPORTED_CURRENCIES = ['USD', 'EUR', 'GBP'] as const
export type Currency = (typeof SUPPORTED_CURRENCIES)[number]

export function isLocale(v: string): v is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(v)
}

export function isCurrency(v: string): v is Currency {
  return (SUPPORTED_CURRENCIES as readonly string[]).includes(v)
}

/** Clamp a number into [min, max]. */
export function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n))
}

/** Deterministic small PRNG (mulberry32) — used by procedural artwork figures. */
export function seededRandom(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** ISO date (YYYY-MM-DD) → locale-formatted date. */
export function formatDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(new Date(iso))
}

/** "2026-08-14T09:30:00Z" → locale date+time. */
export function formatDateTime(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(iso))
}

/** Hydrate-safety: true only in the browser. */
export const isBrowser = typeof window !== 'undefined'

/**
 * Loader-side presentment currency: on the client, read the
 * jdz-currency cookie; during SSR return undefined (default USD — the
 * I18nProvider invalidates loaders after mount when the cookie differs).
 */
export async function loaderCurrency(): Promise<'USD' | 'EUR' | 'GBP'> {
  if (typeof document !== 'undefined') {
    const m = document.cookie.match(/(?:^|;\s*)jdz-currency=(USD|EUR|GBP)/)
    if (m) return m[1] as 'USD' | 'EUR' | 'GBP'
  }
  return 'USD' // API default (TDD §5.1)
}
