import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { HeartStraight, ShoppingCart } from '@phosphor-icons/react'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { Badge, Button, ButtonLink, EmptyState, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { useWishlist } from '~/lib/wishlist'

/** Wishlist — protected (router.go gates /wishlist behind JWT). */
export const Route = createFileRoute('/$locale/wishlist')({
  component: WishlistPage,
})

function WishlistPage() {
  const { t, locale, price } = useI18n()
  const { ready, token } = useAuth()
  const { items, toggle } = useWishlist()
  const { add, busy } = useCart()
  const { push } = useToast()
  const navigate = useNavigate()

  if (!ready) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (!token) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('wishlist.title')}</h1>
        <p className="mt-3 text-ink-500">{t('checkout.signInBody')}</p>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/wishlist` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('nav.account')}</p>
      <h1 className="mt-2 text-display-sm text-ink-900">{t('wishlist.title')}</h1>
      <p className="mt-2 text-[0.92rem] text-ink-500">{t('wishlist.subtitle')}</p>

      {items.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={<HeartStraight size={40} weight="duotone" />}
            title={t('wishlist.empty')}
            body={t('wishlist.emptyBody')}
            action={<ButtonLink to={`/${locale}/catalog`}>{t('cart.emptyCta')}</ButtonLink>}
          />
        </div>
      ) : (
        <div className="mt-8 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {items.map((w) => (
            <article key={w.sku_id} className="card-surface group overflow-hidden">
              <button
                type="button"
                onClick={() => void navigate({ to: '/$locale/catalog/$slug', params: { locale, slug: w.product_slug } })}
                className="block w-full"
              >
                <div className="relative aspect-[4/4.4] bg-gradient-to-b from-wash to-porcelain">
                  <PorcelainFigure kind={w.figure_kind} seed={w.figure_seed} className="h-full w-full transition duration-500 group-hover:scale-[1.04]" />
                  {w.stock === 0 && (
                    <Badge tone="neutral" className="absolute top-3 left-3">
                      {t('product.outOfStock')}
                    </Badge>
                  )}
                </div>
              </button>
              <div className="p-4">
                <h3 className="truncate text-[0.92rem] font-semibold text-ink-800">{w.product_title}</h3>
                <p className="mt-0.5 truncate text-[0.78rem] text-ink-400">{w.artist_name}</p>
                <p className="mt-2 text-[0.95rem] font-semibold text-ink-900">
                  {price(w.price, w.price_currency)}
                </p>
                <div className="mt-4 flex gap-2">
                  <Button
                    size="sm"
                    className="flex-1"
                    disabled={w.stock === 0 || busy}
                    onClick={() =>
                      void add(w.sku_id).then(() =>
                        push({ title: t('toast.addedToCart', { title: w.product_title }) }),
                      )
                    }
                  >
                    <ShoppingCart size={14} weight="duotone" />
                    {t('wishlist.moveToCart')}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={t('common.remove')}
                    onClick={() =>
                      void toggle(w.sku_id).then((r) =>
                        push({ title: t(r === 'removed' ? 'toast.removedFromWishlist' : 'toast.addedToWishlist') }),
                      )
                    }
                  >
                    {t('common.remove')}
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}
