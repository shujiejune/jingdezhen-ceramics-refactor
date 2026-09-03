import { Outlet, createFileRoute, redirect, useRouterState } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { useEffect } from 'react'

import { ConsentBanner } from '~/components/common/ConsentBanner'
import { ToastProvider } from '~/components/common/Toaster'
import { ButtonLink } from '~/components/common/ui'
import { ChatWidget } from '~/components/chat/ChatWidget'
import { Footer } from '~/components/layout/Footer'
import { Header } from '~/components/layout/Header'
import { AuthProvider } from '~/lib/auth'
import { CartProvider } from '~/lib/cart'
import { ChatProvider } from '~/lib/chat'
import { ConsentProvider } from '~/lib/consent'
import { trackPageview } from '~/lib/analytics'
import { I18nProvider, useI18n } from '~/lib/i18n'
import { RealtimeProvider } from '~/lib/realtime'
import { isLocale, type Locale } from '~/lib/utils'
import { WishlistProvider } from '~/lib/wishlist'

/**
 * Locale layout (TDD §6): validates the [locale] segment against
 * models.SupportedLocales and redirects out-of-range locales to en-US
 * with the same sub-path. Wraps everything in the i18n / auth / cart /
 * wishlist / toast providers and the site chrome.
 */
export const Route = createFileRoute('/$locale')({
  beforeLoad: ({ params, location }) => {
    if (!isLocale(params.locale)) {
      const rest = location.pathname.replace(/^\/[^/]+/, '')
      throw redirect({ href: `/en-US${rest}` })
    }
    return { locale: params.locale }
  },
  component: LocaleLayout,
  notFoundComponent: LocaleNotFound,
})

function LocaleNotFound() {
  const { locale, t } = useI18n()
  return (
    <div className="mx-auto flex max-w-shell flex-col items-center px-6 py-32 text-center">
      <p className="eyebrow">404</p>
      <h1 className="mt-3 text-display-sm text-ink-900">{t('errors.not_found')}</h1>
      <ButtonLink to={`/${locale}`} className="mt-8">
        {t('landing.ctaGallery')}
      </ButtonLink>
    </div>
  )
}

function LocaleLayout() {
  const { locale } = Route.useParams()
  const valid: Locale = isLocale(locale) ? locale : 'en-US'
  const { queryClient } = Route.useRouteContext()
  return (
    <QueryClientProvider client={queryClient}>
      <I18nProvider locale={valid}>
        <AuthProvider>
          <RealtimeProvider>
            <ToastProvider>
              <LocaleShell locale={valid} />
            </ToastProvider>
          </RealtimeProvider>
        </AuthProvider>
      </I18nProvider>
    </QueryClientProvider>
  )
}

function LocaleShell({ locale }: { locale: Locale }) {
  const { currency } = useI18n()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  // horizontal-magazine pages (landing + heritage index; detail articles
  // stay vertical) own their chrome: spine + panels, no header/footer.
  // admin pages also own their chrome (sidebar + topbar).
  const isMagazine = /^\/[^/]+\/?$/.test(pathname) || /^\/[^/]+\/ceramicstory\/?$/.test(pathname)
  const isAdmin = /^\/[^/]+\/admin(\/|$)/.test(pathname)

  // analytics: fire a pageview on every route change (PRD §4.3, TDD §4.3)
  useEffect(() => {
    trackPageview(pathname, locale)
  }, [pathname, locale])

  return (
    <ConsentProvider>
      <CartProvider locale={locale} currency={currency}>
        <WishlistProvider locale={locale} currency={currency}>
          <ChatProvider>
            {isMagazine || isAdmin ? (
              <main className="flex-1">
                <Outlet />
              </main>
            ) : (
              <div className="flex min-h-screen flex-col">
                <Header />
                <main className="flex-1">
                  <Outlet />
                </main>
                <Footer />
                <ConsentBanner />
                {/* site-wide chat (PRD §3.3.1); hidden on admin (own console) */}
                <ChatWidget />
              </div>
            )}
          </ChatProvider>
        </WishlistProvider>
      </CartProvider>
    </ConsentProvider>
  )
}
