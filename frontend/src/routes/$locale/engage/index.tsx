import { createFileRoute } from '@tanstack/react-router'

import { ActivityCard } from '~/components/cards'
import { SectionHeading } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

/** Destinations & Local Lifestyle index (SSR). */
export const Route = createFileRoute('/$locale/engage/')({
  loader: async ({ context, params }) =>
    context.queryClient.ensureQueryData({
      queryKey: ['activities', params.locale, 'all'],
      queryFn: () => api.getActivities(params.locale),
    }),
  component: EngagePage,
})

function EngagePage() {
  const { t } = useI18n()
  const activities = Route.useLoaderData()
  const destinations = activities.filter((a) => a.type === 'destination')
  const lifestyle = activities.filter((a) => a.type === 'lifestyle')

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <SectionHeading
        eyebrow={t('landing.visitEyebrow')}
        title={t('nav.visit')}
        sub={t('engage.subtitle')}
      />

      <h2 className="eyebrow mb-5">{t('engage.destinations')}</h2>
      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {destinations.map((a) => (
          <ActivityCard key={a.id} activity={a} />
        ))}
      </div>

      <div className="mt-16">
        <h2 className="eyebrow mb-5">{t('engage.lifestyle')}</h2>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {lifestyle.map((a) => (
            <ActivityCard key={a.id} activity={a} />
          ))}
        </div>
      </div>
    </div>
  )
}
