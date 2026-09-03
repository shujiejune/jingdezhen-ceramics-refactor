/** Cookie consent banner — shown on first visit until the visitor chooses. */
import { Cookie } from '@phosphor-icons/react'
import { useConsent } from '~/lib/consent'
import { useI18n } from '~/lib/i18n'

export function ConsentBanner() {
  const { t } = useI18n()
  const { needsBanner, acceptAll, acceptEssential } = useConsent()

  if (!needsBanner) return null

  return (
    <div
      className="fixed inset-x-0 bottom-0 z-50 px-4 pb-4"
      role="region"
      aria-label={t('consent.bannerBody')}
    >
      <div className="mx-auto flex max-w-3xl flex-col gap-3 rounded-xl border border-cobalt-200 bg-white p-5 shadow-lg sm:flex-row sm:items-center sm:gap-6">
        <Cookie size={28} weight="duotone" className="hidden shrink-0 text-cobalt-600 sm:block" />
        <div className="flex-1">
          <p className="text-[0.88rem] leading-relaxed text-ink-700">{t('consent.bannerBody')}</p>
        </div>
        <div className="flex shrink-0 gap-2">
          <button
            type="button"
            onClick={acceptEssential}
            className="rounded-lg border border-cobalt-200 px-4 py-2 text-[0.84rem] font-medium text-ink-600 transition hover:bg-wash"
          >
            {t('consent.essentialOnly')}
          </button>
          <button
            type="button"
            onClick={acceptAll}
            className="rounded-lg bg-cobalt-600 px-4 py-2 text-[0.84rem] font-medium text-white transition hover:bg-cobalt-700"
          >
            {t('consent.acceptAll')}
          </button>
        </div>
      </div>
    </div>
  )
}
