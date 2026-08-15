import { createFileRoute, notFound } from '@tanstack/react-router'
import { Clock, MapPin } from '@phosphor-icons/react'

import { PorcelainLandscape } from '~/components/artwork/PorcelainFigure'
import { ContentBlocks } from '~/components/content/ContentBlocks'
import { Badge } from '~/components/common/ui'
import { Breadcrumbs } from '~/components/common/ui'
import { JsonLd } from '~/components/seo/JsonLd'
import { api, ApiError } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

export const Route = createFileRoute('/$locale/engage/$slug')({
  loader: async ({ params }) => {
    try {
      return await api.getActivity(params.slug, params.locale)
    } catch (e) {
      if (e instanceof ApiError && e.is('not_found')) throw notFound()
      throw e
    }
  },
  head: ({ loaderData }) => ({
    meta: loaderData ? [{ title: loaderData.title }, { name: 'description', content: loaderData.summary }] : [],
  }),
  notFoundComponent: () => {
    const { t } = useI18n()
    return (
      <div className="mx-auto max-w-shell px-6 py-32 text-center">
        <h1 className="text-display-sm text-ink-900">{t('errors.not_found')}</h1>
      </div>
    )
  },
  component: ActivityPage,
})

/** Styled stand-in for the OSM map embed (no external tiles in prototype). */
function MapMotif({ lat, lng, label }: { lat?: number; lng?: number; label: string }) {
  return (
    <div className="relative overflow-hidden rounded-xl border border-cobalt-100 bg-porcelain" aria-label={label}>
      <svg viewBox="0 0 600 200" className="block h-44 w-full" aria-hidden="true">
        <g stroke="var(--cobalt-300)" strokeOpacity="0.5" strokeWidth="1">
          {Array.from({ length: 7 }, (_, i) => (
            <line key={`h${i}`} x1="0" y1={i * 30 + 10} x2="600" y2={i * 30 + 10} strokeOpacity={0.25 + (i % 2) * 0.2} />
          ))}
          {Array.from({ length: 15 }, (_, i) => (
            <line key={`v${i}`} x1={i * 43 + 10} y1="0" x2={i * 43 + 10} y2="200" strokeOpacity={0.2 + (i % 2) * 0.15} />
          ))}
        </g>
        <path d="M0 150 Q120 120 240 145 T480 130 T600 150 L600 200 L0 200 Z" fill="var(--cobalt-200)" fillOpacity="0.4" />
        <path d="M0 165 Q150 140 300 160 T600 158" fill="none" stroke="var(--cobalt-500)" strokeOpacity="0.4" strokeWidth="1.5" />
        <g transform="translate(300 78)">
          <circle r="26" fill="var(--cobalt-600)" fillOpacity="0.12" />
          <circle r="9" fill="var(--cobalt-600)" />
          <circle r="15" fill="none" stroke="var(--cobalt-600)" strokeOpacity="0.5" strokeWidth="1.5" />
        </g>
      </svg>
      <span className="absolute bottom-2.5 left-3 font-mono text-[0.7rem] text-ink-400">
        {lat?.toFixed(4)}, {lng?.toFixed(4)}
      </span>
      <Badge tone="neutral" className="absolute top-2.5 right-2.5 bg-white/90">
        OSM
      </Badge>
    </div>
  )
}

function ActivityPage() {
  const { t, locale } = useI18n()
  const activity = Route.useLoaderData()

  return (
    <article className="mx-auto max-w-shell px-4 sm:px-6">
      <Breadcrumbs items={[{ label: t('nav.visit'), to: `/${locale}/engage` }, { label: activity.title }]} />

      <div className="relative mt-2 overflow-hidden rounded-2xl border border-cobalt-100">
        <PorcelainLandscape seed={activity.figure_seed} className="h-64 w-full object-cover sm:h-80" />
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-ink-900/55 to-transparent px-6 py-6 sm:px-10">
          <Badge tone="cobalt" className="mb-2.5 bg-white/90">
            {activity.type === 'destination' ? t('engage.destinations') : t('engage.lifestyle')}
          </Badge>
          <h1 className="text-[1.7rem] font-semibold tracking-tight text-white">{activity.title}</h1>
          <p className="mt-1.5 max-w-2xl text-[0.92rem] text-white/80">{activity.summary}</p>
        </div>
      </div>

      <div className="mx-auto mt-10 grid max-w-[52rem] gap-10 pb-8 lg:grid-cols-[1fr_17rem]">
        <ContentBlocks blocks={activity.content} />

        <aside className="flex flex-col gap-4">
          {activity.address && (
            <div className="card-surface p-4">
              <h2 className="flex items-center gap-2 text-[0.8rem] font-semibold tracking-wide text-ink-600 uppercase">
                <MapPin size={14} className="text-cobalt-500" weight="duotone" />
                {t('engage.address')}
              </h2>
              <p className="mt-2 text-[0.86rem] leading-relaxed text-ink-600">{activity.address}</p>
            </div>
          )}
          {activity.opening_info && (
            <div className="card-surface p-4">
              <h2 className="flex items-center gap-2 text-[0.8rem] font-semibold tracking-wide text-ink-600 uppercase">
                <Clock size={14} className="text-cobalt-500" weight="duotone" />
                {t('engage.openingInfo')}
              </h2>
              <p className="mt-2 text-[0.86rem] leading-relaxed text-ink-600">{activity.opening_info}</p>
            </div>
          )}
          {activity.lat !== undefined && activity.lng !== undefined && (
            <MapMotif lat={activity.lat} lng={activity.lng} label={t('engage.map')} />
          )}
        </aside>
      </div>

      <JsonLd
        data={{
          '@context': 'https://schema.org',
          '@type': 'Article',
          headline: activity.title,
          description: activity.summary,
          inLanguage: locale,
        }}
      />
    </article>
  )
}
