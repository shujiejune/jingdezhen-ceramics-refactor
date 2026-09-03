import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { AdminTable, DetailLinkCell, StatusBadge } from '~/components/admin/ContentTable'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { Product } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/products/')({
  component: ProductsListPage,
})

function ProductsListPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const [list, setList] = useState<Product[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListProducts(token, locale)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, locale, t])

  if (!ready || !token) return null

  const columns: Column<Product>[] = [
    {
      header: t('admin.products.title'),
      cell: (row) => (
        <DetailLinkCell to={`/${locale}/admin/products/${row.id}`} label={row.title} />
      ),
    },
    { header: t('admin.products.artist'), cell: (row) => row.artist_name ?? '—' },
    { header: t('admin.products.category'), cell: (row) => row.category ?? '—' },
    { header: t('admin.products.skus'), cell: (row) => row.skus?.length ?? 0 },
    {
      header: t('admin.common.status'),
      cell: (row) => <StatusBadge status={row.status} />,
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.content.products')}</h2>
      </div>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
