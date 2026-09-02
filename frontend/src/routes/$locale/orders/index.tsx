import { Link, createFileRoute } from '@tanstack/react-router'
import { Package } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { Badge, ButtonLink, EmptyState, Spinner } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatDate } from '~/lib/utils'
import type { Order, OrderStatus } from '~/lib/types'

const statusTone: Record<OrderStatus, 'cobalt' | 'success' | 'warning' | 'danger' | 'neutral'> = {
  created: 'warning',
  paid: 'cobalt',
  shipped: 'cobalt',
  completed: 'success',
  cancelled: 'neutral',
  refunded: 'danger',
}

/** Order history (client-rendered — personalized, no SSR need). */
export const Route = createFileRoute('/$locale/orders/')({
  component: OrdersPage,
})

export function OrderStatusBadge({ status }: { status: OrderStatus }) {
  const { t } = useI18n()
  return (
    <Badge tone={statusTone[status]}>
      {t(`orders.status.${status}` as Parameters<typeof t>[0])}
    </Badge>
  )
}

function OrdersPage() {
  const { t, locale, price } = useI18n()
  const { ready, token, user } = useAuth()
  const [orders, setOrders] = useState<Order[] | null>(null)

  useEffect(() => {
    if (ready && token && !user) void setOrders(null)
    if (ready && !token) setOrders([])
    if (ready && token)
      void api
        .listOrders(token, locale)
        .then(setOrders)
        .catch(() => setOrders([]))
  }, [ready, token, locale, user])

  if (!ready || (token && orders === null)) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (!token) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('checkout.signInFirst')}</h1>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/orders` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('nav.account')}</p>
      <h1 className="mt-2 text-display-sm text-ink-900">{t('orders.title')}</h1>
      <p className="mt-2 text-[0.92rem] text-ink-500">{t('orders.subtitle')}</p>

      {orders && orders.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={<Package size={40} weight="duotone" />}
            title={t('orders.empty')}
            body={t('orders.emptyBody')}
            action={<ButtonLink to={`/${locale}/catalog`}>{t('cart.emptyCta')}</ButtonLink>}
          />
        </div>
      ) : (
        <div className="mt-8 flex flex-col gap-4">
          {orders?.map((o) => (
            <div key={o.id} className="card-surface p-5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <span className="text-[0.95rem] font-semibold text-ink-900">
                    {t('orders.orderN', { id: o.id })}
                  </span>
                  <OrderStatusBadge status={o.status} />
                </div>
                <span className="text-[0.82rem] text-ink-400">
                  {t('orders.placedOn', { date: formatDate(o.placed_at, locale) })}
                </span>
              </div>

              <div className="mt-4 flex items-center justify-between gap-4">
                <div className="flex -space-x-3">
                  {o.items?.slice(0, 4).map((i) => (
                    <div
                      key={i.id}
                      className="h-14 w-14 overflow-hidden rounded-lg border border-cobalt-100 bg-wash"
                      title={i.title_snapshot as string}
                    >
                      <PorcelainFigure
                        kind={i.figure_kind ?? 'vase'}
                        seed={i.figure_seed ?? 1}
                        className="h-full w-full"
                      />
                    </div>
                  ))}
                </div>
                <div className="text-right">
                  <p className="text-[0.72rem] text-ink-400">
                    {o.items?.reduce((s, i) => s + i.qty, 0)} {t('orders.items')}
                  </p>
                  <p className="text-[1.02rem] font-semibold text-ink-900">
                    {price(o.total_minor, o.currency)}
                  </p>
                </div>
                <ButtonLink
                  to="/$locale/orders/$id"
                  params={{ locale, id: String(o.id) }}
                  variant="secondary"
                  size="sm"
                >
                  {t('orders.view')}
                </ButtonLink>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
