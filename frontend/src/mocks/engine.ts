/**
 * Mock pricing engine — the stand-in for the Go platform/fx + shipping
 * modules (TDD §7, PRD §3.2.3). Runs "server-side" inside the mock
 * transport; the frontend never re-implements any of this math.
 */
import { FX_MARKUP, FX_RATES, SHIPPING_TIERS, type Translation } from './data'

export type MockLocale = 'en-US' | 'zh-CN'

/** Resolve a bilingual record to the requested locale (zh-CN → en-US fallback). */
export function pick<T>(t: Translation<T>, locale: MockLocale): T {
  return locale === 'zh-CN' ? (t.zhCN ?? t.enUS) : t.enUS
}

/** Effective rate (CNY per 1 unit) after the default 2% markup. */
export function effectiveRate(currency: 'USD' | 'EUR' | 'GBP'): number {
  return FX_RATES[currency].rate_to_cny * (1 - FX_MARKUP)
}

/**
 * PRD §3.2.3 rounding: under 100 → ceil to next 0.50; ≥ 100 → ceil to
 * the next whole unit. (€183.47 → €184.)
 */
export function roundPresentment(major: number): number {
  const step = major < 100 ? 0.5 : 1
  // guard float noise: only round up when meaningfully above the step
  return Math.ceil((major - 1e-9) / step) * step
}

/** CNY minor units → presentment minor units. */
export function convertMinor(cnyMinor: number, currency: 'USD' | 'EUR' | 'GBP'): number {
  const majorCny = cnyMinor / 100
  const major = majorCny / effectiveRate(currency)
  return Math.round(roundPresentment(major) * 100)
}

/** Shipping quote from the per-country tier table (PRD §3.2.3). */
export function quoteShipping(
  country: string,
  weightGrams: number,
): { fee_cny: number; blocked_reason?: 'unshippable' | 'overweight' } {
  const tiers = SHIPPING_TIERS.filter((t) => t.country === country)
  if (tiers.length === 0) {
    return { fee_cny: 0, blocked_reason: 'unshippable' }
  }
  const fit = tiers.find((t) => weightGrams <= t.max_weight_grams)
  if (!fit) {
    return { fee_cny: 0, blocked_reason: 'overweight' }
  }
  return { fee_cny: fit.fee_cny }
}
