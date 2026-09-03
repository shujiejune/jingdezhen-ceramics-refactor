/**
 * Admin chrome — sidebar + topbar for the client-rendered CMS (PRD §3.4.1).
 * The sidebar nav items are permission-keyed: a staff member only sees the
 * modules their role(s) permit. The shell renders inside the locale layout
 * (inheriting auth/i18n/toast providers) but skips the storefront
 * Header/Footer (admin owns its chrome, like magazine pages).
 */
import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'
import {
  ChartBar,
  FileText,
  Package,
  ShoppingBag,
  Airplane,
  Gear,
  Users,
  Scroll,
  Certificate as CertIcon,
  SignOut,
  House,
} from '@phosphor-icons/react'

import { SealMark } from '~/components/ornaments'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { Permission } from '~/lib/types'
import { cn } from '~/lib/utils'

interface NavItem {
  label: string
  to: string
  icon: React.ReactNode
  perm: Permission
}

export function AdminShell() {
  const { t, locale } = useI18n()
  const { user, ready, isStaff, hasPermission, logout } = useAuth()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const navigate = useNavigate()
  const base = `/${locale}/admin`

  // staff guard: redirect non-staff (or signed-out) users to the locale landing
  useEffect(() => {
    if (!ready) return
    if (!isStaff) {
      void navigate({ to: `/${locale}` as never, replace: true })
    }
  }, [ready, isStaff, locale, navigate])

  const navGroups: { title: string; items: NavItem[] }[] = [
    {
      title: t('admin.nav.overview'),
      items: [
        {
          label: t('admin.nav.dashboard'),
          to: `${base}/dashboard`,
          icon: <ChartBar size={18} weight="duotone" />,
          perm: 'dashboard.view',
        },
      ],
    },
    {
      title: t('admin.nav.content'),
      items: [
        {
          label: t('admin.nav.stories'),
          to: `${base}/content/stories`,
          icon: <FileText size={18} weight="duotone" />,
          perm: 'content.write',
        },
        {
          label: t('admin.nav.activities'),
          to: `${base}/content/activities`,
          icon: <FileText size={18} weight="duotone" />,
          perm: 'content.write',
        },
        {
          label: t('admin.nav.artists'),
          to: `${base}/content/artists`,
          icon: <FileText size={18} weight="duotone" />,
          perm: 'content.write',
        },
        {
          label: t('admin.nav.products'),
          to: `${base}/products`,
          icon: <Package size={18} weight="duotone" />,
          perm: 'product.read',
        },
        {
          label: t('admin.nav.media'),
          to: `${base}/media`,
          icon: <FileText size={18} weight="duotone" />,
          perm: 'content.write',
        },
      ],
    },
    {
      title: t('admin.nav.operations'),
      items: [
        {
          label: t('admin.nav.orders'),
          to: `${base}/orders`,
          icon: <ShoppingBag size={18} weight="duotone" />,
          perm: 'order.read',
        },
        {
          label: t('admin.nav.itineraries'),
          to: `${base}/itineraries`,
          icon: <Airplane size={18} weight="duotone" />,
          perm: 'itinerary.read',
        },
        {
          label: t('admin.nav.certificates'),
          to: `${base}/certificates`,
          icon: <CertIcon size={18} weight="duotone" />,
          perm: 'certificate.manage',
        },
      ],
    },
    {
      title: t('admin.nav.admin'),
      items: [
        {
          label: t('admin.nav.users'),
          to: `${base}/users`,
          icon: <Users size={18} weight="duotone" />,
          perm: 'users.manage',
        },
        {
          label: t('admin.nav.audit'),
          to: `${base}/audit`,
          icon: <Scroll size={18} weight="duotone" />,
          perm: 'settings.manage',
        },
        {
          label: t('admin.nav.settings'),
          to: `${base}/settings`,
          icon: <Gear size={18} weight="duotone" />,
          perm: 'settings.manage',
        },
      ],
    },
  ]

  if (!ready) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-wash">
        <div className="h-7 w-7 animate-spin text-cobalt-400" />
      </div>
    )
  }

  if (!isStaff) return null

  return (
    <div className="flex min-h-screen bg-wash">
      {/* sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 flex w-60 flex-col border-r border-cobalt-100 bg-white">
        <div className="flex items-center gap-2.5 px-5 py-5">
          <SealMark size={28} />
          <div className="flex flex-col leading-none">
            <span className="text-[0.88rem] font-semibold text-ink-900">{t('common.brand')}</span>
            <span className="mt-0.5 text-[0.58rem] font-medium tracking-[0.16em] text-cobalt-600 uppercase">
              {t('admin.title')}
            </span>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 py-2">
          {navGroups.map((group) => {
            const visibleItems = group.items.filter((item) => hasPermission(item.perm))
            if (visibleItems.length === 0) return null
            return (
              <div key={group.title} className="mb-5">
                <h3 className="px-3 py-1 text-[0.64rem] font-semibold tracking-[0.16em] text-ink-300 uppercase">
                  {group.title}
                </h3>
                <ul className="mt-1 flex flex-col gap-0.5">
                  {visibleItems.map((item) => {
                    const active = pathname.startsWith(item.to)
                    return (
                      <li key={item.to}>
                        <Link
                          to={item.to as never}
                          className={cn(
                            'flex items-center gap-2.5 rounded-lg px-3 py-2 text-[0.84rem] font-medium transition',
                            active
                              ? 'bg-cobalt-50 text-cobalt-700'
                              : 'text-ink-500 hover:bg-wash hover:text-ink-700',
                          )}
                        >
                          {item.icon}
                          {item.label}
                        </Link>
                      </li>
                    )
                  })}
                </ul>
              </div>
            )
          })}
        </nav>

        <div className="border-t border-cobalt-50 px-3 py-3">
          <Link
            to={`/${locale}` as never}
            className="flex items-center gap-2.5 rounded-lg px-3 py-2 text-[0.82rem] text-ink-500 transition hover:bg-wash hover:text-ink-700"
          >
            <House size={16} />
            {t('admin.nav.backToStore')}
          </Link>
          <button
            type="button"
            onClick={logout}
            className="mt-1 flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-[0.82rem] text-ink-500 transition hover:bg-wash hover:text-ink-700"
          >
            <SignOut size={16} />
            {t('nav.logout')}
          </button>
        </div>
      </aside>

      {/* main */}
      <div className="ml-60 flex-1">
        <header className="sticky top-0 z-20 flex items-center justify-between border-b border-cobalt-100 bg-white/90 px-6 py-3 backdrop-blur">
          <h1 className="text-[0.92rem] font-semibold text-ink-800">{t('admin.title')}</h1>
          <div className="flex items-center gap-3">
            <span className="text-[0.82rem] text-ink-500">{user?.email}</span>
            {user?.avatar_glyph && (
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-cobalt-100 text-[0.82rem] font-semibold text-cobalt-700">
                {user.avatar_glyph}
              </span>
            )}
          </div>
        </header>
        <main className="px-6 py-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
