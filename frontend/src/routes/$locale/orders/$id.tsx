import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { CheckCircle, Truck } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import { z } from 'zod'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { Badge, Breadcrumbs, Button, EmptyState, Spinner } from '~/components/common/ui'
import { OrderStatusBadge } from './index'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useToast } from '~/components/common/Toaster'
import { useI18n } from '~/lib/i18n'
import { noindexHead } from '~/lib/seo'
import { formatDateTime } from '~/lib/utils'
import type { Order, OrderStatus } from '~/lib/types'
import type { CatalogKey } from '~/i18n/en-US'

const searchSchema = z.object({ placed: z.number().optional() })

/** Order detail — status timeline, tracking, cancel-unpaid, refund note. */
export const Route = createFileRoute('/$locale/orders/$id')({
  validateSearch: searchSchema,
  head: () => noindexHead('Order detail — Jingdezhen Ceramics'),
  component: OrderDetailPage,
})

const FLOW: Array<{ status: OrderStatus; label: CatalogKey }> = [
  { status: 'created', label: 'orders.timeline.created' },
  { status: 'paid', label: 'orders.timeline.paid' },
  { status: 'shipped', label: 'orders.timeline.shipped' },
  { status: 'completed', label: 'orders.timeline.completed' },
]

function flowIndex(status: OrderStatus): number {
  if (status === 'completed') return 3
  if (status === 'shipped') return 2
  if (status === 'paid') return 1
  return 0
}

function OrderDetailPage() {
  const { t, locale, price } = useI18n()
  const { id } = Route.useParams()
  const search = useSearch({ from: '/$locale/orders/$id' })
  const { ready, token } = useAuth()
  const { push } = useToast()
  const navigate = useNavigate()
  const [order, setOrder] = useState<Order | null | undefined>(undefined)
  const [cancelling, setCancelling] = useState(false)
  const [showCancel, setShowCancel] = useState(false)
  const [cancelReason, setCancelReason] = useState('')

  useEffect(() => {
    if (ready && token) {
      void api
        .getOrder(token, Number(id), locale)
        .then(setOrder)
        .catch(() => setOrder(null))
    } else if (ready) {
      setOrder(null)
    }
  }, [ready, token, id, locale])

  if (!ready || order === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (order === null) {
    return (
      <div className="mx-auto max-w-shell px-4 pt-16 sm:px-6">
        <EmptyState
          title={t('errors.not_found')}
          action={
            <Button onClick={() => void navigate({ to: `/${locale}/orders` })}>
              {t('orders.title')}
            </Button>
          }
        />
      </div>
    )
  }

  const step = flowIndex(order.status)
  const isBad = order.status === 'cancelled' || order.status === 'refunded'

  const cancel = async () => {
    if (!token) return
    setCancelling(true)
    try {
      const updated = await api.cancelOrder(token, order.id, cancelReason || undefined)
      setOrder(updated)
      setShowCancel(false)
      push({ title: t('toast.orderCancelled') })
    } finally {
      setCancelling(false)
    }
  }

  return (
    <div className="mx-auto max-w-4xl px-4 pt-8 sm:px-6">
      <Breadcrumbs
        items={[
          { label: t('orders.title'), to: `/${locale}/orders` },
          { label: t('orders.orderN', { id: order.id }) },
        ]}
      />

      {search.placed === 1 && (
        <div className="mb-8 flex items-center gap-3 rounded-xl border border-[color:var(--color-success)]/25 bg-[color:var(--color-success-bg)] px-5 py-4">
          <CheckCircle size={22} weight="duotone" className="text-[color:var(--color-success)]" />
          <p className="text-[0.92rem] font-medium text-[color:var(--color-success)]">
            {t('orders.placedBanner')}
          </p>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <h1 className="text-[1.5rem] font-semibold tracking-tight text-ink-900">
            {t('orders.orderN', { id: order.id })}
          </h1>
          <OrderStatusBadge status={order.status} />
        </div>
        <p className="text-[0.84rem] text-ink-400">
          {t('orders.placedOn', { date: formatDateTime(order.placed_at, locale) })}
        </p>
      </div>

      {/* ------------------------- timeline ------------------------- */}
      <div className="card-surface mt-6 p-6">
        {isBad ? (
          <div className="flex items-center gap-3">
            <Badge tone={order.status === 'refunded' ? 'danger' : 'neutral'}>
              {t(`orders.timeline.${order.status}` as 'orders.timeline.created')}
              {order.cancelled_at ? ` · ${formatDateTime(order.cancelled_at, locale)}` : ''}
              {order.refunded_at ? ` · ${formatDateTime(order.refunded_at, locale)}` : ''}
            </Badge>
            {order.cancel_reason && (
              <span className="text-[0.84rem] text-ink-400">“{order.cancel_reason}”</span>
            )}
          </div>
        ) : (
          <ol className="grid gap-4 sm:grid-cols-4">
            {FLOW.map((f, i) => (
              <li key={f.status} className="flex items-start gap-3">
                <span
                  className={
                    'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-[0.7rem] font-bold ' +
                    (i <= step
                      ? 'border-cobalt-600 bg-cobalt-600 text-white'
                      : 'border-ink-300 bg-white text-ink-300')
                  }
                >
                  {i + 1}
                </span>
                <div>
                  <p
                    className={
                      'text-[0.85rem] font-medium ' + (i <= step ? 'text-ink-800' : 'text-ink-300')
                    }
                  >
                    {t(f.label)}
                  </p>
                  <p className="mt-0.5 text-[0.72rem] text-ink-300">
                    {f.status === 'created' && formatDateTime(order.placed_at, locale)}
                    {f.status === 'paid' && order.paid_at && formatDateTime(order.paid_at, locale)}
                    {f.status === 'shipped' &&
                      order.shipped_at &&
                      formatDateTime(order.shipped_at, locale)}
                    {f.status === 'completed' &&
                      order.completed_at &&
                      formatDateTime(order.completed_at, locale)}
                  </p>
                </div>
              </li>
            ))}
          </ol>
        )}

        {order.status === 'shipped' && order.carrier_name && (
          <div className="mt-6 flex flex-wrap items-center gap-3 rounded-lg bg-mist px-4 py-3.5">
            <Truck size={20} className="text-cobalt-500" weight="duotone" />
            <div>
              <p className="text-[0.84rem] text-ink-500">
                {t('orders.trackingNote', { carrier: order.carrier_name })}
              </p>
              <p className="mt-0.5 font-mono text-[0.9rem] font-semibold text-ink-800">
                {order.tracking_number}
              </p>
            </div>
          </div>
        )}
      </div>

      {/* ------------------------- items ------------------------- */}
      <div className="card-surface mt-6 p-6">
        <h2 className="text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">
          {t('orders.items')}
        </h2>
        <ul className="mt-4 flex flex-col divide-y divide-cobalt-50">
          {order.items?.map((i) => (
            <li key={i.id} className="flex items-center gap-4 py-3.5">
              <div className="h-16 w-16 shrink-0 overflow-hidden rounded-lg border border-cobalt-100 bg-wash">
                <PorcelainFigure
                  kind={i.figure_kind ?? 'vase'}
                  seed={i.figure_seed ?? 1}
                  className="h-full w-full"
                />
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[0.9rem] font-medium text-ink-800">
                  {i.title_snapshot as string}
                </p>
                <p className="mt-0.5 text-[0.78rem] text-ink-400">× {i.qty}</p>
              </div>
              <p className="text-[0.9rem] font-semibold text-ink-900">
                {price(i.unit_price_minor * i.qty, order.currency)}
              </p>
            </li>
          ))}
        </ul>

        <dl className="mt-4 flex flex-col gap-2 border-t border-cobalt-100 pt-4 text-[0.9rem]">
          <div className="flex justify-between">
            <dt className="text-ink-500">{t('cart.subtotal')}</dt>
            <dd className="font-medium text-ink-800">
              {price(order.subtotal_minor, order.currency)}
            </dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-500">{t('cart.estimatedShipping')}</dt>
            <dd className="font-medium text-ink-800">
              {price(order.shipping_minor, order.currency)}
            </dd>
          </div>
          <div className="flex justify-between border-t border-cobalt-100 pt-2.5 text-[1.05rem]">
            <dt className="font-semibold text-ink-800">{t('orders.total')}</dt>
            <dd className="font-semibold text-ink-900">
              {price(order.total_minor, order.currency)}
            </dd>
          </div>
        </dl>
        <p className="mt-3 text-[0.74rem] text-ink-300">
          {t('checkout.fxSnapshot', { rate: (order.fx_rate_used ?? 0).toFixed(3) })}
        </p>

        {/* address */}
        <div className="mt-6 rounded-lg border border-cobalt-100 bg-wash/60 px-4 py-3.5 text-[0.84rem] leading-relaxed text-ink-600">
          <p className="font-semibold text-ink-700">{t('checkout.deliverTo')}</p>
          <p className="mt-1">
            {order.address.recipient}, {order.address.line1}
            {order.address.line2 ? `, ${order.address.line2}` : ''}, {order.address.city}{' '}
            {order.address.postal_code},{' '}
            {new Intl.DisplayNames([locale], { type: 'region' }).of(order.address.country)}
          </p>
        </div>

        <p className="mt-4 text-[0.76rem] text-ink-300">{t('orders.refundNote')}</p>

        {order.status === 'created' && (
          <div className="mt-5 border-t border-cobalt-50 pt-5">
            {!showCancel ? (
              <Button variant="danger" size="sm" onClick={() => setShowCancel(true)}>
                {t('orders.cancelOrder')}
              </Button>
            ) : (
              <div className="rounded-xl border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-bg)]/50 p-5">
                <p className="text-[0.86rem] leading-relaxed text-ink-600">
                  {t('orders.cancelBody')}
                </p>
                <input
                  className="input-base mt-3"
                  placeholder={t('orders.cancelReason')}
                  value={cancelReason}
                  onChange={(e) => setCancelReason(e.target.value)}
                />
                <div className="mt-4 flex gap-3">
                  <Button variant="ghost" size="sm" onClick={() => setShowCancel(false)}>
                    {t('common.close')}
                  </Button>
                  <Button
                    variant="danger"
                    size="sm"
                    loading={cancelling}
                    onClick={() => void cancel()}
                  >
                    {t('orders.cancelConfirm')}
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
