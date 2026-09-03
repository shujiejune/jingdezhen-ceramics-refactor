import { createFileRoute } from '@tanstack/react-router'
import { Plus, Trash } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { OptionRate, ShippingTier } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/settings')({
  component: SettingsPage,
})

function SettingsPage() {
  const { t } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [tiers, setTiers] = useState<ShippingTier[] | null>(null)
  const [rates, setRates] = useState<OptionRate[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [newTier, setNewTier] = useState({
    country_code: '',
    min_weight_grams: '0',
    max_weight_grams: '500',
    fee_cny: '0',
  })
  const [newRate, setNewRate] = useState({ option_key: '', label: '', rate_cny: '0' })

  const canManage = hasPermission('settings.manage')

  const reload = async () => {
    if (!token) return
    setError(null)
    try {
      const [tiersRes, ratesRes] = await Promise.all([
        api.adminListShippingTiers(token),
        api.adminListOptionRates(token),
      ])
      setTiers(tiersRes.data)
      setRates(ratesRes.data)
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  useEffect(() => {
    if (!ready || !token) return
    void reload()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, token])

  const refreshFx = async () => {
    if (!token) return
    try {
      await api.adminRefreshFX(token)
      push({ title: t('admin.settings.fxRefreshed'), kind: 'success' })
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const addTier = async () => {
    if (!token) return
    try {
      await api.adminCreateShippingTier(token, {
        country_code: newTier.country_code,
        min_weight_grams: Number(newTier.min_weight_grams) || 0,
        max_weight_grams: Number(newTier.max_weight_grams) || 0,
        fee_cny: Number(newTier.fee_cny) || 0,
      })
      setNewTier({ country_code: '', min_weight_grams: '0', max_weight_grams: '500', fee_cny: '0' })
      void reload()
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const deleteTier = async (id: number) => {
    if (!token) return
    try {
      await api.adminDeleteShippingTier(token, id)
      void reload()
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const addRate = async () => {
    if (!token) return
    try {
      await api.adminCreateOptionRate(token, {
        option_key: newRate.option_key,
        label: newRate.label,
        rate_cny: Number(newRate.rate_cny) || 0,
      })
      setNewRate({ option_key: '', label: '', rate_cny: '0' })
      void reload()
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const deleteRate = async (id: number) => {
    if (!token) return
    try {
      await api.adminDeleteOptionRate(token, id)
      void reload()
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  if (!ready || !token) return null

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.settings.title')}</h2>
        {canManage && (
          <Button variant="secondary" size="sm" onClick={() => void refreshFx()}>
            {t('admin.settings.refreshFx')}
          </Button>
        )}
      </div>

      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}

      {tiers === null || rates === null ? (
        <div className="flex justify-center py-16">
          <Spinner className="h-6 w-6 text-cobalt-400" />
        </div>
      ) : (
        <div className="mt-6 flex flex-col gap-6">
          {/* Shipping tiers */}
          <div className="card-surface p-6">
            <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">
              {t('admin.settings.shipping')}
            </h3>
            <div className="overflow-x-auto rounded-lg border border-cobalt-50">
              <table className="w-full text-left text-[0.82rem]">
                <thead className="bg-wash/50">
                  <tr className="border-b border-cobalt-50">
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.country')}
                    </th>
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.minWeight')}
                    </th>
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.maxWeight')}
                    </th>
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.feeCny')}
                    </th>
                    {canManage && <th className="px-3 py-2" />}
                  </tr>
                </thead>
                <tbody>
                  {tiers.map((tier) => (
                    <tr key={tier.id} className="border-b border-cobalt-50/60">
                      <td className="px-3 py-2 text-ink-700">{tier.country_code}</td>
                      <td className="px-3 py-2 text-ink-700">{tier.min_weight_grams}</td>
                      <td className="px-3 py-2 text-ink-700">{tier.max_weight_grams}</td>
                      <td className="px-3 py-2 text-ink-700">{tier.fee_cny}</td>
                      {canManage && (
                        <td className="px-3 py-2">
                          <button
                            type="button"
                            onClick={() => void deleteTier(tier.id)}
                            className="text-ink-400 transition hover:text-[color:var(--color-danger)]"
                          >
                            <Trash size={14} />
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {canManage && (
              <div className="mt-4 flex flex-wrap items-end gap-3">
                <div>
                  <label className="label-base">{t('admin.settings.country')}</label>
                  <input
                    className="input-base w-24"
                    value={newTier.country_code}
                    onChange={(e) =>
                      setNewTier((s) => ({ ...s, country_code: e.target.value.toUpperCase() }))
                    }
                  />
                </div>
                <div>
                  <label className="label-base">{t('admin.settings.minWeight')}</label>
                  <input
                    className="input-base w-24"
                    type="number"
                    value={newTier.min_weight_grams}
                    onChange={(e) =>
                      setNewTier((s) => ({ ...s, min_weight_grams: e.target.value }))
                    }
                  />
                </div>
                <div>
                  <label className="label-base">{t('admin.settings.maxWeight')}</label>
                  <input
                    className="input-base w-24"
                    type="number"
                    value={newTier.max_weight_grams}
                    onChange={(e) =>
                      setNewTier((s) => ({ ...s, max_weight_grams: e.target.value }))
                    }
                  />
                </div>
                <div>
                  <label className="label-base">{t('admin.settings.feeCny')}</label>
                  <input
                    className="input-base w-24"
                    type="number"
                    value={newTier.fee_cny}
                    onChange={(e) => setNewTier((s) => ({ ...s, fee_cny: e.target.value }))}
                  />
                </div>
                <Button variant="secondary" onClick={() => void addTier()}>
                  <Plus size={15} /> Add
                </Button>
              </div>
            )}
          </div>

          {/* Option rates */}
          <div className="card-surface p-6">
            <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">
              {t('admin.settings.optionRates')}
            </h3>
            <div className="overflow-x-auto rounded-lg border border-cobalt-50">
              <table className="w-full text-left text-[0.82rem]">
                <thead className="bg-wash/50">
                  <tr className="border-b border-cobalt-50">
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.optionKey')}
                    </th>
                    <th className="px-3 py-2 font-semibold text-ink-500">Label</th>
                    <th className="px-3 py-2 font-semibold text-ink-500">
                      {t('admin.settings.rateCny')}
                    </th>
                    {canManage && <th className="px-3 py-2" />}
                  </tr>
                </thead>
                <tbody>
                  {rates.map((rate) => (
                    <tr key={rate.id} className="border-b border-cobalt-50/60">
                      <td className="px-3 py-2 text-ink-700">{rate.option_key}</td>
                      <td className="px-3 py-2 text-ink-700">{rate.label ?? '—'}</td>
                      <td className="px-3 py-2 text-ink-700">{rate.rate_cny}</td>
                      {canManage && (
                        <td className="px-3 py-2">
                          <button
                            type="button"
                            onClick={() => void deleteRate(rate.id)}
                            className="text-ink-400 transition hover:text-[color:var(--color-danger)]"
                          >
                            <Trash size={14} />
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {canManage && (
              <div className="mt-4 flex flex-wrap items-end gap-3">
                <div>
                  <label className="label-base">{t('admin.settings.optionKey')}</label>
                  <input
                    className="input-base"
                    value={newRate.option_key}
                    onChange={(e) => setNewRate((s) => ({ ...s, option_key: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label-base">Label</label>
                  <input
                    className="input-base"
                    value={newRate.label}
                    onChange={(e) => setNewRate((s) => ({ ...s, label: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label-base">{t('admin.settings.rateCny')}</label>
                  <input
                    className="input-base w-32"
                    type="number"
                    value={newRate.rate_cny}
                    onChange={(e) => setNewRate((s) => ({ ...s, rate_cny: e.target.value }))}
                  />
                </div>
                <Button variant="secondary" onClick={() => void addRate()}>
                  <Plus size={15} /> Add
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
