import { createFileRoute } from '@tanstack/react-router'

import { AdminShell } from '~/components/admin/AdminShell'

/**
 * Admin layout (PRD §3.4.1): staff guard runs inside AdminShell (which has
 * access to the auth context). Child routes render inside <Outlet />.
 * Admin pages skip the storefront Header/Footer (handled in $locale.tsx).
 */
export const Route = createFileRoute('/$locale/admin')({
  component: AdminLayout,
})

function AdminLayout() {
  return <AdminShell />
}
