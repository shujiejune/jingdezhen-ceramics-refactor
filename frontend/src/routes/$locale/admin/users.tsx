import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { AdminTable } from '~/components/admin/ContentTable'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { AdminUser, StaffRole } from '~/lib/types'
import type { Column } from '~/components/admin/ContentTable'

export const Route = createFileRoute('/$locale/admin/users')({
  component: UsersPage,
})

const STAFF_ROLES: StaffRole[] = [
  'super_admin',
  'content_editor',
  'travel_planner',
  'ecommerce_operator',
  'customer_service',
]

function UsersPage() {
  const { t } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [list, setList] = useState<AdminUser[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const canManage = hasPermission('users.manage')

  useEffect(() => {
    if (!ready || !token) return
    setList(null)
    api
      .adminListUsers(token)
      .then((res) => setList(res.data))
      .catch((e) => {
        setError(t(errorKey(e) as Parameters<typeof t>[0]))
        setList([])
      })
  }, [ready, token, t])

  const assignRole = async (userId: string, role: string) => {
    if (!token) return
    try {
      await api.adminAssignRole(token, userId, role)
      setList(
        (prev) =>
          prev?.map((u) =>
            u.id === userId
              ? { ...u, role: role as AdminUser['role'], roles: [role as StaffRole] }
              : u,
          ) ?? null,
      )
      push({ title: t('admin.users.roleAssigned'), kind: 'success' })
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  if (!ready || !token) return null

  const columns: Column<AdminUser>[] = [
    { header: t('admin.users.email'), cell: (row) => row.email },
    { header: t('admin.users.role'), cell: (row) => row.roles?.[0] ?? row.role },
    {
      header: t('admin.users.twoFa'),
      cell: (row) => (row.two_fa_enabled ? '✓' : '—'),
    },
    {
      header: t('admin.users.assignRole'),
      cell: (row) =>
        canManage ? (
          <select
            className="input-base text-[0.78rem]"
            value={row.roles?.[0] ?? row.role}
            onChange={(e) => void assignRole(row.id, e.target.value)}
          >
            {STAFF_ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        ) : (
          (row.roles?.[0] ?? row.role)
        ),
    },
  ]

  return (
    <div>
      <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.users.title')}</h2>
      {error && <p className="mt-4 text-[0.84rem] text-[color:var(--color-danger)]">{error}</p>}
      <div className="mt-6">
        <AdminTable data={list ?? []} loading={list === null} columns={columns} />
      </div>
    </div>
  )
}
