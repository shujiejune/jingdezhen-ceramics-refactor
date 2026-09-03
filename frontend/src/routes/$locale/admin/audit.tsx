import { createFileRoute } from '@tanstack/react-router'
import { Download } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { AdminTable } from '~/components/admin/ContentTable'
import { Button } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { AuditLog } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/audit')({
  component: AuditPage,
})

function AuditPage() {
  const { t } = useI18n()
  const { ready, token } = useAuth()
  const { push } = useToast()
  const [list, setList] = useState<AuditLog[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListAuditLog(token)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, t])

  if (!ready || !token) return null

  const exportCsv = () => {
    push({ title: 'Audit CSV', kind: 'info' })
  }

  const columns: Column<AuditLog>[] = [
    {
      header: t('admin.audit.timestamp'),
      cell: (row) => new Date(row.created_at).toLocaleString(),
    },
    { header: t('admin.audit.user'), cell: (row) => row.user_email },
    { header: t('admin.audit.action'), cell: (row) => row.action },
    {
      header: t('admin.audit.entity'),
      cell: (row) => `${row.entity_type}${row.entity_id ? ` #${row.entity_id}` : ''}`,
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.audit.title')}</h2>
        <Button variant="secondary" size="sm" onClick={exportCsv}>
          <Download size={13} /> {t('admin.audit.exportCsv')}
        </Button>
      </div>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
