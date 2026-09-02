import { describe, expect, it } from 'vitest'

import { seededRandom } from '~/lib/utils'

describe('seededRandom (procedural artwork determinism)', () => {
  it('same seed → identical sequence', () => {
    const a = seededRandom(101)
    const b = seededRandom(101)
    for (let i = 0; i < 8; i++) {
      expect(a()).toBe(b())
    }
  })

  it('different seeds → different sequences', () => {
    const a = Array.from({ length: 6 }, () => seededRandom(1)())
    const b = Array.from({ length: 6 }, () => seededRandom(2)())
    expect(a).not.toEqual(b)
  })

  it('emits values in [0, 1)', () => {
    const r = seededRandom(42)
    for (let i = 0; i < 200; i++) {
      const v = r()
      expect(v).toBeGreaterThanOrEqual(0)
      expect(v).toBeLessThan(1)
    }
  })
})
