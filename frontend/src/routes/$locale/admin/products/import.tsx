import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, UploadSimple } from '@phosphor-icons/react'
import { useRef, useState } from 'react'

import { Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { BulkImportSummary } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/products/import')({
  component: ProductImportPage,
})

function ProductImportPage() {
  const { t, locale } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const fileRef = useRef<HTMLInputElement>(null)
  const [preview, setPreview] = useState<string[][] | null>(null)
  const [summary, setSummary] = useState<BulkImportSummary | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const canWrite = hasPermission('product.write')

  const onFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setError(null)
    setSummary(null)
    setLoading(true)
    try {
      const text = await file.text()
      const rows = text
        .split('\n')
        .filter((l) => l.trim())
        .map((l) => l.split(',').map((c) => c.trim()))
      setPreview(rows.slice(0, 5))
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  const doImport = async () => {
    if (!token) return
    setLoading(true)
    setError(null)
    try {
      const csv = (preview ?? []).map((row) => row.join(',')).join('\n')
      const result = await api.adminBulkImportProducts(token, { csv })
      setSummary(result)
      push({ title: t('admin.products.import'), kind: 'success' })
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  if (!ready || !token) return null

  return (
    <div>
      <Link
        to={`/${locale}/admin/products` as never}
        className="inline-flex items-center gap-1.5 text-[0.84rem] text-ink-500 hover:text-cobalt-700"
      >
        <ArrowLeft size={14} /> {t('admin.content.products')}
      </Link>

      <h2 className="mt-4 text-[1.1rem] font-semibold text-ink-900">
        {t('admin.products.import')}
      </h2>

      {error && <FieldError>{error}</FieldError>}

      <div className="mt-6 card-surface p-6">
        <input
          ref={fileRef}
          type="file"
          accept=".csv,text/csv"
          className="hidden"
          onChange={(e) => void onFile(e)}
        />
        <Button variant="secondary" disabled={!canWrite} onClick={() => fileRef.current?.click()}>
          <UploadSimple size={15} /> {t('admin.products.import')}
        </Button>

        {loading && (
          <div className="mt-4 flex justify-center">
            <Spinner className="h-5 w-5 text-cobalt-400" />
          </div>
        )}

        {preview && preview.length > 0 && (
          <div className="mt-6">
            <h3 className="mb-2 text-[0.88rem] font-semibold text-ink-700">
              {t('admin.products.importPreview')}
            </h3>
            <div className="overflow-x-auto rounded-lg border border-cobalt-50">
              <table className="w-full text-left text-[0.8rem]">
                <tbody>
                  {preview.map((row, ri) => (
                    <tr key={ri} className="border-b border-cobalt-50/60">
                      {row.map((cell, ci) => (
                        <td key={ci} className="px-3 py-2 text-ink-600">
                          {cell}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Button className="mt-4" disabled={!canWrite} onClick={() => void doImport()}>
              {t('admin.products.importSubmit').replace('{count}', String(preview.length))}
            </Button>
          </div>
        )}

        {summary && (
          <div className="mt-6 rounded-lg bg-wash/30 p-4 text-[0.84rem] text-ink-700">
            {t('admin.products.importResult')
              .replace('{imported}', String(summary.imported))
              .replace('{updated}', String(summary.updated))
              .replace('{failed}', String(summary.failed))}
          </div>
        )}
      </div>
    </div>
  )
}
