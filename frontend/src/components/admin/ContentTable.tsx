/**
 * Reusable table component for admin list views (PRD §3.4.1).
 * Handles status badges and row click → detail navigation.
 */
import { Link } from '@tanstack/react-router'

import { Badge, Spinner } from '~/components/common/ui'
import { useI18n } from '~/lib/i18n'
import type { ContentStatus } from '~/lib/types'
import { cn } from '~/lib/utils'

const statusTone: Record<ContentStatus, 'cobalt' | 'success' | 'warning' | 'danger' | 'neutral'> = {
  draft: 'neutral',
  in_review: 'warning',
  published: 'success',
  rejected: 'danger',
}

export interface Column<T> {
  header: string
  cell: (row: T) => React.ReactNode
}

export interface AdminTableProps<T> {
  data: T[]
  loading?: boolean
  columns: Column<T>[]
  emptyLabel?: string
}

export function AdminTable<T extends { id: number | string }>({
  data,
  loading,
  columns,
  emptyLabel,
}: AdminTableProps<T>) {
  const { t } = useI18n()

  if (loading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="h-6 w-6 text-cobalt-400" />
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="py-16 text-center text-[0.88rem] text-ink-400">
        {emptyLabel ?? t('admin.common.empty')}
      </div>
    )
  }

  return (
    <div className="overflow-x-auto rounded-xl border border-cobalt-100 bg-white">
      <table className="w-full text-left text-[0.84rem]">
        <thead>
          <tr className="border-b border-cobalt-50 bg-wash/50">
            {columns.map((col, i) => (
              <th key={i} className="px-4 py-2.5 font-semibold text-ink-500">
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, ri) => (
            <tr
              key={row.id}
              className={cn(
                'border-b border-cobalt-50/60 transition hover:bg-wash/30',
                ri % 2 === 1 && 'bg-wash/10',
              )}
            >
              {columns.map((col, ci) => (
                <td key={ci} className="px-4 py-3 text-ink-700">
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** Status badge for content workflow states. */
export function StatusBadge({ status }: { status: ContentStatus }) {
  const { t } = useI18n()
  return (
    <Badge tone={statusTone[status]}>
      {t(`admin.status.${status}` as Parameters<typeof t>[0])}
    </Badge>
  )
}

/** Link cell for navigating to a detail row. */
export function DetailLinkCell({ to, label }: { to: string; label: string }) {
  return (
    <Link
      to={to as never}
      className="font-medium text-cobalt-600 transition hover:text-cobalt-700 hover:underline"
    >
      {label}
    </Link>
  )
}
