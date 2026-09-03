import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { AdminTable, DetailLinkCell } from '~/components/admin/ContentTable'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatMinor } from '~/lib/money'
import type { Order } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/orders/')({
  component: OrdersListPage,
})

function OrdersListPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const [list, setList] = useState<Order[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListOrders(token)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, t])

  if (!ready || !token) return null

  const columns: Column<Order>[] = [
    {
      header: t('admin.orders.id'),
      cell: (row) => (
        <DetailLinkCell to={`/${locale}/admin/orders/${row.id}`} label={`#${row.id}`} />
      ),
    },
    { header: t('admin.orders.customer'), cell: (row) => row.user_id },
    {
      header: t('admin.common.status'),
      cell: (row) => row.status,
    },
    {
      header: t('admin.orders.total'),
      cell: (row) => `${formatMinor(row.total_minor, row.currency, locale)}`,
    },
    {
      header: t('admin.orders.date'),
      cell: (row) => new Date(row.placed_at).toLocaleDateString(),
    },
  ]

  return (
    <div>
      <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.nav.orders')}</h2>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
