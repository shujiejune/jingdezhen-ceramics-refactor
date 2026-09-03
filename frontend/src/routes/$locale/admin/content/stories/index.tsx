import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { AdminTable, DetailLinkCell, StatusBadge } from '~/components/admin/ContentTable'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { CeramicStory } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/content/stories/')({
  component: StoriesListPage,
})

function StoriesListPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const [list, setList] = useState<CeramicStory[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListStories(token, locale)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, locale, t])

  if (!ready || !token) return null

  const columns: Column<CeramicStory>[] = [
    {
      header: t('admin.common.title'),
      cell: (row) => (
        <DetailLinkCell to={`/${locale}/admin/content/stories/${row.id}`} label={row.title} />
      ),
    },
    { header: t('admin.common.slug'), cell: (row) => row.slug },
    {
      header: t('admin.common.status'),
      cell: (row) => <StatusBadge status={row.status} />,
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.content.stories')}</h2>
      </div>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
