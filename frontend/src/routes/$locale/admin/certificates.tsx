import { createFileRoute } from '@tanstack/react-router'
import { ArrowsClockwise } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { AdminTable } from '~/components/admin/ContentTable'
import { Button } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { Certificate } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/certificates')({
  component: CertificatesPage,
})

function CertificatesPage() {
  const { t } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [list, setList] = useState<Certificate[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const canManage = hasPermission('certificate.manage')

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListCertificates(token)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, t])

  const regenerate = async (id: number) => {
    if (!token) return
    try {
      await api.adminRegenerateCertificate(token, id)
      push({ title: t('admin.cert.regenerated'), kind: 'success' })
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  if (!ready || !token) return null

  const columns: Column<Certificate>[] = [
    { header: t('admin.cert.code'), cell: (row) => row.cert_code },
    { header: t('admin.cert.product'), cell: (row) => row.product_title },
    { header: t('admin.cert.artist'), cell: (row) => row.artist_name },
    {
      header: t('admin.cert.issued'),
      cell: (row) => new Date(row.issued_at).toLocaleDateString(),
    },
    {
      header: t('admin.cert.regenerate'),
      cell: (row) =>
        canManage ? (
          <Button variant="secondary" size="sm" onClick={() => void regenerate(row.id)}>
            <ArrowsClockwise size={13} /> {t('admin.cert.regenerate')}
          </Button>
        ) : (
          '—'
        ),
    },
  ]

  return (
    <div>
      <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.cert.title')}</h2>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
