import { describe, expect, it } from 'vitest'

import { formatMinor, formatWeight } from '~/lib/money'

describe('formatMinor (display only — never math)', () => {
  it('formats whole-unit amounts without decimals', () => {
    expect(formatMinor(3_107_00, 'USD', 'en-US')).toBe('$3,107')
    expect(formatMinor(5_800_00, 'CNY', 'en-US')).toBe('CN¥5,800')
  })

  it('keeps .50-style halves (PRD rounding only produces .00/.50)', () => {
    expect(formatMinor(3_107_50, 'USD', 'en-US')).toBe('$3,107.50')
  })

  it('formats EUR and GBP', () => {
    expect(formatMinor(184_00, 'EUR', 'en-US')).toBe('€184')
    expect(formatMinor(3_958_00, 'GBP', 'en-US')).toBe('£3,958')
  })

  it('renders a dash for missing amounts', () => {
    expect(formatMinor(undefined, 'USD', 'en-US')).toBe('—')
    expect(formatMinor(null, 'USD', 'en-US')).toBe('—')
  })

  it('locale-aware (zh-CN uses US$ prefix)', () => {
    expect(formatMinor(3_107_00, 'USD', 'zh-CN')).toContain('3,107')
  })
})

describe('formatWeight', () => {
  it('formats kilograms', () => {
    expect(formatWeight(1900, 'en-US')).toBe('1.9 kg')
    expect(formatWeight(12500, 'en-US')).toBe('13 kg') // ≥10 kg shows whole units
  })
})
