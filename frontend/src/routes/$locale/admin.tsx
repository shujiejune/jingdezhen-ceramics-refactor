import { createFileRoute } from '@tanstack/react-router'

import { AdminShell } from '~/components/admin/AdminShell'
import { noindexHead } from '~/lib/seo'

/**
 * Admin layout (PRD §3.4.1): staff guard runs inside AdminShell (which has
 * access to the auth context). Child routes render inside <Outlet />.
 * Admin pages skip the storefront Header/Footer (handled in $locale.tsx).
 * `noindex` cascades to all admin child routes (PRD §4.4).
 */
export const Route = createFileRoute('/$locale/admin')({
  head: () => noindexHead('Admin — Jingdezhen Ceramics'),
  component: AdminLayout,
})

function AdminLayout() {
  return <AdminShell />
}
