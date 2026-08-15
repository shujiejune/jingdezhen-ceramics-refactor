/**
 * Money display (TDD §7, PRD §3.2.3).
 *
 * All amounts arrive as integer minor units. This module ONLY formats —
 * FX conversion and the PRD rounding rule (<100 → ceil .50; ≥100 → ceil
 * 1.00) happen server-side (mock transport here, platform/fx in Go).
 * `Number()` is used strictly inside the formatter for display; never
 * for arithmetic.
 */

const minorDivisors: Record<string, number> = {
  USD: 100,
  EUR: 100,
  GBP: 100,
  CNY: 100,
}

export function formatMinor(
  minor: number | undefined | null,
  currency: string,
  locale: string,
): string {
  if (minor == null) return '—'
  const divisor = minorDivisors[currency] ?? 100
  const value = Number(minor) / divisor
  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    // keep whole-unit prices clean (PRD rounding yields .00/.50 only)
    minimumFractionDigits: value % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(value)
}

/** Compact weight for cart/checkout summaries. */
export function formatWeight(grams: number, locale: string): string {
  const kg = grams / 1000
  return new Intl.NumberFormat(locale, {
    style: 'unit',
    unit: 'kilogram',
    maximumFractionDigits: kg < 10 ? 1 : 0,
  }).format(kg)
}
