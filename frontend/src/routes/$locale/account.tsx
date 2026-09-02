import { Link, createFileRoute } from '@tanstack/react-router'
import { DownloadSimple, Trash, MapPin } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, ButtonLink, Spinner, Badge } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { SUPPORTED_CURRENCIES } from '~/lib/utils'
import { formatDate } from '~/lib/utils'
import type { Address } from '~/lib/types'

/** Account — profile, preferences, address book, privacy actions (PRD §3.5). */
export const Route = createFileRoute('/$locale/account')({
  component: AccountPage,
})

function AccountPage() {
  const { t, locale, currency, setCurrency } = useI18n()
  const { ready, token, user, logout } = useAuth()
  const { push } = useToast()
  const [addresses, setAddresses] = useState<Address[] | null>(null)
  const [nickname, setNickname] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (token) {
      setNickname(user?.nickname ?? '')
      void api
        .getAddresses(token)
        .then(setAddresses)
        .catch(() => setAddresses([]))
    }
  }, [token, user])

  if (!ready) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (!token || !user) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('checkout.signInFirst')}</h1>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/account` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  const save = async () => {
    setSaving(true)
    try {
      await api.updateProfile(token, {
        nickname,
        preferred_currency: currency,
        preferred_locale: locale,
      })
      push({ title: t('account.saved') })
    } finally {
      setSaving(false)
    }
  }

  const exportData = () => {
    const blob = new Blob(
      [
        JSON.stringify(
          { exported_at: new Date().toISOString(), profile: user, addresses },
          null,
          2,
        ),
      ],
      { type: 'application/json' },
    )
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'jdz-account-export.json'
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('nav.account')}</p>
      <h1 className="mt-2 text-display-sm text-ink-900">{t('account.title')}</h1>
      <p className="mt-2 text-[0.84rem] text-ink-400">
        {t('account.memberSince', { date: formatDate(user.created_at, locale) })}
      </p>

      {/* profile */}
      <section className="card-surface mt-8 p-6">
        <h2 className="text-[1rem] font-semibold text-ink-900">{t('account.profile')}</h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <label className="label-base">{t('auth.nickname')}</label>
            <input
              className="input-base"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
            />
          </div>
          <div>
            <label className="label-base">{t('auth.email')}</label>
            <input className="input-base" value={user.email} disabled />
          </div>
        </div>

        <h3 className="mt-6 text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">
          {t('account.preferences')}
        </h3>
        <div className="mt-3 grid gap-4 sm:grid-cols-2">
          <div>
            <label className="label-base">{t('account.currencyPref')}</label>
            <select
              className="input-base"
              value={currency}
              onChange={(e) => setCurrency(e.target.value as 'USD')}
            >
              {SUPPORTED_CURRENCIES.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="label-base">{t('account.localePref')}</label>
            <input
              className="input-base"
              value={locale === 'zh-CN' ? '中文（简体）' : 'English (US)'}
              disabled
            />
          </div>
        </div>

        <Button className="mt-6" loading={saving} onClick={() => void save()}>
          {t('common.save')}
        </Button>
      </section>

      {/* addresses */}
      <section className="card-surface mt-6 p-6">
        <h2 className="text-[1rem] font-semibold text-ink-900">{t('account.addresses')}</h2>
        {addresses === null ? (
          <Spinner className="mt-6 h-5 w-5 text-cobalt-400" />
        ) : (
          <ul className="mt-4 flex flex-col gap-3">
            {addresses.map((a) => (
              <li
                key={a.id}
                className="flex items-start justify-between gap-4 rounded-lg border border-cobalt-100 bg-wash/50 p-4"
              >
                <div className="flex items-start gap-3">
                  <MapPin size={17} className="mt-0.5 text-cobalt-500" weight="duotone" />
                  <div>
                    <p className="flex items-center gap-2 text-[0.88rem] font-semibold text-ink-800">
                      {a.recipient}
                      {a.is_default && <Badge tone="cobalt">{t('checkout.defaultBadge')}</Badge>}
                    </p>
                    <p className="mt-1 text-[0.82rem] leading-relaxed text-ink-500">
                      {a.line1}
                      {a.line2 ? `, ${a.line2}` : ''}, {a.city} {a.postal_code},{' '}
                      {new Intl.DisplayNames([locale], { type: 'region' }).of(a.country)}
                    </p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
        <ButtonLink to={`/${locale}/checkout`} variant="secondary" size="sm" className="mt-4">
          + {t('account.addAddress')}
        </ButtonLink>
      </section>

      {/* privacy */}
      <section className="card-surface mt-6 p-6">
        <h2 className="text-[1rem] font-semibold text-ink-900">{t('account.privacyTitle')}</h2>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button variant="secondary" onClick={exportData}>
            <DownloadSimple size={15} />
            {t('account.exportData')}
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              logout()
              void push({ title: t('account.deleteNote'), kind: 'info' })
            }}
          >
            <Trash size={15} />
            {t('account.deleteAccount')}
          </Button>
        </div>
        <p className="mt-4 text-[0.78rem] leading-relaxed text-ink-300">
          {t('account.deleteNote')}
        </p>
      </section>
    </div>
  )
}
