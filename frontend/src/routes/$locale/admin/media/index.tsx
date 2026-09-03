import { createFileRoute } from '@tanstack/react-router'
import { Trash, UploadSimple } from '@phosphor-icons/react'
import { useEffect, useRef, useState } from 'react'

import { Button, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { MediaAsset } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/media/')({
  component: MediaLibraryPage,
})

function MediaLibraryPage() {
  const { t } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [assets, setAssets] = useState<MediaAsset[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const canWrite = hasPermission('content.write')

  useEffect(() => {
    if (!ready || !token) return
    api
      .adminListMediaAssets(token)
      .then((res) => setAssets(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setAssets([])
      })
  }, [ready, token, t])

  const onUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!token || !e.target.files?.length) return
    setUploading(true)
    setError(null)
    try {
      const fd = new FormData()
      fd.append('file', e.target.files[0]!)
      await api.adminUploadLocal(token, fd)
      const res = await api.adminListMediaAssets(token)
      setAssets(res.data)
      push({ title: t('admin.media.upload'), kind: 'success' })
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const onDelete = async (id: number) => {
    if (!token) return
    try {
      await api.adminDeleteAsset(token, id)
      setAssets((prev) => prev?.filter((a) => a.id !== id) ?? null)
      push({ title: t('admin.media.delete'), kind: 'success' })
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    }
  }

  if (!ready || !token) return null

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.media.title')}</h2>
        {canWrite && (
          <div>
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(e) => void onUpload(e)}
            />
            <Button
              variant="secondary"
              loading={uploading}
              onClick={() => fileRef.current?.click()}
            >
              <UploadSimple size={15} /> {t('admin.media.upload')}
            </Button>
          </div>
        )}
      </div>

      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}

      {assets === null ? (
        <div className="flex justify-center py-16">
          <Spinner className="h-6 w-6 text-cobalt-400" />
        </div>
      ) : assets.length === 0 ? (
        <div className="py-16 text-center text-[0.88rem] text-ink-400">
          {t('admin.common.empty')}
        </div>
      ) : (
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {assets.map((asset) => (
            <div key={asset.id} className="card-surface flex flex-col gap-2 overflow-hidden p-3">
              <div className="flex h-32 items-center justify-center rounded-lg bg-wash/50">
                <img
                  src={asset.public_url}
                  alt={asset.caption ?? ''}
                  width={256}
                  height={128}
                  className="max-h-full max-w-full object-contain"
                  onError={(e) => {
                    ;(e.target as HTMLImageElement).style.display = 'none'
                  }}
                />
              </div>
              <div className="flex items-center justify-between text-[0.78rem] text-ink-500">
                <span>{asset.mime_type ?? 'image'}</span>
                <span>{Math.round((asset.file_size ?? 0) / 1024)} KB</span>
              </div>
              {asset.caption && <p className="text-[0.8rem] text-ink-600">{asset.caption}</p>}
              {canWrite && (
                <Button variant="danger" size="sm" onClick={() => void onDelete(asset.id)}>
                  <Trash size={13} /> {t('admin.media.delete')}
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
