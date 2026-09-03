import { Link, createFileRoute } from '@tanstack/react-router'
import { DownloadSimple, MapPin, PencilSimple, Trash, Warning } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Badge, Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { SUPPORTED_CURRENCIES, formatDate } from '~/lib/utils'
import type { Address, ConsentRecord } from '~/lib/types'

/** Account — profile, preferences, address book, consent history, GDPR (PRD §3.5). */
export const Route = createFileRoute('/$locale/account')({
  component: AccountPage,
})

function AccountPage() {
  const { t, locale, currency, setCurrency } = useI18n()
  const { ready, token, user, logout } = useAuth()
  const { push } = useToast()
  const [addresses, setAddresses] = useState<Address[] | null>(null)
  const [consentRecords, setConsentRecords] = useState<ConsentRecord[]>([])
  const [nickname, setNickname] = useState('')
  const [saving, setSaving] = useState(false)

  // address editing state
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showAddrForm, setShowAddrForm] = useState(false)

  // GDPR delete state
  const [deleteConfirm, setDeleteConfirm] = useState('')
  const [deleting, setDeleting] = useState(false)
  const [showDeleteForm, setShowDeleteForm] = useState(false)

  const reloadAddresses = () => {
    if (!token) return
    void api
      .getAddresses(token)
      .then(setAddresses)
      .catch(() => setAddresses([]))
  }

  useEffect(() => {
    if (token) {
      setNickname(user?.nickname ?? '')
      void api
        .getAddresses(token)
        .then(setAddresses)
        .catch(() => setAddresses([]))
      void api
        .getConsentHistory(token)
        .then((res) => setConsentRecords(res))
        .catch(() => setConsentRecords([]))
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

  const exportData = async () => {
    try {
      const data = await api.exportUserData(token, locale)
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'jdz-account-export.json'
      a.click()
      URL.revokeObjectURL(url)
      push({ title: t('account.exportReady') })
    } catch (err) {
      push({ title: t(errorKey(err) as Parameters<typeof t>[0]), kind: 'error' })
    }
  }

  const deleteAccount = async () => {
    if (deleteConfirm !== 'DELETE') return
    setDeleting(true)
    try {
      await api.deleteAccount(token)
      logout()
      push({ title: t('account.deleted'), kind: 'info' })
    } catch (err) {
      push({ title: t(errorKey(err) as Parameters<typeof t>[0]), kind: 'error' })
    } finally {
      setDeleting(false)
    }
  }

  const onDeleteAddress = async (id: number) => {
    try {
      await api.deleteAddress(token, id)
      reloadAddresses()
      push({ title: t('address.deleted') })
    } catch (err) {
      push({ title: t(errorKey(err) as Parameters<typeof t>[0]), kind: 'error' })
    }
  }

  const onSetDefault = async (id: number) => {
    try {
      await api.setDefaultAddress(token, id)
      reloadAddresses()
    } catch (err) {
      push({ title: t(errorKey(err) as Parameters<typeof t>[0]), kind: 'error' })
    }
  }

  const consentLabel = (kind: string) => {
    const key = `consent.${kind === 'privacy_policy' ? 'privacy' : kind === 'tos' ? 'tos' : kind === 'cookie_analytics' ? 'analytics' : 'marketing'}`
    return t(key as Parameters<typeof t>[0])
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
            <label className="label-base" htmlFor="ac-nick">
              {t('auth.nickname')}
            </label>
            <input
              id="ac-nick"
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
              <li key={a.id} className="rounded-lg border border-cobalt-100 bg-wash/50 p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-3">
                    <MapPin size={17} className="mt-0.5 text-cobalt-500" weight="duotone" />
                    <div>
                      <p className="flex items-center gap-2 text-[0.88rem] font-semibold text-ink-800">
                        {a.recipient}
                        {a.is_default && <Badge tone="cobalt">{t('account.default')}</Badge>}
                      </p>
                      <p className="mt-1 text-[0.82rem] leading-relaxed text-ink-500">
                        {a.line1}
                        {a.line2 ? `, ${a.line2}` : ''}, {a.city} {a.postal_code},{' '}
                        {new Intl.DisplayNames([locale], { type: 'region' }).of(a.country)}
                      </p>
                      {a.phone && <p className="mt-0.5 text-[0.78rem] text-ink-400">{a.phone}</p>}
                    </div>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <button
                      type="button"
                      onClick={() => {
                        setEditingId(a.id)
                        setShowAddrForm(true)
                      }}
                      className="rounded p-1.5 text-ink-400 hover:bg-cobalt-50 hover:text-cobalt-600"
                      aria-label={t('account.editAddress')}
                    >
                      <PencilSimple size={14} />
                    </button>
                    <button
                      type="button"
                      onClick={() => void onDeleteAddress(a.id)}
                      className="rounded p-1.5 text-ink-400 hover:bg-red-50 hover:text-red-600"
                      aria-label={t('account.deleteAddress')}
                    >
                      <Trash size={14} />
                    </button>
                  </div>
                </div>
                {!a.is_default && (
                  <button
                    type="button"
                    onClick={() => void onSetDefault(a.id)}
                    className="mt-2 text-[0.78rem] text-cobalt-600 hover:text-cobalt-700"
                  >
                    {t('account.setDefault')}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
        <div className="mt-4 flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              setEditingId(null)
              setShowAddrForm(true)
            }}
          >
            + {t('account.addAddress')}
          </Button>
        </div>
        {showAddrForm && (
          <AddressForm
            key={editingId ?? 'new'}
            token={token}
            address={editingId ? addresses?.find((a) => a.id === editingId) : undefined}
            onDone={() => {
              setShowAddrForm(false)
              setEditingId(null)
              reloadAddresses()
            }}
            onCancel={() => {
              setShowAddrForm(false)
              setEditingId(null)
            }}
          />
        )}
      </section>

      {/* consent history */}
      <section className="card-surface mt-6 p-6">
        <h2 className="text-[1rem] font-semibold text-ink-900">{t('account.consentHistory')}</h2>
        {consentRecords.length === 0 ? (
          <p className="mt-4 text-[0.84rem] text-ink-400">{t('account.noConsent')}</p>
        ) : (
          <ul className="mt-4 flex flex-col gap-2">
            {consentRecords.map((r) => (
              <li
                key={r.id}
                className="flex items-center justify-between gap-4 border-b border-cobalt-50 py-2 text-[0.84rem] last:border-0"
              >
                <span className="text-ink-700">{consentLabel(r.kind)}</span>
                <span className="flex items-center gap-2">
                  <span
                    className={
                      r.granted ? 'text-[0.8rem] text-cobalt-600' : 'text-[0.8rem] text-ink-400'
                    }
                  >
                    {r.granted ? t('consent.granted') : t('consent.denied')}
                  </span>
                  <span className="text-ink-300">{formatDate(r.created_at, locale)}</span>
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* privacy / GDPR */}
      <section className="card-surface mt-6 p-6">
        <h2 className="text-[1rem] font-semibold text-ink-900">{t('account.privacyTitle')}</h2>
        <div className="mt-4 flex flex-wrap gap-3">
          <Button variant="secondary" onClick={() => void exportData()}>
            <DownloadSimple size={15} />
            {t('account.exportData')}
          </Button>
          <Button variant="danger" onClick={() => setShowDeleteForm((v) => !v)}>
            <Trash size={15} />
            {t('account.deleteAccount')}
          </Button>
        </div>
        {showDeleteForm && (
          <div className="mt-4 rounded-lg border border-red-200 bg-red-50/50 p-4">
            <p className="flex items-center gap-2 text-[0.84rem] font-medium text-red-700">
              <Warning size={15} weight="duotone" />
              {t('account.deleteConfirm')}
            </p>
            <input
              className="input-base mt-3"
              value={deleteConfirm}
              onChange={(e) => setDeleteConfirm(e.target.value)}
              placeholder={t('account.deleteConfirmPlaceholder')}
            />
            <Button
              variant="danger"
              className="mt-3"
              loading={deleting}
              disabled={deleteConfirm !== 'DELETE'}
              onClick={() => void deleteAccount()}
            >
              {t('account.deleteConfirmBtn')}
            </Button>
          </div>
        )}
        <p className="mt-4 text-[0.78rem] leading-relaxed text-ink-300">
          {t('account.deleteNote')}
        </p>
      </section>
    </div>
  )
}

/* ----------------------- address form ----------------------- */

function AddressForm({
  token,
  address,
  onDone,
  onCancel,
}: {
  token: string
  address?: Address
  onDone: () => void
  onCancel: () => void
}) {
  const { t } = useI18n()
  const { push } = useToast()
  const [recipient, setRecipient] = useState(address?.recipient ?? '')
  const [line1, setLine1] = useState(address?.line1 ?? '')
  const [line2, setLine2] = useState(address?.line2 ?? '')
  const [city, setCity] = useState(address?.city ?? '')
  const [region, setRegion] = useState(address?.region ?? '')
  const [postalCode, setPostalCode] = useState(address?.postal_code ?? '')
  const [country, setCountry] = useState(address?.country ?? 'US')
  const [phone, setPhone] = useState(address?.phone ?? '')
  const [isDefault, setIsDefault] = useState(address?.is_default ?? false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const body = {
        recipient,
        line1,
        line2,
        city,
        region,
        postal_code: postalCode,
        country,
        phone,
        is_default: isDefault,
      }
      if (address) {
        await api.updateAddress(token, address.id, body)
      } else {
        await api.createAddress(token, body)
      }
      push({ title: t('account.addressSaved') })
      onDone()
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="mt-4 rounded-lg border border-cobalt-100 bg-wash/30 p-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <label className="label-base" htmlFor="af-recipient">
            {t('account.addressRecipient')}
          </label>
          <input
            id="af-recipient"
            className="input-base"
            value={recipient}
            onChange={(e) => setRecipient(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label-base" htmlFor="af-phone">
            {t('account.addressPhone')}
          </label>
          <input
            id="af-phone"
            className="input-base"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        <div className="sm:col-span-2">
          <label className="label-base" htmlFor="af-line1">
            {t('account.addressLine1')}
          </label>
          <input
            id="af-line1"
            className="input-base"
            value={line1}
            onChange={(e) => setLine1(e.target.value)}
            required
          />
        </div>
        <div className="sm:col-span-2">
          <label className="label-base" htmlFor="af-line2">
            {t('account.addressLine2')}
          </label>
          <input
            id="af-line2"
            className="input-base"
            value={line2}
            onChange={(e) => setLine2(e.target.value)}
          />
        </div>
        <div>
          <label className="label-base" htmlFor="af-city">
            {t('account.addressCity')}
          </label>
          <input
            id="af-city"
            className="input-base"
            value={city}
            onChange={(e) => setCity(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label-base" htmlFor="af-region">
            {t('account.addressRegion')}
          </label>
          <input
            id="af-region"
            className="input-base"
            value={region}
            onChange={(e) => setRegion(e.target.value)}
          />
        </div>
        <div>
          <label className="label-base" htmlFor="af-postal">
            {t('account.addressPostal')}
          </label>
          <input
            id="af-postal"
            className="input-base"
            value={postalCode}
            onChange={(e) => setPostalCode(e.target.value)}
            required
          />
        </div>
        <div>
          <label className="label-base" htmlFor="af-country">
            {t('account.addressCountry')}
          </label>
          <select
            id="af-country"
            className="input-base"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
          >
            <option value="US">United States</option>
            <option value="GB">United Kingdom</option>
            <option value="DE">Germany</option>
            <option value="FR">France</option>
            <option value="NL">Netherlands</option>
            <option value="CA">Canada</option>
            <option value="AU">Australia</option>
            <option value="JP">Japan</option>
            <option value="SG">Singapore</option>
            <option value="CN">China</option>
          </select>
        </div>
        <div className="sm:col-span-2">
          <label className="flex items-center gap-2 text-[0.84rem] text-ink-700">
            <input
              type="checkbox"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
              className="h-4 w-4 rounded border-cobalt-200"
            />
            {t('account.setDefault')}
          </label>
        </div>
      </div>
      {error && (
        <div className="mt-3">
          <FieldError>{error}</FieldError>
        </div>
      )}
      <div className="mt-4 flex gap-2">
        <Button type="submit" size="sm" loading={saving}>
          {t('account.saveAddress')}
        </Button>
        <Button type="button" variant="secondary" size="sm" onClick={onCancel}>
          {t('account.cancelEdit')}
        </Button>
      </div>
    </form>
  )
}
