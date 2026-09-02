import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import tailwindConfig from '../../../tailwind.config'

/**
 * M-FD guard: tokens.css (CSS variables for SVG/gradients) and
 * tailwind.config.ts (hex literals so alpha modifiers work) are intentional
 * twins. This test keeps them in sync — edit both or neither.
 */

const FAMILIES = ['cobalt', 'ink', 'rose', 'celadon', 'cinnabar', 'imperial'] as const
const SINGLES = ['paper', 'mist', 'porcelain', 'wash'] as const

type Scale = Record<string, string>

function cssVars(): Scale {
  const css = readFileSync(resolve(process.cwd(), 'src/styles/tokens.css'), 'utf8')
  const vars: Scale = {}
  for (const m of css.matchAll(/^\s*--([a-z][a-z0-9-]*):\s*(#[0-9a-fA-F]{6})\s*;/gm)) {
    vars[m[1]] = m[2].toLowerCase()
  }
  return vars
}

function configColors(): Scale {
  const colors = (tailwindConfig.theme?.extend?.colors ?? {}) as Record<
    string,
    string | Record<string, string>
  >
  const flat: Scale = {}
  for (const [key, value] of Object.entries(colors)) {
    if (typeof value === 'string') {
      flat[key] = value.toLowerCase()
    } else {
      for (const [shade, hex] of Object.entries(value)) {
        flat[`${key}-${shade}`] = String(hex).toLowerCase()
      }
    }
  }
  return flat
}

describe('tokens.css ↔ tailwind.config.ts sync', () => {
  const vars = cssVars()
  const cfg = configColors()

  it.each(FAMILIES)('%s scale matches', (family) => {
    const shades = Object.keys(vars).filter((k) => k.startsWith(`${family}-`))
    expect(shades.length, `${family} missing from tokens.css`).toBeGreaterThan(0)
    for (const shade of shades) {
      expect(cfg[shade], `tailwind.config.ts missing ${shade}`).toBe(vars[shade])
    }
  })

  it.each(SINGLES)('%s matches', (single) => {
    expect(vars[single], `tokens.css missing --${single}`).toBeDefined()
    expect(cfg[single]).toBe(vars[single])
  })

  it('gold stays a config-only alias of imperial', () => {
    expect(cfg['gold-400']).toBe(cfg['imperial-400'])
    expect(cfg['gold-500']).toBe(cfg['imperial-500'])
    expect(cfg['gold-600']).toBe(cfg['imperial-600'])
  })

  it('every family shade in the config exists in tokens.css', () => {
    for (const [key, hex] of Object.entries(cfg)) {
      if (FAMILIES.some((f) => key.startsWith(`${f}-`)) || SINGLES.includes(key as never)) {
        expect(vars[key], `tokens.css missing --${key}`).toBe(hex)
      }
    }
  })
})
