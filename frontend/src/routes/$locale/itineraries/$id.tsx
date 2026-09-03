import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, CheckCircle, ClockCountdown, XCircle } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Badge, Breadcrumbs, Button, EmptyState, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatMinor } from '~/lib/money'
import { cn, formatDate } from '~/lib/utils'
import type { ItineraryRequest, ItineraryStatus } from '~/lib/types'
import type { CatalogKey } from '~/i18n/en-US'

const statusTone: Record<ItineraryStatus, 'cobalt' | 'success' | 'warning' | 'neutral' | 'danger'> =
  {
    pending: 'warning',
    processing: 'cobalt',
    quoted: 'cobalt',
    deposit_paid: 'success',
    confirmed: 'success',
    cancelled: 'neutral',
    closed: 'neutral',
  }

/** Itinerary detail — request summary + quote breakdown + pay-deposit flow (PRD §3.3.2). */
export const Route = createFileRoute('/$locale/itineraries/$id')({
  component: ItineraryDetailPage,
})

function ItineraryDetailPage() {
  const { t, locale } = useI18n()
  const { id } = Route.useParams()
  const { ready, token } = useAuth()
  const { push } = useToast()
  const navigate = useNavigate()
  const [itin, setItin] = useState<ItineraryRequest | null | undefined>(undefined)
  const [paying, setPaying] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [showCancel, setShowCancel] = useState(false)

  useEffect(() => {
    if (ready && token) {
      void api
        .getItinerary(token, Number(id))
        .then((res) => setItin(res))
        .catch(() => setItin(null))
    } else if (ready) {
      setItin(null)
    }
  }, [ready, token, id])

  if (!ready || itin === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (itin === null) {
    return (
      <div className="mx-auto max-w-shell px-4 pt-16 sm:px-6">
        <EmptyState
          title={t('errors.not_found')}
          action={
            <Button onClick={() => void navigate({ to: `/${locale}/itineraries` })}>
              {t('itin.list.title')}
            </Button>
          }
        />
      </div>
    )
  }

  const quote = itin.quote
  const isPaid = itin.status === 'deposit_paid' || itin.status === 'confirmed'
  const canCancel = itin.status === 'pending' || itin.status === 'quoted'

  const payDeposit = async () => {
    if (!token) return
    setPaying(true)
    try {
      const res = await api.payItineraryDeposit(token, itin.id, 'mock')
      if (res.hosted_url && /^https?:\/\//.test(res.hosted_url)) {
        window.location.href = res.hosted_url
        return
      }
      push({ title: t('itin.payDepositOk'), kind: 'success' })
      void navigate({ to: '/$locale/itineraries', params: { locale } })
    } catch (e) {
      push({ title: t(errorKey(e) as Parameters<typeof t>[0]), kind: 'error' })
    } finally {
      setPaying(false)
    }
  }

  const cancel = async () => {
    if (!token) return
    setCancelling(true)
    try {
      await api.cancelItinerary(token, itin.id)
      push({ title: t('itin.cancelOk'), kind: 'success' })
      void navigate({ to: '/$locale/itineraries', params: { locale } })
    } catch (e) {
      push({ title: t(errorKey(e) as Parameters<typeof t>[0]), kind: 'error' })
    } finally {
      setCancelling(false)
    }
  }

  const interests = itin.interests.length ? itin.interests.join(' · ') : t('common.optional')

  return (
    <div className="mx-auto max-w-3xl px-4 pt-8 sm:px-6">
      <Breadcrumbs
        items={[
          { label: t('itin.list.title'), to: '/$locale/itineraries', params: { locale } },
          { label: t('itin.requestN', { id: itin.id }) },
        ]}
      />

      <Link
        to="/$locale/itineraries"
        params={{ locale }}
        className="mb-6 inline-flex items-center gap-1.5 text-[0.84rem] font-medium text-cobalt-600 hover:underline"
      >
        <ArrowLeft size={14} />
        {t('itin.list.title')}
      </Link>

      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-display-sm text-ink-900">{t('itin.requestN', { id: itin.id })}</h1>
        <Badge tone={statusTone[itin.status]}>
          {t(`itin.status.${itin.status}` as CatalogKey)}
        </Badge>
      </div>

      {/* ------------------------- trip details ------------------------- */}
      <div className="card-surface mt-6 p-6">
        <h2 className="text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">
          {t('itin.tripDetails')}
        </h2>
        <dl className="mt-4 grid gap-x-8 gap-y-3 sm:grid-cols-2">
          <DetailRow label={t('itin.arrivalDate')} value={formatDate(itin.arrival_date, locale)} />
          <DetailRow label={t('itin.duration')} value={`${itin.duration_days} ${t('itin.days')}`} />
          <DetailRow
            label={t('itin.travelers')}
            value={`${itin.adults}${itin.children > 0 ? ` + ${itin.children}` : ''}`}
          />
          <DetailRow label={t('itin.pace')} value={t(`itin.pace.${itin.pace}` as CatalogKey)} />
          <DetailRow label={t('itin.interests')} value={interests} />
          <DetailRow
            label={t('itin.flexible')}
            value={itin.flexible ? t('common.yes') : t('common.no')}
          />
        </dl>

        {/* services */}
        <div className="mt-6 flex flex-wrap gap-2 border-t border-cobalt-50 pt-5">
          <ServiceChip active={itin.services.guide !== 'none'}>
            {t('itin.guide')}: {t(`itin.guide.${itin.services.guide}` as CatalogKey)}
          </ServiceChip>
          <ServiceChip active={itin.services.hotel}>
            {t('itin.hotel')}
            {itin.services.hotel_level
              ? ` · ${t(`itin.hotelLevel.${itin.services.hotel_level}` as CatalogKey)}`
              : ''}
          </ServiceChip>
          <ServiceChip active={itin.services.pickup}>{t('itin.pickup')}</ServiceChip>
          <ServiceChip active={itin.services.experience}>{t('itin.experience')}</ServiceChip>
        </div>

        {/* SLA / submitted */}
        <div className="mt-6 flex flex-wrap items-center gap-3 border-t border-cobalt-50 pt-5 text-[0.8rem] text-ink-400">
          <span className="flex items-center gap-1.5">
            <ClockCountdown size={13} />
            {t('itin.slaBadge')} · {formatDate(itin.sla_deadline, locale)}
          </span>
          <span>·</span>
          <span>{t('itin.submittedOn', { date: formatDate(itin.submitted_at, locale) })}</span>
        </div>
      </div>

      {/* ------------------------- quote ------------------------- */}
      {quote ? (
        <div className="card-surface mt-6 p-6">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-[1.2rem] font-semibold text-ink-900">{t('itin.quoteTitle')}</h2>
              <p className="mt-1 text-[0.88rem] text-ink-500">{t('itin.quoteSubtitle')}</p>
            </div>
            <Badge tone="gold">{quote.currency}</Badge>
          </div>

          {/* line items */}
          <ul className="mt-6 flex flex-col divide-y divide-cobalt-50">
            {quote.line_items.map((item, i) => (
              <li key={i} className="flex items-start justify-between gap-4 py-3.5">
                <div className="min-w-0">
                  <p className="text-[0.9rem] font-medium text-ink-800">{item.label}</p>
                  {item.detail && (
                    <p className="mt-0.5 text-[0.78rem] text-ink-400">{item.detail}</p>
                  )}
                </div>
                <p className="shrink-0 text-[0.9rem] font-semibold text-ink-900">
                  {formatMinor(item.amount ?? item.amount_minor, quote.currency, locale)}
                </p>
              </li>
            ))}
          </ul>

          {/* totals */}
          <dl className="mt-4 flex flex-col gap-2 border-t border-cobalt-100 pt-4 text-[0.9rem]">
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('itin.deposit')}</dt>
              <dd className="font-medium text-cobalt-700">
                {formatMinor(quote.deposit_minor, quote.currency, locale)}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('itin.balanceOnArrival')}</dt>
              <dd className="font-medium text-ink-700">
                {formatMinor(
                  Math.max(0, quote.total_minor - quote.deposit_minor),
                  quote.currency,
                  locale,
                )}
              </dd>
            </div>
            <div className="flex justify-between border-t border-cobalt-100 pt-2.5 text-[1.05rem]">
              <dt className="font-semibold text-ink-800">{t('orders.total')}</dt>
              <dd className="font-semibold text-ink-900">
                {formatMinor(quote.total_minor, quote.currency, locale)}
              </dd>
            </div>
          </dl>

          {quote.fx_rate_used != null && (
            <p className="mt-3 text-[0.74rem] text-ink-300">
              {t('checkout.fxSnapshot', { rate: quote.fx_rate_used.toFixed(3) })}
            </p>
          )}

          {/* pay / confirmed */}
          <div className="mt-6 border-t border-cobalt-50 pt-5">
            {isPaid ? (
              <div className="flex items-center gap-3 rounded-xl border border-[color:var(--color-success)]/25 bg-[color:var(--color-success-bg)] px-5 py-4">
                <CheckCircle
                  size={22}
                  weight="duotone"
                  className="text-[color:var(--color-success)]"
                />
                <p className="text-[0.92rem] font-medium text-[color:var(--color-success)]">
                  {t('itin.payDepositOk')}
                </p>
              </div>
            ) : itin.status === 'quoted' ? (
              <Button size="lg" loading={paying} onClick={() => void payDeposit()}>
                {paying ? t('itin.payDepositRedirecting') : t('itin.payDeposit')}
              </Button>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="card-surface mt-6 p-6 text-center">
          <p className="text-[0.92rem] text-ink-500">{t('itin.quoteNotFound')}</p>
        </div>
      )}

      {/* ------------------------- cancel ------------------------- */}
      {canCancel && (
        <div className="mt-6">
          {!showCancel ? (
            <Button variant="danger" size="sm" onClick={() => setShowCancel(true)}>
              <XCircle size={15} />
              {t('itin.cancelTitle')}
            </Button>
          ) : (
            <div className="rounded-xl border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-bg)]/50 p-5">
              <p className="text-[0.9rem] font-medium text-ink-700">{t('itin.cancelTitle')}</p>
              <p className="mt-1 text-[0.84rem] leading-relaxed text-ink-500">
                {t('itin.cancelConfirm')}
              </p>
              <div className="mt-4 flex gap-3">
                <Button variant="ghost" size="sm" onClick={() => setShowCancel(false)}>
                  {t('common.back')}
                </Button>
                <Button
                  variant="danger"
                  size="sm"
                  loading={cancelling}
                  onClick={() => void cancel()}
                >
                  {t('common.confirm')}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

/* --------------------------- small parts --------------------------- */

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <dt className="text-[0.8rem] text-ink-400">{label}</dt>
      <dd className="text-[0.9rem] font-medium text-ink-800 text-right">{value}</dd>
    </div>
  )
}

function ServiceChip({ active, children }: { active: boolean; children: React.ReactNode }) {
  return (
    <span
      className={cn(
        'rounded-full border px-3.5 py-1.5 text-[0.8rem] font-medium transition',
        active
          ? 'border-cobalt-200 bg-cobalt-50/60 text-cobalt-700'
          : 'border-ink-300/30 text-ink-400',
      )}
    >
      {children}
    </span>
  )
}
