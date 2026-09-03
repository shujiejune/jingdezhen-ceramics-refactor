import { createFileRoute, redirect } from '@tanstack/react-router'

import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'

/**
 * Admin index — redirects to the first module the user has permission for.
 * (Dashboard for most staff; content for editors; products for ecommerce
 * operators; itineraries for travel planners.)
 */
export const Route = createFileRoute('/$locale/admin/')({
  component: AdminIndex,
})

function AdminIndex() {
  const { locale } = useI18n()
  const { ready, hasPermission } = useAuth()
  const base = `/${locale}/admin`

  if (!ready) {
    return (
      <div className="flex items-center justify-center py-32">
        <div className="h-7 w-7 animate-spin text-cobalt-400" />
      </div>
    )
  }

  // redirect to the first permitted module
  const modules: Array<{ perm: Parameters<typeof hasPermission>[0]; to: string }> = [
    { perm: 'dashboard.view', to: `${base}/dashboard` },
    { perm: 'content.write', to: `${base}/content/stories` },
    { perm: 'product.read', to: `${base}/products` },
    { perm: 'order.read', to: `${base}/orders` },
    { perm: 'itinerary.read', to: `${base}/itineraries` },
    { perm: 'certificate.manage', to: `${base}/certificates` },
    { perm: 'users.manage', to: `${base}/users` },
    { perm: 'settings.manage', to: `${base}/settings` },
  ]

  const first = modules.find((m) => hasPermission(m.perm))
  if (first) {
    throw redirect({ href: first.to, replace: true })
  }

  return (
    <div className="mx-auto max-w-md py-32 text-center">
      <p className="text-[0.92rem] text-ink-500">No admin modules available for your role.</p>
    </div>
  )
}
