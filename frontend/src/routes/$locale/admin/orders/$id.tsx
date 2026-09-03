import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatMinor } from '~/lib/money'
import type { Order } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/orders/$id')({
  component: OrderDetailPage,
})

function OrderDetailPage() {
  const { id } = Route.useParams()
  const { t, locale } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [order, setOrder] = useState<Order | null | undefined>(undefined)
  const [err, setErr] = useState<string | null>(null)
  const [carrier, setCarrier] = useState('')
  const [tracking, setTracking] = useState('')
  const [actionLoading, setActionLoading] = useState(false)

  useEffect(() => {
    if (!ready || !token) return
    api
      .adminGetOrder(token, Number(id))
      .then((o) => {
        setOrder(o)
        setCarrier(o.carrier_name ?? '')
        setTracking(o.tracking_number ?? '')
      })
      .catch(() => setOrder(null))
  }, [ready, token, id])

  const canWrite = hasPermission('order.write')
  const canRefund = hasPermission('order.refund')

  const ship = async () => {
    if (!token) return
    setActionLoading(true)
    setErr(null)
    try {
      const updated = await api.adminShipOrder(token, Number(id), {
        carrier_name: carrier,
        tracking_number: tracking,
      })
      setOrder(updated)
      push({ title: t('admin.orders.shipped'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setActionLoading(false)
    }
  }

  const complete = async () => {
    if (!token) return
    setActionLoading(true)
    setErr(null)
    try {
      const updated = await api.adminCompleteOrder(token, Number(id))
      setOrder(updated)
      push({ title: t('admin.orders.completed'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setActionLoading(false)
    }
  }

  const refund = async () => {
    if (!token) return
    if (!confirm(t('admin.orders.refundConfirm'))) return
    setActionLoading(true)
    setErr(null)
    try {
      const updated = await api.adminRefundOrder(token, Number(id))
      setOrder(updated)
      push({ title: t('admin.orders.refunded'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setActionLoading(false)
    }
  }

  if (order === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-6 w-6 text-cobalt-400" />
      </div>
    )
  }
  if (order === null) {
    return (
      <div className="py-32 text-center text-[0.88rem] text-ink-400">
        <p>{t('admin.common.empty')}</p>
        <Link to={`/${locale}/admin/orders` as never} className="mt-4 link-quiet">
          {t('admin.common.back')}
        </Link>
      </div>
    )
  }

  return (
    <div>
      <Link
        to={`/${locale}/admin/orders` as never}
        className="inline-flex items-center gap-1.5 text-[0.84rem] text-ink-500 hover:text-cobalt-700"
      >
        <ArrowLeft size={14} /> {t('admin.nav.orders')}
      </Link>

      <div className="mt-4 flex items-center gap-3">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">#{order.id}</h2>
        <span className="text-[0.84rem] text-ink-500">{order.status}</span>
      </div>

      {err && <FieldError>{err}</FieldError>}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        {/* Order details */}
        <div className="card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">Items</h3>
          {order.items && order.items.length > 0 ? (
            <div className="flex flex-col gap-2">
              {order.items.map((item) => (
                <div key={item.id} className="flex justify-between text-[0.84rem] text-ink-700">
                  <span>
                    {item.title_snapshot} × {item.qty}
                  </span>
                  <span>{formatMinor(item.unit_price_minor, order.currency, locale)}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
          )}

          <div className="mt-4 border-t border-cobalt-50 pt-4 text-[0.84rem]">
            <div className="flex justify-between text-ink-600">
              <span>Subtotal</span>
              <span>{formatMinor(order.subtotal_minor, order.currency, locale)}</span>
            </div>
            <div className="flex justify-between text-ink-600">
              <span>Shipping</span>
              <span>{formatMinor(order.shipping_minor, order.currency, locale)}</span>
            </div>
            <div className="mt-2 flex justify-between font-semibold text-ink-900">
              <span>Total</span>
              <span>{formatMinor(order.total_minor, order.currency, locale)}</span>
            </div>
          </div>

          <div className="mt-4 border-t border-cobalt-50 pt-4 text-[0.82rem] text-ink-600">
            <p>
              <strong>Ship to:</strong> {order.address.recipient}, {order.address.line1}
              {order.address.line2 ? `, ${order.address.line2}` : ''}, {order.address.city},{' '}
              {order.address.country}
            </p>
          </div>
        </div>

        {/* Actions */}
        <div className="card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">
            {t('admin.common.status')}
          </h3>
          <div className="flex flex-col gap-3">
            {canWrite && order.status === 'paid' && (
              <div className="flex flex-col gap-2">
                <div>
                  <label className="label-base">{t('admin.orders.shipCarrier')}</label>
                  <input
                    className="input-base"
                    value={carrier}
                    onChange={(e) => setCarrier(e.target.value)}
                  />
                </div>
                <div>
                  <label className="label-base">{t('admin.orders.shipTracking')}</label>
                  <input
                    className="input-base"
                    value={tracking}
                    onChange={(e) => setTracking(e.target.value)}
                  />
                </div>
                <Button variant="secondary" loading={actionLoading} onClick={() => void ship()}>
                  {t('admin.orders.ship')}
                </Button>
              </div>
            )}
            {canWrite && order.status === 'shipped' && (
              <Button variant="secondary" loading={actionLoading} onClick={() => void complete()}>
                {t('admin.orders.complete')}
              </Button>
            )}
            {canRefund && order.status !== 'refunded' && order.status !== 'cancelled' && (
              <Button variant="danger" loading={actionLoading} onClick={() => void refund()}>
                {t('admin.orders.refund')}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
