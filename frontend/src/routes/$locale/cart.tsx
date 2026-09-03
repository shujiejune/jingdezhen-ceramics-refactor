import { createFileRoute } from '@tanstack/react-router'
import { Bag, SealWarning } from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { WaveBand } from '~/components/ornaments'
import { Button, ButtonLink, EmptyState, QuantityStepper, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { formatWeight } from '~/lib/money'
import { CONTACT, SHIPPABLE_COUNTRIES } from '~/mocks/data'
import type { ShippingQuote } from '~/lib/types'
import { cn } from '~/lib/utils'

/**
 * Cart — qty updates, bulk remove, and a shipping *preview* keyed by the
 * destination country. All totals (incl. FX) come from the API (TDD §7);
 * the quote endpoint does the tier math.
 */
export const Route = createFileRoute('/$locale/cart')({
  component: CartPage,
})

function CartPage() {
  const { t, locale, currency, price } = useI18n()
  const { cart, busy, setQty, remove, bulkRemove } = useCart()
  const { push } = useToast()
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [country, setCountry] = useState('US')
  const [quote, setQuote] = useState<ShippingQuote | null>(null)
  const [quoteLoading, setQuoteLoading] = useState(false)

  const weight = useMemo(
    () => cart?.items.reduce((s, i) => s + i.weight_grams * i.qty, 0) ?? 0,
    [cart],
  )

  // reactive shipping preview (PRD §3.2.3: updates when cart/address changes)
  useEffect(() => {
    if (!cart || cart.items.length === 0) {
      setQuote(null)
      return
    }
    let cancelled = false
    setQuoteLoading(true)
    api
      .getShippingQuote(country, weight, currency)
      .then((q) => !cancelled && setQuote(q))
      .catch(() => !cancelled && setQuote(null))
      .finally(() => !cancelled && setQuoteLoading(false))
    return () => {
      cancelled = true
    }
  }, [cart, country, weight, currency])

  if (!cart) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (cart.items.length === 0) {
    return (
      <div className="mx-auto max-w-shell px-4 pt-16 sm:px-6">
        <EmptyState
          icon={<Bag size={40} weight="duotone" />}
          title={t('cart.empty')}
          body={t('cart.emptyBody')}
          action={<ButtonLink to={`/${locale}/catalog`}>{t('cart.emptyCta')}</ButtonLink>}
        />
      </div>
    )
  }

  const toggleSel = (skuId: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(skuId)) next.delete(skuId)
      else next.add(skuId)
      return next
    })
  }

  const totalWithShipping =
    quote && !quote.blocked_reason && quote.fee !== undefined && cart.total !== undefined
      ? cart.total + quote.fee
      : cart.total

  const blocked = quote?.blocked_reason

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <div className="flex items-end justify-between">
        <div>
          <p className="eyebrow">{t('nav.cart')}</p>
          <h1 className="mt-2 text-display-sm text-ink-900">{t('cart.title')}</h1>
        </div>
        <p className="text-[0.85rem] text-ink-400">{t('cart.items', { count: cart.item_count })}</p>
      </div>

      <div className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_22rem]">
        {/* ------------------------------ lines ------------------------------ */}
        <div className="flex flex-col gap-4">
          {cart.items.map((item) => (
            <div key={item.sku_id} className="card-surface flex items-center gap-4 p-4 sm:gap-5">
              <input
                type="checkbox"
                checked={selected.has(item.sku_id)}
                onChange={() => toggleSel(item.sku_id)}
                aria-label={t('cart.select')}
                className="h-4 w-4 shrink-0 accent-[var(--cobalt-600)]"
              />
              <div className="h-20 w-20 shrink-0 overflow-hidden rounded-lg border border-cobalt-100 bg-wash">
                <PorcelainFigure
                  kind={item.figure_kind}
                  seed={item.figure_seed}
                  className="h-full w-full"
                />
              </div>
              <div className="min-w-0 flex-1">
                <a
                  href={`/${locale}/catalog/${item.product_slug}`}
                  className="block truncate text-[0.92rem] font-semibold text-ink-800 hover:text-cobalt-700"
                >
                  {item.product_title}
                </a>
                <p className="mt-0.5 truncate text-[0.78rem] text-ink-400">
                  {[item.attributes?.size, item.artist_name].filter(Boolean).join(' · ')}
                </p>
                <p className="mt-1 text-[0.75rem] text-ink-300">
                  {t('product.weight')}: {formatWeight(item.weight_grams * item.qty, locale)}
                </p>
              </div>
              <div className="flex flex-col items-end gap-2.5 sm:flex-row sm:items-center sm:gap-5">
                <QuantityStepper
                  value={item.qty}
                  max={item.stock}
                  disabled={busy}
                  onChange={(q) => {
                    void setQty(item.sku_id, q).then(() => push({ title: t('toast.cartQty') }))
                  }}
                />
                <p className="w-24 text-right text-[0.92rem] font-semibold text-ink-900">
                  {price(item.line_total ?? item.line_total_cny)}
                </p>
                <button
                  type="button"
                  aria-label={t('common.remove')}
                  className="text-[0.75rem] text-ink-300 transition hover:text-[color:var(--color-danger)]"
                  onClick={() => void remove(item.sku_id)}
                >
                  {t('common.remove')}
                </button>
              </div>
            </div>
          ))}

          {selected.size > 0 && (
            <div className="flex justify-end">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  void bulkRemove([...selected]).then(() => {
                    setSelected(new Set())
                    push({ title: t('toast.cartQty') })
                  })
                }}
              >
                {t('cart.removeSelected')} ({selected.size})
              </Button>
            </div>
          )}
        </div>

        {/* ------------------------------ summary ------------------------------ */}
        <aside className="card-surface sticky top-24 p-6">
          <h2 className="text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">
            {t('checkout.summary')}
          </h2>

          <label htmlFor="cart-country" className="label-base mt-5">
            {t('checkout.country')}
          </label>
          <select
            id="cart-country"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
            className="input-base"
          >
            {SHIPPABLE_COUNTRIES.map((c) => (
              <option key={c} value={c}>
                {new Intl.DisplayNames([locale], { type: 'region' }).of(c)}
              </option>
            ))}
          </select>

          <dl className="mt-6 flex flex-col gap-3 text-[0.9rem]">
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('cart.subtotal')}</dt>
              <dd className="font-medium text-ink-800">{price(cart.total, cart.currency)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('cart.estimatedShipping')}</dt>
              <dd className="font-medium text-ink-800">
                {quoteLoading ? (
                  <Spinner className="h-4 w-4 text-cobalt-400" />
                ) : blocked ? (
                  '—'
                ) : (
                  price(quote?.fee)
                )}
              </dd>
            </div>
            <div className="flex justify-between border-t border-cobalt-100 pt-3">
              <dt className="text-ink-500">{t('cart.totalWeight')}</dt>
              <dd className="font-medium text-ink-800">{formatWeight(weight, locale)}</dd>
            </div>
            <div className="flex justify-between border-t border-cobalt-100 pt-3 text-[1.05rem]">
              <dt className="font-semibold text-ink-800">{t('cart.total')}</dt>
              <dd className="font-semibold text-ink-900">{price(totalWithShipping)}</dd>
            </div>
          </dl>

          {blocked && (
            <div className="mt-5 rounded-lg border border-[color:var(--color-warning)]/30 bg-[color:var(--color-warning-bg)] p-4">
              <p className="flex items-center gap-2 text-[0.84rem] font-semibold text-[color:var(--color-warning)]">
                <SealWarning size={16} weight="duotone" />
                {t('cart.overweightTitle')}
              </p>
              <p className="mt-2 text-[0.82rem] leading-relaxed text-ink-600">
                {blocked === 'overweight' ? t('cart.overweightBody') : t('cart.unshippableBody')}
              </p>
              <p className="mt-2.5 flex flex-wrap gap-x-4 gap-y-1 text-[0.82rem] font-medium text-cobalt-600">
                <a href={`mailto:${CONTACT.email}`} className="hover:underline">
                  {CONTACT.email}
                </a>
                <a href={`tel:${CONTACT.phone.replace(/\s/g, '')}`} className="hover:underline">
                  {CONTACT.phone}
                </a>
              </p>
            </div>
          )}

          <ButtonLink
            to={`/${locale}/checkout`}
            size="lg"
            className={cn('mt-6 w-full', blocked && 'pointer-events-none opacity-50')}
          >
            {t('cart.proceed')}
          </ButtonLink>
          <ButtonLink to={`/${locale}/catalog`} variant="ghost" size="sm" className="mt-3 w-full">
            {t('cart.continueShopping')}
          </ButtonLink>
          <p className="mt-4 text-[0.72rem] leading-relaxed text-ink-300">{t('cart.fxNote')}</p>
        </aside>
      </div>

      <div className="mt-16 flex justify-center">
        <WaveBand width={200} />
      </div>
    </div>
  )
}
