import { createFileRoute, notFound } from '@tanstack/react-router'

import { ArtistMedallion, PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { ProductCard } from '~/components/cards'
import { BrushRule, CornerFrame, PetalScatter } from '~/components/ornaments'
import { Breadcrumbs, EmptyState, SectionHeading } from '~/components/common/ui'
import { JsonLd } from '~/components/seo/JsonLd'
import { api, ApiError } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

export const Route = createFileRoute('/$locale/artists/$slug')({
  loader: async ({ params }) => {
    try {
      const artist = await api.getArtist(params.slug, params.locale)
      const works = await api.getProducts({ locale: params.locale, artist: params.slug, limit: 12 })
      return { artist, works: works.data }
    } catch (e) {
      if (e instanceof ApiError && e.is('not_found')) throw notFound()
      throw e
    }
  },
  head: ({ loaderData }) => ({
    meta: loaderData
      ? [
          { title: loaderData.artist.name },
          { name: 'description', content: loaderData.artist.bio?.slice(0, 155) },
        ]
      : [],
  }),
  notFoundComponent: () => {
    const { t } = useI18n()
    return (
      <div className="mx-auto max-w-shell px-6 py-32 text-center">
        <h1 className="text-display-sm text-ink-900">{t('errors.not_found')}</h1>
      </div>
    )
  },
  component: ArtistDetailPage,
})

function ArtistDetailPage() {
  const { t, locale } = useI18n()
  const { artist, works } = Route.useLoaderData()
  const gallery = artist.gallery ?? []

  return (
    <div className="mx-auto max-w-shell px-4 pt-8 sm:px-6">
      <Breadcrumbs
        items={[{ label: t('nav.artists'), to: `/${locale}/artists` }, { label: artist.name }]}
      />

      <div className="relative overflow-hidden rounded-2xl border border-cobalt-100 bg-gradient-to-b from-porcelain/70 to-wash">
        <PetalScatter seed={artist.id * 7} className="pointer-events-none absolute top-4 right-8 opacity-60" />
        <div className="relative grid gap-10 px-6 py-12 sm:px-10 lg:grid-cols-[auto_1fr_auto] lg:items-center">
          <ArtistMedallion glyph={artist.glyph} seed={artist.id} size={104} className="mx-auto" />
          <div className="max-w-2xl text-center lg:text-left">
            <p className="eyebrow">{t('landing.artistsEyebrow')}</p>
            <h1 className="mt-2.5 text-display-sm text-ink-900">{artist.name}</h1>
            <BrushRule className="mx-auto mt-4 lg:mx-0" />
            <p className="mt-5 leading-relaxed text-ink-500">{artist.bio}</p>
          </div>
          {gallery.length > 0 && (
            <div className="hidden w-36 flex-col gap-3 lg:flex">
              {gallery.slice(0, 2).map((g) => (
                <div
                  key={g.media_id}
                  className="relative overflow-hidden rounded-lg border border-cobalt-100 bg-white"
                >
                  <PorcelainFigure kind={g.figure_kind} seed={g.figure_seed} className="h-auto w-full" />
                  <CornerFrame inset={7} />
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <section className="mt-16">
        <SectionHeading title={t('artist.worksTitle')} />
        {works.length === 0 ? (
          <EmptyState title={t('artist.emptyWorks')} />
        ) : (
          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {works.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        )}
      </section>

      <JsonLd
        data={{
          '@context': 'https://schema.org',
          '@type': 'Person',
          name: artist.name,
          description: artist.bio,
          jobTitle: 'Ceramic artist',
        }}
      />
    </div>
  )
}
