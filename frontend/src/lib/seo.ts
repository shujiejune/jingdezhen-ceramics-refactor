/**
 * SEO helpers — build canonical, hreflang, Open Graph, and Twitter
 * card meta for route `head()` functions (PRD §4.4, TDD §6).
 *
 * The site origin is read from `VITE_SITE_ORIGIN` (or falls back to the
 * current `window.location.origin` in the browser / a placeholder in SSR).
 */
import { SUPPORTED_LOCALES } from '~/lib/utils'

export const SITE_NAME = 'Jingdezhen Ceramics Platform'

function siteOrigin(): string {
  const env = import.meta.env.VITE_SITE_ORIGIN as string | undefined
  if (env) return env.replace(/\/$/, '')
  if (typeof window !== 'undefined') return window.location.origin
  return 'https://jingdezhen-ceramics.com'
}

/** Build an absolute canonical URL for a locale + path. */
export function canonicalUrl(locale: string, path: string): string {
  const p = path.startsWith('/') ? path : `/${path}`
  return `${siteOrigin()}/${locale}${p === '/' ? '' : p}`
}

export interface SeoMetaInput {
  locale: string
  /** path relative to the locale, e.g. '/catalog/blue-and-white-vase' */
  path: string
  title: string
  description: string
  /** og:type — 'website' for landing/index, 'product' for products, 'article' for stories */
  ogType?: string
  /** absolute image URL for OG/Twitter; undefined = no image tag */
  image?: string
  /** locale → slug map for the OTHER translations (current locale excluded) */
  alternates?: Record<string, string>
  /** the route pattern that produces the alternates, e.g. '/catalog/$slug' — used to build the alternate URL */
  alternateRoute?: 'catalog' | 'artists' | 'ceramicstory' | 'engage'
}

/** TanStack Router meta entry (minimal shape we use). */
interface MetaEntry {
  charSet?: string
  title?: string
  name?: string
  property?: string
  content?: string
}

/** TanStack Router links entry. */
interface LinkEntry {
  rel: string
  href: string
  hrefLang?: string
}

export interface SeoHead {
  meta: MetaEntry[]
  links: LinkEntry[]
}

/**
 * Build the full set of SEO meta + links for a route.
 * Includes: title, description, canonical, hreflang alternates,
 * OG tags, Twitter card summary_large_image.
 */
export function buildSeoHead(input: SeoMetaInput): SeoHead {
  const {
    locale,
    path,
    title,
    description,
    ogType = 'website',
    image,
    alternates,
    alternateRoute,
  } = input
  const canonical = canonicalUrl(locale, path)

  const meta: MetaEntry[] = [
    { property: 'og:title', content: title },
    { property: 'og:description', content: description },
    { property: 'og:type', content: ogType },
    { property: 'og:url', content: canonical },
    { property: 'og:site_name', content: SITE_NAME },
    { property: 'og:locale', content: locale.replace('-', '_') },
    { name: 'twitter:card', content: image ? 'summary_large_image' : 'summary' },
    { name: 'twitter:title', content: title },
    { name: 'twitter:description', content: description },
  ]

  if (image) {
    meta.push({ property: 'og:image', content: image })
    meta.push({ name: 'twitter:image', content: image })
  }

  // hreflang alternates: current locale + every other locale in `alternates`
  const links: LinkEntry[] = [{ rel: 'canonical', href: canonical }]

  // self-referencing hreflang for the current locale
  links.push({ rel: 'alternate', hrefLang: locale, href: canonical })

  if (alternates && alternateRoute) {
    for (const [altLocale, altSlug] of Object.entries(alternates)) {
      const altPath = `/${alternateRoute}/${altSlug}`
      links.push({ rel: 'alternate', hrefLang: altLocale, href: canonicalUrl(altLocale, altPath) })
    }
    // x-default points at en-US if available
    if (alternates['en-US']) {
      links.push({
        rel: 'alternate',
        hrefLang: 'x-default',
        href: canonicalUrl('en-US', `/${alternateRoute}/${alternates['en-US']}`),
      })
    }
  }

  // og:locale:alternate for each other supported locale
  for (const loc of SUPPORTED_LOCALES) {
    if (loc !== locale) {
      meta.push({ property: 'og:locale:alternate', content: loc.replace('-', '_') })
    }
  }

  return { meta, links }
}

/**
 * Head for non-public routes (auth, account, cart, checkout, orders,
 * wishlist, itinerary, notifications, admin). Emits `noindex, nofollow`
 * so these pages never appear in search results (PRD §4.4).
 */
export function noindexHead(title: string): { meta: MetaEntry[]; links: LinkEntry[] } {
  return {
    meta: [{ name: 'robots', content: 'noindex, nofollow' }, { title }],
    links: [],
  }
}
