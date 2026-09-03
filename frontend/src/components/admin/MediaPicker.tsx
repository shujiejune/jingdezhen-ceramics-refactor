/**
 * Media picker panel for attaching media assets to an entity (PRD §3.4.1).
 * Renders as an inline panel showing the media library grid with pick buttons.
 */
import { Check } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { FieldError, Spinner } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { MediaAsset } from '~/lib/types'

export interface MediaPickerProps {
  /** Called when the user picks an asset. */
  onPick: (asset: MediaAsset) => void
  /** Optional list of asset IDs already attached (to show as disabled). */
  attachedIds?: number[]
}

export function MediaPicker({ onPick, attachedIds = [] }: MediaPickerProps) {
  const { t } = useI18n()
  const { ready, token } = useAuth()
  const [assets, setAssets] = useState<MediaAsset[] | null>(null)
  const [error, setError] = useState<string | null>(null)

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

  if (assets === null) {
    return (
      <div className="flex justify-center py-8">
        <Spinner className="h-5 w-5 text-cobalt-400" />
      </div>
    )
  }

  if (assets.length === 0) {
    return <p className="py-8 text-center text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
  }

  return (
    <div>
      {error && <FieldError>{error}</FieldError>}
      <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
        {assets.map((asset) => {
          const attached = attachedIds.includes(asset.id)
          return (
            <button
              key={asset.id}
              type="button"
              disabled={attached}
              onClick={() => onPick(asset)}
              className="card-surface flex flex-col items-center gap-1.5 p-2 text-center transition hover:border-cobalt-300 disabled:opacity-50"
            >
              <div className="flex h-16 items-center justify-center rounded bg-wash/50">
                <img
                  src={asset.public_url}
                  alt={asset.caption ?? ''}
                  width={64}
                  height={64}
                  className="max-h-full max-w-full object-contain"
                  onError={(e) => {
                    ;(e.target as HTMLImageElement).style.display = 'none'
                  }}
                />
              </div>
              <span className="flex items-center gap-1 text-[0.72rem] text-ink-500">
                {attached && <Check size={11} className="text-success-500" />}
                {asset.mime_type ?? 'image'}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
