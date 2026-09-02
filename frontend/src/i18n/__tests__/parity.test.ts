import { describe, expect, it } from 'vitest'

import { enUS } from '~/i18n/en-US'
import { zhCN } from '~/i18n/zh-CN'

/** UI catalogs must stay key-identical across locales (TDD §3.2). */
describe('i18n catalog parity', () => {
  const en = new Set(Object.keys(enUS))
  const zh = new Set(Object.keys(zhCN))

  it('zh-CN has every en-US key', () => {
    expect([...en].filter((k) => !zh.has(k))).toEqual([])
  })

  it('en-US has every zh-CN key', () => {
    expect([...zh].filter((k) => !en.has(k))).toEqual([])
  })

  it('no empty translations', () => {
    for (const [k, v] of Object.entries(zhCN)) {
      expect(String(v).length, `zh-CN "${k}" is empty`).toBeGreaterThan(0)
    }
  })
})
