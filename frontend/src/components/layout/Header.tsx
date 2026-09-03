/** Site header — nav, locale toggle, currency menu, wishlist/cart badges, account. */
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import {
  Bag,
  BellSimple,
  CaretDown,
  GlobeHemisphereWest,
  HeartStraight,
  List,
  Package,
  SignOut,
  UserCircle,
  Wrench,
  X,
} from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { SealMark } from '~/components/ornaments'
import { Button, ButtonLink } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { useWishlist } from '~/lib/wishlist'
import { cn, SUPPORTED_CURRENCIES, type Locale } from '~/lib/utils'

const CURRENCY_SYMBOLS: Record<string, string> = { USD: '$', EUR: '€', GBP: '£' }

export function Header() {
  const { t, locale, currency, setCurrency } = useI18n()
  const { user, logout, ready, token } = useAuth()
  const { count: cartCount } = useCart()
  const { ids: wishlistIds } = useWishlist()
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const [mobileOpen, setMobileOpen] = useState(false)
  const [unreadCount, setUnreadCount] = useState(0)

  useEffect(() => setMobileOpen(false), [pathname])

  useEffect(() => {
    if (!ready || !token) return
    let active = true
    const poll = () => {
      void api
        .getUnreadNotificationCount(token)
        .then((res) => active && setUnreadCount(res.count))
        .catch(() => {})
    }
    poll()
    const id = setInterval(poll, 30_000)
    return () => {
      active = false
      clearInterval(id)
    }
  }, [ready, token])

  const base = `/${locale}`
  const otherLocale: Locale = locale === 'en-US' ? 'zh-CN' : 'en-US'
  const otherPath = pathname.replace(/^\/[^/]+/, `/${otherLocale}`) || `/${otherLocale}`

  const navItems = [
    { label: t('nav.gallery'), to: `${base}/catalog` },
    { label: t('nav.artists'), to: `${base}/artists` },
    { label: t('nav.heritage'), to: `${base}/ceramicstory` },
    { label: t('nav.visit'), to: `${base}/engage` },
    { label: t('nav.travel'), to: `${base}/itinerary` },
  ]

  const isActive = (to: string) => pathname === to || pathname.startsWith(to + '/')

  return (
    <header className="sticky top-0 z-50 border-b border-cobalt-100/70 bg-white/90 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-shell items-center gap-6 px-4 sm:px-6">
        {/* brand */}
        <Link
          to={base as never}
          className="flex shrink-0 items-center gap-2.5"
          aria-label={t('common.brand')}
        >
          <SealMark size={34} />
          <span className="flex flex-col leading-none">
            <span className="text-[0.95rem] font-semibold tracking-tight text-ink-900">
              {t('common.brand')}
            </span>
            <span className="mt-1 text-[0.62rem] font-medium tracking-[0.16em] text-cobalt-600 uppercase">
              {t('common.brandSub')}
            </span>
          </span>
        </Link>

        {/* desktop nav */}
        <nav className="hidden items-center gap-1 lg:flex" aria-label="primary">
          {navItems.map((item) => (
            <Link
              key={item.to}
              to={item.to as never}
              className={cn(
                'rounded-md px-3 py-2 text-sm font-medium transition',
                isActive(item.to)
                  ? 'bg-cobalt-50 text-cobalt-700'
                  : 'text-ink-600 hover:bg-mist hover:text-ink-800',
              )}
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-1.5 sm:gap-2">
          {/* locale toggle */}
          <Link
            to={otherPath as never}
            aria-label="switch locale"
            className="hidden h-9 items-center gap-1.5 rounded-md px-2.5 text-[0.8rem] font-semibold text-ink-500 transition hover:bg-mist hover:text-cobalt-700 sm:flex"
          >
            <GlobeHemisphereWest size={16} />
            {otherLocale === 'zh-CN' ? '中文' : 'EN'}
          </Link>

          {/* currency */}
          <DropdownMenu.Root>
            <DropdownMenu.Trigger className="flex h-9 items-center gap-1 rounded-md px-2.5 text-[0.8rem] font-semibold text-ink-500 transition hover:bg-mist hover:text-cobalt-700">
              <span className="text-[0.9rem]">{CURRENCY_SYMBOLS[currency]}</span>
              {currency}
              <CaretDown size={11} className="text-ink-300" />
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content
                sideOffset={6}
                align="end"
                className="z-50 min-w-32 rounded-lg border border-cobalt-100 bg-white p-1 shadow-pop"
              >
                {SUPPORTED_CURRENCIES.map((c) => (
                  <DropdownMenu.Item
                    key={c}
                    onSelect={() => c !== currency && setCurrency(c)}
                    className={cn(
                      'flex cursor-pointer items-center justify-between rounded-md px-3 py-2 text-sm outline-none',
                      c === currency
                        ? 'bg-cobalt-50 text-cobalt-700'
                        : 'text-ink-600 data-[highlighted]:bg-mist',
                    )}
                  >
                    <span className="font-medium">{c}</span>
                    <span className="text-ink-300">{CURRENCY_SYMBOLS[c]}</span>
                  </DropdownMenu.Item>
                ))}
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>

          {/* notifications */}
          <Link
            to={`${base}/notifications` as never}
            aria-label={t('notif.title')}
            className="relative flex h-9 w-9 items-center justify-center rounded-md text-ink-500 transition hover:bg-mist hover:text-cobalt-700"
          >
            <BellSimple size={19} />
            {unreadCount > 0 && (
              <span className="absolute top-0.5 right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-cobalt-600 px-1 text-[0.62rem] font-bold text-white">
                {unreadCount}
              </span>
            )}
          </Link>

          {/* wishlist */}
          <Link
            to={`${base}/wishlist` as never}
            aria-label={t('nav.wishlist')}
            className="relative flex h-9 w-9 items-center justify-center rounded-md text-ink-500 transition hover:bg-mist hover:text-cobalt-700"
          >
            <HeartStraight size={19} />
            {wishlistIds.size > 0 && (
              <span className="absolute top-0.5 right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-cobalt-600 px-1 text-[0.62rem] font-bold text-white">
                {wishlistIds.size}
              </span>
            )}
          </Link>

          {/* cart */}
          <Link
            to={`${base}/cart` as never}
            aria-label={t('nav.cart')}
            className="relative flex h-9 w-9 items-center justify-center rounded-md text-ink-500 transition hover:bg-mist hover:text-cobalt-700"
          >
            <Bag size={19} />
            {cartCount > 0 && (
              <span className="absolute top-0.5 right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-cobalt-600 px-1 text-[0.62rem] font-bold text-white">
                {cartCount}
              </span>
            )}
          </Link>

          {/* account */}
          {!ready ? (
            <div className="h-9 w-20" />
          ) : user ? (
            <DropdownMenu.Root>
              <DropdownMenu.Trigger className="flex h-9 items-center gap-2 rounded-full border border-cobalt-100 py-1 pr-3 pl-1 transition hover:border-cobalt-200 hover:bg-cobalt-50/60">
                <span className="flex h-7 w-7 items-center justify-center rounded-full bg-cobalt-600 text-[0.78rem] font-bold text-white">
                  {(user.avatar_glyph ?? user.nickname).slice(0, 1)}
                </span>
                <span className="hidden max-w-24 truncate text-[0.82rem] font-medium text-ink-700 sm:block">
                  {user.nickname}
                </span>
                <CaretDown size={11} className="text-ink-300" />
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content
                  sideOffset={6}
                  align="end"
                  className="z-50 w-52 rounded-lg border border-cobalt-100 bg-white p-1 shadow-pop"
                >
                  <div className="border-b border-cobalt-50 px-3 py-2.5">
                    <p className="truncate text-sm font-semibold text-ink-800">{user.nickname}</p>
                    <p className="truncate text-[0.75rem] text-ink-400">{user.email}</p>
                  </div>
                  <MenuLink
                    icon={<Package size={15} />}
                    label={t('nav.orders')}
                    to={`${base}/orders`}
                  />
                  <MenuLink
                    icon={<Wrench size={15} />}
                    label={t('nav.itineraries')}
                    to={`${base}/itineraries`}
                  />
                  <MenuLink
                    icon={<UserCircle size={15} />}
                    label={t('nav.profile')}
                    to={`${base}/account`}
                  />
                  <DropdownMenu.Item
                    onSelect={() => {
                      logout()
                      void navigate({ to: base as never })
                    }}
                    className="mt-1 flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm text-[color:var(--color-danger)] outline-none data-[highlighted]:bg-[color:var(--color-danger-bg)]"
                  >
                    <SignOut size={15} />
                    {t('nav.logout')}
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>
          ) : (
            <ButtonLink to={`${base}/auth/login`} size="sm" className="hidden sm:inline-flex">
              {t('nav.login')}
            </ButtonLink>
          )}

          {/* mobile menu */}
          <button
            type="button"
            className="flex h-9 w-9 items-center justify-center rounded-md text-ink-600 hover:bg-mist lg:hidden"
            aria-label={t('nav.menu')}
            onClick={() => setMobileOpen((v) => !v)}
          >
            {mobileOpen ? <X size={20} /> : <List size={20} />}
          </button>
        </div>
      </div>

      {mobileOpen && (
        <nav
          className="border-t border-cobalt-100/70 bg-white px-4 pt-2 pb-4 lg:hidden"
          aria-label="mobile"
        >
          {navItems.map((item) => (
            <Link
              key={item.to}
              to={item.to as never}
              className={cn(
                'block rounded-md px-3 py-2.5 text-sm font-medium',
                isActive(item.to) ? 'bg-cobalt-50 text-cobalt-700' : 'text-ink-600',
              )}
            >
              {item.label}
            </Link>
          ))}
          <div className="mt-2 flex items-center gap-3 border-t border-cobalt-50 pt-3">
            <Link
              to={otherPath as never}
              className="flex items-center gap-1.5 px-3 py-2 text-sm font-semibold text-ink-600"
            >
              <GlobeHemisphereWest size={16} />
              {otherLocale === 'zh-CN' ? '中文' : 'English'}
            </Link>
            {!user && (
              <Button
                size="sm"
                onClick={() => void navigate({ to: `${base}/auth/login` as never })}
                className="flex-1"
              >
                {t('nav.login')}
              </Button>
            )}
          </div>
        </nav>
      )}
    </header>
  )
}

function MenuLink({ icon, label, to }: { icon: React.ReactNode; label: string; to: string }) {
  return (
    <DropdownMenu.Item asChild>
      <Link
        to={to as never}
        className="flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm text-ink-600 outline-none data-[highlighted]:bg-mist data-[highlighted]:text-ink-800"
      >
        {icon}
        {label}
      </Link>
    </DropdownMenu.Item>
  )
}
