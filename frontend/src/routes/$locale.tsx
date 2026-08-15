import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { ToastProvider } from '~/components/common/Toaster'
import { ButtonLink } from '~/components/common/ui'
import { Footer } from '~/components/layout/Footer'
import { Header } from '~/components/layout/Header'
import { AuthProvider } from '~/lib/auth'
import { CartProvider } from '~/lib/cart'
import { I18nProvider, useI18n } from '~/lib/i18n'
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
  return (
    <I18nProvider locale={valid}>
      <AuthProvider>
        <ToastProvider>
          <LocaleShell locale={valid} />
        </ToastProvider>
      </AuthProvider>
    </I18nProvider>
  )
}

function LocaleShell({ locale }: { locale: Locale }) {
  const { currency } = useI18n()
  return (
    <CartProvider locale={locale} currency={currency}>
      <WishlistProvider locale={locale} currency={currency}>
        <div className="flex min-h-screen flex-col">
          <Header />
          <main className="flex-1">
            <Outlet />
          </main>
          <Footer />
        </div>
      </WishlistProvider>
    </CartProvider>
  )
}
