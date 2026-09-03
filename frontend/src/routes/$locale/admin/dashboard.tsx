import { createFileRoute } from '@tanstack/react-router'
import { Download } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { DashboardFunnel, DashboardSales, DashboardTraffic } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/dashboard')({
  component: DashboardPage,
})

const RANGES = ['day', 'week', 'month', 'year'] as const
type Range = (typeof RANGES)[number]

function DashboardPage() {
  const { t } = useI18n()
  const { ready, token } = useAuth()
  const { push } = useToast()
  const [range, setRange] = useState<Range>('week')
  const [traffic, setTraffic] = useState<DashboardTraffic | null>(null)
  const [sales, setSales] = useState<DashboardSales | null>(null)
  const [funnel, setFunnel] = useState<DashboardFunnel | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    setTraffic(null)
    setSales(null)
    setFunnel(null)
    setError(null)
    const params = { range }
    Promise.all([
      api.adminDashboardTraffic(token, params),
      api.adminDashboardSales(token, params),
      api.adminDashboardFunnel(token, params),
    ])
      .then(([tr, sa, fu]) => {
        setTraffic(tr)
        setSales(sa)
        setFunnel(fu)
      })
      .catch((e) => setError(t(errorKey(e) as Parameters<typeof t>[0])))
  }, [ready, token, range, t])

  if (!ready || !token) return null

  const exportCsv = (label: string) => {
    push({ title: `${label} CSV`, kind: 'info' })
  }

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.dashboard.title')}</h2>
        <div className="flex gap-1">
          {RANGES.map((r) => (
            <Button
              key={r}
              variant={r === range ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setRange(r)}
            >
              {t(
                `admin.dashboard.range${r.charAt(0).toUpperCase() + r.slice(1)}` as Parameters<
                  typeof t
                >[0],
              )}
            </Button>
          ))}
        </div>
      </div>

      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}

      {traffic === null || sales === null || funnel === null ? (
        <div className="flex justify-center py-16">
          <Spinner className="h-6 w-6 text-cobalt-400" />
        </div>
      ) : (
        <div className="mt-6 flex flex-col gap-6">
          {/* Traffic */}
          <div className="card-surface p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-[0.88rem] font-semibold text-ink-700">
                {t('admin.dashboard.traffic')}
              </h3>
              <Button variant="secondary" size="sm" onClick={() => exportCsv('Traffic')}>
                <Download size={13} /> {t('admin.dashboard.exportCsv')}
              </Button>
            </div>
            <div className="flex flex-col gap-2">
              {traffic.data.length === 0 ? (
                <p className="text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
              ) : (
                traffic.data.slice(0, 7).map((d) => (
                  <div key={d.date} className="flex items-center gap-2 text-[0.8rem]">
                    <span className="w-24 text-ink-500">{d.date}</span>
                    <div className="flex-1 rounded bg-wash/30">
                      <div
                        className="h-4 rounded bg-cobalt-300"
                        style={{ width: `${Math.min(100, d.pageviews / 10)}%` }}
                      />
                    </div>
                    <span className="w-16 text-ink-600">{d.pageviews}</span>
                    <span className="w-16 text-ink-500">{d.unique_visitors}</span>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Sales */}
          <div className="card-surface p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-[0.88rem] font-semibold text-ink-700">
                {t('admin.dashboard.sales')}
              </h3>
              <Button variant="secondary" size="sm" onClick={() => exportCsv('Sales')}>
                <Download size={13} /> {t('admin.dashboard.exportCsv')}
              </Button>
            </div>
            <div className="flex flex-col gap-2">
              {sales.data.length === 0 ? (
                <p className="text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
              ) : (
                sales.data.slice(0, 7).map((d) => (
                  <div key={d.date} className="flex items-center gap-2 text-[0.8rem]">
                    <span className="w-24 text-ink-500">{d.date}</span>
                    <div className="flex-1 rounded bg-wash/30">
                      <div
                        className="h-4 rounded bg-success-300"
                        style={{ width: `${Math.min(100, d.orders)}%` }}
                      />
                    </div>
                    <span className="w-16 text-ink-600">{d.orders}</span>
                    <span className="w-24 text-ink-500">¥{(d.revenue_cny / 100).toFixed(0)}</span>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Funnel */}
          <div className="card-surface p-6">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-[0.88rem] font-semibold text-ink-700">
                {t('admin.dashboard.funnel')}
              </h3>
              <Button variant="secondary" size="sm" onClick={() => exportCsv('Funnel')}>
                <Download size={13} /> {t('admin.dashboard.exportCsv')}
              </Button>
            </div>
            <div className="flex flex-col gap-2">
              {funnel.steps.length === 0 ? (
                <p className="text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
              ) : (
                funnel.steps.map((s) => (
                  <div key={s.step} className="flex items-center gap-2 text-[0.8rem]">
                    <span className="w-32 text-ink-500">{s.label}</span>
                    <div className="flex-1 rounded bg-wash/30">
                      <div
                        className="h-4 rounded bg-cobalt-400"
                        style={{ width: `${Math.min(100, s.rate * 100)}%` }}
                      />
                    </div>
                    <span className="w-16 text-ink-600">{s.count}</span>
                    <span className="w-12 text-ink-500">{(s.rate * 100).toFixed(0)}%</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
