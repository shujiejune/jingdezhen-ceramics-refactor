import { Link, createFileRoute } from '@tanstack/react-router'
import { Airplane, CalendarBlank, User } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Badge, ButtonLink, EmptyState, Spinner } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatDate } from '~/lib/utils'
import type { ItineraryRequest, ItineraryStatus } from '~/lib/types'

const statusTone: Record<ItineraryStatus, 'cobalt' | 'success' | 'warning' | 'neutral' | 'danger'> =
  {
    pending: 'warning',
    processing: 'cobalt',
    quoted: 'cobalt',
    deposit_paid: 'cobalt',
    confirmed: 'success',
    cancelled: 'neutral',
    closed: 'neutral',
  }

/** My journeys — submitted itinerary requests + statuses (PRD §3.3.2). */
export const Route = createFileRoute('/$locale/itineraries')({
  component: ItinerariesPage,
})

function ItinerariesPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const [list, setList] = useState<ItineraryRequest[] | null>(null)

  useEffect(() => {
    if (ready && token)
      void api
        .listItineraries(token)
        .then((res) => setList(res.data))
        .catch(() => setList([]))
    else if (ready) setList([])
  }, [ready, token])

  if (!ready || (token && list === null)) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (!token) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('itin.list.title')}</h1>
        <p className="mt-3 text-ink-500">{t('itin.signInBody')}</p>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/itineraries` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('nav.account')}</p>
      <h1 className="mt-2 text-display-sm text-ink-900">{t('itin.list.title')}</h1>
      <p className="mt-2 text-[0.92rem] text-ink-500">{t('itin.list.subtitle')}</p>

      {list && list.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={<Airplane size={40} weight="duotone" />}
            title={t('itin.list.empty')}
            body={t('itin.list.emptyBody')}
            action={<ButtonLink to={`/${locale}/itinerary`}>{t('landing.travelCta')}</ButtonLink>}
          />
        </div>
      ) : (
        <div className="mt-8 flex flex-col gap-4">
          {list?.map((r) => (
            <div
              key={r.id}
              className="card-surface flex flex-wrap items-center justify-between gap-4 p-5"
            >
              <div>
                <div className="flex items-center gap-3">
                  <span className="text-[0.95rem] font-semibold text-ink-900">
                    {t('itin.requestN', { id: r.id })}
                  </span>
                  <Badge tone={statusTone[r.status]}>{t(`itin.status.${r.status}`)}</Badge>
                </div>
                <div className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-[0.82rem] text-ink-400">
                  <span className="flex items-center gap-1.5">
                    <CalendarBlank size={13} />
                    {formatDate(r.arrival_date, locale)} · {r.duration_days} {t('itin.days')}
                  </span>
                  <span className="flex items-center gap-1.5">
                    <User size={13} />
                    {r.adults}
                    {r.children > 0 ? ` + ${r.children}` : ''}
                  </span>
                  <span className="flex items-center gap-1.5">{t(`itin.pace.${r.pace}`)}</span>
                </div>
              </div>
              {r.status === 'quoted' ? (
                <span className="inline-flex h-9 items-center rounded-lg bg-cobalt-600 px-4 text-[0.82rem] font-medium text-white">
                  {t('itin.viewQuote')}
                </span>
              ) : (
                <Link
                  to="/$locale/itinerary"
                  params={{ locale }}
                  className="text-[0.82rem] font-medium text-cobalt-600 hover:underline"
                >
                  {t('landing.travelCta')}
                </Link>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
