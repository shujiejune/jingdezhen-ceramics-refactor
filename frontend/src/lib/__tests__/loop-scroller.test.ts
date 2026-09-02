import { describe, expect, it } from 'vitest'

import { nearestPanelIndex, wrapDelta, wrapPos } from '~/lib/loop-scroller'

describe('wrapDelta (infinite-loop seam math)', () => {
  const w = 1000
  it('forward step inside the set', () => {
    expect(wrapDelta(100, 300, w)).toBe(200)
  })
  it('wraps across the seam the short way (end → start)', () => {
    // target 950 → 50 is +100 forward, not -900 backward
    expect(wrapDelta(950, 50, w)).toBe(100)
  })
  it('wraps backward across the seam (start → end)', () => {
    expect(wrapDelta(50, 950, w)).toBe(-100)
  })
  it('exactly half circumference stays positive (deterministic tie-break)', () => {
    expect(wrapDelta(0, 500, w)).toBe(500)
  })
})

describe('wrapPos', () => {
  it('normalizes into [0, w)', () => {
    expect(wrapPos(-300, 1000)).toBe(700)
    expect(wrapPos(1200, 1000)).toBe(200)
    expect(wrapPos(0.5, 1000)).toBe(0.5)
  })
})

describe('nearestPanelIndex (center-detection across both copies)', () => {
  const offsets = [0, 100, 200]
  const widths = [50, 50, 50]
  const vw = 200
  const w = 300 // one set

  it('picks the panel centered in view', () => {
    // viewport center = 100 → panel 1 (center 125) nearest
    expect(nearestPanelIndex(offsets, widths, 0, vw, w)).toBe(1)
  })

  it('just past the seam, the copy-2 first panel wins → index 0', () => {
    // x such that viewport center ≈ 325 (copy 2's panel 1)
    expect(nearestPanelIndex(offsets, widths, 250, vw, w)).toBe(0)
  })

  it('handles mixed panel widths (crate cards)', () => {
    const offs = [0, 400]
    const wids = [320, 320]
    const vwc = 780
    expect(nearestPanelIndex(offs, wids, 400 + 160 - vwc / 2, vwc, 800)).toBe(1)
  })
})
