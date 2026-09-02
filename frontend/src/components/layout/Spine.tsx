/**
 * Spine — the vertical left rail used on horizontal-magazine pages
 * (landing, heritage). Brand seal + vertical wordmark on top, chapter
 * dots in the middle (with hover labels), utility cluster at the bottom
 * (locale, currency, wishlist, cart, account). Replaces the normal
 * Header/Footer chrome on those routes.
 */
import { Link, useLocation } from '@tanstack/react-router'
import { Bag, GlobeHemisphereWest, HeartStraight, SignIn } from '@phosphor-icons/react'

import { SealMark } from '~/components/ornaments'
import { useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { useWishlist } from '~/lib/wishlist'
import { cn, SUPPORTED_CURRENCIES, type Locale } from '~/lib/utils'
import type { CatalogKey } from '~/i18n/en-US'

export interface SpineChapter {
  /** static catalog key (mag.cover…) or a dynamic label (e.g. a dynasty year) */
  labelKey?: CatalogKey
  label?: string
}

export function Spine({
  chapters,
  activeIndex,
  onJump,
}: {
  chapters: SpineChapter[]
  activeIndex: number
  onJump: (i: number) => void
}) {
  const { t, locale, currency, setCurrency } = useI18n()
  const { user } = useAuth()
  const { count: cartCount } = useCart()
  const { ids: wishlistIds } = useWishlist()
  const pathname = useLocation({ select: (s) => s.pathname })
  const otherLocale: Locale = locale === 'en-US' ? 'zh-CN' : 'en-US'
  const otherPath = pathname.replace(/^\/[^/]+/, `/${otherLocale}`) || `/${otherLocale}`

  return (
    <aside className="absolute inset-y-0 left-0 z-40 flex w-14 flex-col items-center justify-between border-r border-cobalt-100/80 bg-white/92 py-4 backdrop-blur-md sm:w-16">
      {/* brand */}
      <div className="flex flex-col items-center gap-3">
        <Link
          to={`/${locale}` as never}
          aria-label={t('common.brand')}
          className="transition hover:scale-105"
        >
          <SealMark size={34} />
        </Link>
        <span
          className="hidden text-[0.6rem] font-semibold tracking-[0.3em] text-cobalt-600 uppercase sm:block"
          style={{ writingMode: 'vertical-rl' }}
        >
          {locale === 'zh-CN' ? '景德镇陶瓷' : 'Jingdezhen'}
        </span>
      </div>

      {/* chapter dots */}
      <nav aria-label={t('mag.tagline')} className="flex flex-col items-center gap-1.5">
        {chapters.map((ch, i) => {
          const label = ch.label ?? t(ch.labelKey ?? 'mag.cover')
          return (
            <button
              key={ch.labelKey ?? ch.label ?? i}
              type="button"
              onClick={() => onJump(i)}
              aria-label={label}
              aria-current={i === activeIndex}
              className="group relative flex h-8 w-8 items-center justify-center"
            >
              <span
                className={cn(
                  'pointer-events-none absolute left-1/2 -translate-x-1/2 rounded-sm border border-cobalt-100 bg-white px-2 py-1 text-[0.68rem] font-medium whitespace-nowrap text-ink-600 opacity-0 shadow-card transition group-hover:opacity-100',
                  i === activeIndex && 'hidden',
                )}
                style={{ writingMode: 'horizontal-tb', left: '130%' }}
              >
                {label}
              </span>
              <span
                className={cn(
                  'block rounded-full transition-all duration-300',
                  i === activeIndex
                    ? 'h-2.5 w-2.5 bg-cobalt-600 ring-4 ring-cobalt-100'
                    : 'h-1.5 w-1.5 bg-ink-300 group-hover:bg-cobalt-400',
                )}
              />
            </button>
          )
        })}
      </nav>

      {/* utilities */}
      <div className="flex flex-col items-center gap-1">
        <Link
          to={otherPath as never}
          aria-label="switch locale"
          className="flex h-8 w-8 items-center justify-center text-ink-500 transition hover:text-cobalt-700"
        >
          <GlobeHemisphereWest size={17} />
        </Link>
        <select
          aria-label={t('account.currencyPref')}
          value={currency}
          onChange={(e) => setCurrency(e.target.value as 'USD')}
          className="h-7 w-9 cursor-pointer rounded-sm border border-cobalt-100 bg-white text-[0.62rem] font-semibold text-cobalt-700"
        >
          {SUPPORTED_CURRENCIES.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <Link
          to={`/${locale}/wishlist` as never}
          aria-label={t('nav.wishlist')}
          className="relative flex h-8 w-8 items-center justify-center text-ink-500 transition hover:text-cobalt-700"
        >
          <HeartStraight size={17} />
          {wishlistIds.size > 0 && (
            <span className="absolute top-0.5 right-0 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-rose-500 px-1 text-[0.58rem] font-bold text-white">
              {wishlistIds.size}
            </span>
          )}
        </Link>
        <Link
          to={`/${locale}/cart` as never}
          aria-label={t('nav.cart')}
          className="relative flex h-8 w-8 items-center justify-center text-ink-500 transition hover:text-cobalt-700"
        >
          <Bag size={17} />
          {cartCount > 0 && (
            <span className="absolute top-0.5 right-0 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-cobalt-600 px-1 text-[0.58rem] font-bold text-white">
              {cartCount}
            </span>
          )}
        </Link>
        {user ? (
          <Link
            to={`/${locale}/account` as never}
            aria-label={t('nav.account')}
            className="mt-0.5 flex h-7 w-7 items-center justify-center rounded-full bg-cobalt-600 text-[0.68rem] font-bold text-white"
          >
            {(user.avatar_glyph ?? user.nickname).slice(0, 1)}
          </Link>
        ) : (
          <Link
            to={`/${locale}/auth/login` as never}
            aria-label={t('nav.login')}
            className="flex h-8 w-8 items-center justify-center text-ink-500 transition hover:text-cobalt-700"
          >
            <SignIn size={17} />
          </Link>
        )}
      </div>
    </aside>
  )
}
