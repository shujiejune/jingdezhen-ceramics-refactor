import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { AdminTable, DetailLinkCell } from '~/components/admin/ContentTable'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { ItineraryRequest } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/itineraries/')({
  component: ItinerariesListPage,
})

function ItinerariesListPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const [list, setList] = useState<ItineraryRequest[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListItineraries(token)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, t])

  if (!ready || !token) return null

  const columns: Column<ItineraryRequest>[] = [
    {
      header: t('admin.itin.id'),
      cell: (row) => (
        <DetailLinkCell to={`/${locale}/admin/itineraries/${row.id}`} label={`#${row.id}`} />
      ),
    },
    { header: t('admin.itin.status'), cell: (row) => row.status },
    { header: 'Arrival', cell: (row) => new Date(row.arrival_date).toLocaleDateString() },
    { header: t('admin.itin.sla'), cell: (row) => new Date(row.sla_deadline).toLocaleDateString() },
  ]

  return (
    <div>
      <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.nav.itineraries')}</h2>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
