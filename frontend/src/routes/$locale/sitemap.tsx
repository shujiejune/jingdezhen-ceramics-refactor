import { createFileRoute, Link } from '@tanstack/react-router'

import { SealMark } from '~/components/ornaments'
import { useI18n } from '~/lib/i18n'
import { buildSeoHead } from '~/lib/seo'

/**
 * Sitemap — a plain HTML cross-link page (PRD §4.4). Lists every public
 * route grouped by section so search engines (and visitors) can discover
 * the full site structure. The admin CMS is intentionally excluded.
 */
export const Route = createFileRoute('/$locale/sitemap')({
  head: ({ params }) => {
    const title = 'Sitemap — Jingdezhen Ceramics'
    const description =
      'Full site map of the Jingdezhen Ceramics Platform — gallery, artists, heritage, visit, travel.'
    const { meta, links } = buildSeoHead({
      locale: params.locale,
      path: '/sitemap',
      title,
      description,
      ogType: 'website',
    })
    return {
      meta: [{ title }, { name: 'description', content: description }, ...meta],
      links,
    }
  },
  component: SitemapPage,
})

function SitemapPage() {
  const { t, locale } = useI18n()
  const base = `/${locale}`

  const sections: { title: string; links: { label: string; to: string }[] }[] = [
    {
      title: t('footer.explore'),
      links: [
        { label: t('nav.gallery'), to: `${base}/catalog` },
        { label: t('nav.artists'), to: `${base}/artists` },
        { label: t('nav.heritage'), to: `${base}/ceramicstory` },
        { label: t('nav.visit'), to: `${base}/engage` },
        { label: t('nav.travel'), to: `${base}/itinerary` },
      ],
    },
    {
      title: t('footer.support'),
      links: [
        { label: t('nav.cart'), to: `${base}/cart` },
        { label: t('nav.wishlist'), to: `${base}/wishlist` },
        { label: t('nav.orders'), to: `${base}/orders` },
        { label: t('footer.contact'), to: `${base}/engage` },
      ],
    },
    {
      title: t('footer.legal'),
      links: [
        { label: t('footer.privacy'), to: `${base}/privacy` },
        { label: t('footer.terms'), to: `${base}/terms` },
        { label: t('footer.cookies'), to: `${base}/cookies` },
      ],
    },
  ]

  return (
    <div className="mx-auto max-w-3xl px-4 pt-16 pb-16 sm:px-6">
      <div className="flex items-center gap-3">
        <SealMark size={28} />
        <h1 className="text-display-sm text-ink-900">{t('footer.sitemap')}</h1>
      </div>
      <div className="mt-10 grid gap-10 sm:grid-cols-3">
        {sections.map((sec) => (
          <nav key={sec.title} aria-label={sec.title}>
            <h2 className="text-[0.72rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">
              {sec.title}
            </h2>
            <ul className="mt-4 flex flex-col gap-2.5">
              {sec.links.map((l) => (
                <li key={l.label}>
                  <Link
                    to={l.to as never}
                    className="text-sm text-ink-600 transition hover:text-cobalt-700"
                  >
                    {l.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
    </div>
  )
}
