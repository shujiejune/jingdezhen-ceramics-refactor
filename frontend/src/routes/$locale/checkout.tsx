import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { SealCheck, SealWarning } from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'

import { api, ApiError } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { formatWeight } from '~/lib/money'
import type { Address, Order, ShippingQuote } from '~/lib/types'
import { SHIPPABLE_COUNTRIES } from '~/mocks/data'
import { useToast } from '~/components/common/Toaster'
import { Badge, Button, ButtonLink, FieldError, Spinner } from '~/components/common/ui'

/**
 * Checkout (signed-in only, PRD §3.2.3): address → reactive shipping
 * quote → payment (sandbox) → order. Consent checkboxes are enforced by
 * the API (ErrConsentRequired) as well as inline.
 */
export const Route = createFileRoute('/$locale/checkout')({
  component: CheckoutPage,
})

type Gateway = 'airwallex' | 'paypal'

function CheckoutPage() {
  const { t, locale, currency, price } = useI18n()
  const { ready, token, user } = useAuth()
  const { cart, refresh } = useCart()
  const navigate = useNavigate()
  const { push } = useToast()

  const [addresses, setAddresses] = useState<Address[] | null>(null)
  const [addressId, setAddressId] = useState<number | null>(null)
  const [showNewAddress, setShowNewAddress] = useState(false)
  const [quote, setQuote] = useState<ShippingQuote | null>(null)
  const [gateway, setGateway] = useState<Gateway>('airwallex')
  const [agreeToS, setAgreeToS] = useState(false)
  const [agreePrivacy, setAgreePrivacy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [placing, setPlacing] = useState(false)
  const [polling, setPolling] = useState(false)
  const [sandboxOrder, setSandboxOrder] = useState<Order | null>(null)

  const weight = useMemo(
    () => cart?.items.reduce((s, i) => s + i.weight_grams * i.qty, 0) ?? 0,
    [cart],
  )
  const address = addresses?.find((a) => a.id === addressId) ?? null
  const blocked = quote?.blocked_reason

  /* load address book */
  useEffect(() => {
    if (!token) return
    void api.getAddresses(token).then((list) => {
      setAddresses(list)
      setAddressId(list.find((a) => a.is_default)?.id ?? list[0]?.id ?? null)
      if (list.length === 0) setShowNewAddress(true)
    })
  }, [token])

  /* reactive shipping quote */
  useEffect(() => {
    if (!address || !cart || cart.items.length === 0) {
      setQuote(null)
      return
    }
    let cancelled = false
    api
      .getShippingQuote(address.country, weight, currency)
      .then((q) => !cancelled && setQuote(q))
      .catch(() => !cancelled && setQuote(null))
    return () => {
      cancelled = true
    }
  }, [address, cart, weight, currency])

  /* poll an order until status === 'paid', then navigate to the order detail
   * page with ?placed=1. Returns a cleanup function that cancels the timer. */
  const pollOrderStatus = (orderId: number, authToken: string) => {
    setPolling(true)
    const maxRetries = 15
    let retries = 0
    let timer: ReturnType<typeof setTimeout>
    const tick = () => {
      retries += 1
      api
        .getOrder(authToken, orderId, locale)
        .then((order) => {
          if (order.status === 'paid') {
            setPolling(false)
            push({ title: t('checkout.paymentConfirmed'), kind: 'success' })
            void refresh()
            void navigate({
              to: '/$locale/orders/$id',
              params: { locale, id: String(orderId) },
              search: { placed: 1 },
            })
          } else if (retries < maxRetries) {
            timer = setTimeout(tick, 2000)
          } else {
            setPolling(false)
            push({ title: t('checkout.pollTimeout'), kind: 'error' })
          }
        })
        .catch(() => {
          if (retries < maxRetries) {
            timer = setTimeout(tick, 2000)
          } else {
            setPolling(false)
            push({ title: t('checkout.pollTimeout'), kind: 'error' })
          }
        })
    }
    timer = setTimeout(tick, 2000)
    return () => clearTimeout(timer)
  }

  /* return-from-gateway: when the payment gateway redirects back with
   * ?order_id=...&paid=1, start polling for payment confirmation. */
  useEffect(() => {
    if (!token) return
    const params = new URLSearchParams(window.location.search)
    const orderIdParam = params.get('order_id')
    const paid = params.get('paid')
    if (orderIdParam && paid === '1') {
      const orderId = Number(orderIdParam)
      if (Number.isFinite(orderId)) {
        push({ title: t('checkout.polling'), kind: 'info' })
        return pollOrderStatus(orderId, token)
      }
    }
    return undefined
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  if (!ready || (token && (!cart || addresses === null))) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  /* ---- sign-in gate ---- */
  if (!token || !user) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('checkout.signInFirst')}</h1>
        <p className="mt-3 text-ink-500">{t('checkout.signInBody')}</p>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/checkout` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  if (cart && cart.items.length === 0) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('errors.cart_empty')}</h1>
        <ButtonLink to={`/${locale}/catalog`} className="mt-8">
          {t('cart.emptyCta')}
        </ButtonLink>
      </div>
    )
  }

  const placeOrder = async () => {
    setError(null)
    if (!agreeToS || !agreePrivacy) {
      setError(t('errors.consent_required'))
      return
    }
    if (!address) return
    setPlacing(true)
    try {
      const order = await api.checkout(token, {
        address_id: address.id,
        currency,
        gateway,
        locale,
        consent: true,
      })
      const hostedUrl = order.hosted_url
      if (hostedUrl && /^https?:\/\//.test(hostedUrl)) {
        /* live gateway: redirect away; the gateway will redirect back here
         * with ?order_id=...&paid=1, handled by the polling effect. */
        push({ title: t('checkout.redirecting'), kind: 'info' })
        window.location.href = hostedUrl
        return
      }
      /* mock / sandbox: fall through to the interstitial modal. */
      setSandboxOrder(order)
    } catch (e) {
      setError(t(errorKey(e) as Parameters<typeof t>[0]))
      if (e instanceof ApiError && e.is('cart_empty')) await refresh()
    } finally {
      setPlacing(false)
    }
  }

  const approveSandbox = async () => {
    if (!sandboxOrder) return
    setPlacing(true)
    try {
      await api.simulatePayment(token, sandboxOrder.id)
      await refresh()
      push({ title: t('checkout.polling'), kind: 'info' })
      pollOrderStatus(sandboxOrder.id, token)
    } finally {
      setPlacing(false)
      setSandboxOrder(null)
    }
  }

  const total =
    cart?.total !== undefined && !blocked && quote?.fee !== undefined
      ? cart.total + quote.fee
      : cart?.total

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('checkout.title')}</p>
      <h1 className="mt-2 text-display-sm text-ink-900">{t('checkout.title')}</h1>

      <div className="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_22rem]">
        <div className="flex flex-col gap-8">
          {/* ------------------------- delivery ------------------------- */}
          <section className="card-surface p-6">
            <h2 className="text-[1.02rem] font-semibold text-ink-900">
              1 · {t('checkout.delivery')}
            </h2>

            {addresses && addresses.length > 0 && (
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                {addresses.map((a) => (
                  <button
                    key={a.id}
                    type="button"
                    onClick={() => setAddressId(a.id)}
                    className={
                      'rounded-xl border p-4 text-left transition ' +
                      (a.id === addressId
                        ? 'border-cobalt-500 bg-cobalt-50/50 ring-1 ring-cobalt-200'
                        : 'border-ink-300/40 hover:border-cobalt-300')
                    }
                  >
                    <span className="flex items-center justify-between">
                      <span className="text-[0.88rem] font-semibold text-ink-800">
                        {a.recipient}
                      </span>
                      {a.is_default && <Badge tone="cobalt">{t('checkout.defaultBadge')}</Badge>}
                    </span>
                    <span className="mt-1.5 block text-[0.82rem] leading-relaxed text-ink-500">
                      {a.line1}
                      {a.line2 ? `, ${a.line2}` : ''}, {a.city} {a.postal_code},{' '}
                      {new Intl.DisplayNames([locale], { type: 'region' }).of(a.country)}
                    </span>
                  </button>
                ))}
              </div>
            )}

            {!showNewAddress ? (
              <button
                type="button"
                className="mt-4 text-[0.84rem] font-medium text-cobalt-600 hover:underline"
                onClick={() => setShowNewAddress(true)}
              >
                + {t('checkout.newAddress')}
              </button>
            ) : (
              <NewAddressForm
                onCreated={(a) => {
                  setAddresses((prev) => [...(prev ?? []), a])
                  setAddressId(a.id)
                  setShowNewAddress(false)
                }}
              />
            )}

            {address && (
              <p className="mt-5 rounded-lg bg-mist px-4 py-3 text-[0.85rem] text-ink-600">
                {t('checkout.shippingTo', {
                  country:
                    new Intl.DisplayNames([locale], { type: 'region' }).of(address.country) ?? '',
                })}{' '}
                · {t('checkout.weightLabel')}: {formatWeight(weight, locale)}
                {quote && !blocked && (
                  <>
                    {' '}
                    · {t('cart.estimatedShipping')}: <strong>{price(quote.fee)}</strong>
                  </>
                )}
              </p>
            )}

            {blocked && (
              <div className="mt-4 rounded-lg border border-[color:var(--color-warning)]/30 bg-[color:var(--color-warning-bg)] p-4">
                <p className="flex items-center gap-2 text-[0.86rem] font-semibold text-[color:var(--color-warning)]">
                  <SealWarning size={16} weight="duotone" />
                  {t('cart.overweightTitle')}
                </p>
                <p className="mt-2 text-[0.84rem] leading-relaxed text-ink-600">
                  {blocked === 'overweight' ? t('cart.overweightBody') : t('cart.unshippableBody')}
                </p>
                <a
                  href="mailto:hello@jdz-atelier.example"
                  className="mt-2 inline-block text-[0.84rem] font-medium text-cobalt-600 hover:underline"
                >
                  {t('cart.contactUs')}
                </a>
              </div>
            )}
          </section>

          {/* ------------------------- payment ------------------------- */}
          <section className="card-surface p-6">
            <h2 className="text-[1.02rem] font-semibold text-ink-900">
              2 · {t('checkout.payment')}
            </h2>
            <p className="mt-1">
              <Badge tone="warning">SANDBOX</Badge>
            </p>

            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              {(Object.keys({ airwallex: 1, paypal: 1 }) as Gateway[]).map((g) => (
                <button
                  key={g}
                  type="button"
                  onClick={() => setGateway(g)}
                  className={
                    'rounded-xl border p-4 text-left transition ' +
                    (gateway === g
                      ? 'border-cobalt-500 bg-cobalt-50/50 ring-1 ring-cobalt-200'
                      : 'border-ink-300/40 hover:border-cobalt-300')
                  }
                >
                  <span className="block text-[0.9rem] font-semibold text-ink-800">
                    {t(`checkout.gateway.${g}` as Parameters<typeof t>[0])}
                  </span>
                  <span className="mt-0.5 block text-[0.78rem] text-ink-400">
                    {t(`checkout.gateway.${g}Sub` as Parameters<typeof t>[0])}
                  </span>
                </button>
              ))}
            </div>

            {gateway === 'airwallex' && (
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <div className="sm:col-span-2">
                  <label htmlFor="ck-card-number" className="label-base">
                    {t('checkout.cardNumber')}
                  </label>
                  <input
                    id="ck-card-number"
                    className="input-base"
                    placeholder="4242 4242 4242 4242"
                    inputMode="numeric"
                  />
                </div>
                <div>
                  <label htmlFor="ck-card-expiry" className="label-base">
                    {t('checkout.cardExpiry')}
                  </label>
                  <input
                    id="ck-card-expiry"
                    className="input-base"
                    placeholder="12 / 28"
                    inputMode="numeric"
                  />
                </div>
                <div>
                  <label htmlFor="ck-card-cvc" className="label-base">
                    {t('checkout.cardCvc')}
                  </label>
                  <input
                    id="ck-card-cvc"
                    className="input-base"
                    placeholder="123"
                    inputMode="numeric"
                  />
                </div>
                <div className="sm:col-span-2">
                  <label htmlFor="ck-card-name" className="label-base">
                    {t('checkout.cardName')}
                  </label>
                  <input id="ck-card-name" className="input-base" placeholder={user.nickname} />
                </div>
              </div>
            )}
          </section>
        </div>

        {/* ------------------------------ summary ------------------------------ */}
        <aside className="card-surface sticky top-24 p-6">
          <h2 className="text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">
            {t('checkout.summary')}
          </h2>

          {cart && (
            <ul className="mt-4 flex flex-col gap-3 border-b border-cobalt-100 pb-4">
              {cart.items.map((i) => (
                <li key={i.sku_id} className="flex justify-between gap-3 text-[0.84rem]">
                  <span className="min-w-0 truncate text-ink-600">
                    {i.product_title} <span className="text-ink-300">× {i.qty}</span>
                  </span>
                  <span className="shrink-0 font-medium text-ink-800">
                    {price(i.line_total ?? i.line_total_cny)}
                  </span>
                </li>
              ))}
            </ul>
          )}

          <dl className="mt-4 flex flex-col gap-2.5 text-[0.9rem]">
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('cart.subtotal')}</dt>
              <dd className="font-medium text-ink-800">{price(cart?.total, cart?.currency)}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">{t('cart.estimatedShipping')}</dt>
              <dd className="font-medium text-ink-800">{blocked ? '—' : price(quote?.fee)}</dd>
            </div>
            <div className="mt-1 flex justify-between border-t border-cobalt-100 pt-3 text-[1.05rem]">
              <dt className="font-semibold text-ink-800">{t('cart.total')}</dt>
              <dd className="font-semibold text-ink-900">{price(total)}</dd>
            </div>
          </dl>

          {/* consent */}
          <div className="mt-5 flex flex-col gap-2.5">
            <label className="flex cursor-pointer items-start gap-2.5 text-[0.82rem] leading-snug text-ink-600">
              <input
                type="checkbox"
                checked={agreeToS}
                onChange={(e) => setAgreeToS(e.target.checked)}
                className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]"
              />
              {t('checkout.tos')}
            </label>
            <label className="flex cursor-pointer items-start gap-2.5 text-[0.82rem] leading-snug text-ink-600">
              <input
                type="checkbox"
                checked={agreePrivacy}
                onChange={(e) => setAgreePrivacy(e.target.checked)}
                className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]"
              />
              {t('checkout.privacy')}
            </label>
          </div>

          {error && (
            <p className="mt-4 rounded-lg bg-[color:var(--color-danger-bg)] px-3.5 py-2.5 text-[0.84rem] text-[color:var(--color-danger)]">
              {error}
            </p>
          )}

          <Button
            size="lg"
            className="mt-5 w-full"
            loading={placing}
            disabled={Boolean(blocked) || !address}
            onClick={() => void placeOrder()}
          >
            <SealCheck size={17} weight="duotone" />
            {t('checkout.placeOrder', { amount: price(total) })}
          </Button>

          <p className="mt-4 text-[0.74rem] leading-relaxed text-ink-300">
            {t('checkout.customs')}
          </p>
        </aside>
      </div>

      {/* ------------------------- sandbox interstitial ------------------------- */}
      {sandboxOrder && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-ink-900/45 p-4 backdrop-blur-sm">
          <div className="card-surface w-full max-w-md p-8 text-center shadow-pop">
            <Badge tone="warning">{t('checkout.sandboxTitle')}</Badge>
            <h2 className="mt-4 text-[1.25rem] font-semibold text-ink-900">
              {t('checkout.placeOrder', { amount: price(sandboxOrder.total_minor) })}
            </h2>
            <p className="mt-2.5 text-[0.88rem] leading-relaxed text-ink-500">
              {t('checkout.sandboxBody')}
            </p>
            <p className="mt-3 text-[0.78rem] text-ink-300">
              {t('orders.orderN', { id: sandboxOrder.id })} · {gateway}
            </p>
            <div className="mt-7 flex gap-3">
              <Button
                variant="secondary"
                className="flex-1"
                onClick={() => {
                  setSandboxOrder(null)
                  void navigate({
                    to: '/$locale/orders/$id',
                    params: { locale, id: String(sandboxOrder.id) },
                  })
                }}
              >
                {t('checkout.declinePayment')}
              </Button>
              <Button className="flex-1" loading={placing} onClick={() => void approveSandbox()}>
                <SealCheck size={16} weight="duotone" />
                {t('checkout.approvePayment')}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* ------------------------- payment polling overlay ------------------------- */}
      {polling && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-ink-900/45 p-4 backdrop-blur-sm">
          <div className="card-surface w-full max-w-md p-8 text-center shadow-pop">
            <Spinner className="mx-auto h-7 w-7 text-cobalt-400" />
            <p className="mt-4 text-[0.9rem] leading-relaxed text-ink-600">
              {t('checkout.polling')}
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

/* ------------------------------------------------------------------ */

function NewAddressForm({ onCreated }: { onCreated: (a: Address) => void }) {
  const { t, locale } = useI18n()
  const { token } = useAuth()
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState({
    recipient: '',
    line1: '',
    line2: '',
    city: '',
    region: '',
    postal_code: '',
    country: 'US',
    phone: '',
  })
  const [formError, setFormError] = useState<string | null>(null)

  const set =
    (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
      setForm((f) => ({ ...f, [k]: e.target.value }))

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    setSaving(true)
    try {
      const a = await api.createAddress(token!, form)
      onCreated(a)
    } catch {
      setFormError(t('errors.validation_failed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form
      onSubmit={submit}
      className="mt-5 grid gap-4 rounded-xl border border-cobalt-100 bg-wash/60 p-5 sm:grid-cols-2"
    >
      {(
        [
          ['recipient', t('checkout.addressName'), true],
          ['line1', t('checkout.addressLine1'), true],
          ['line2', `${t('checkout.addressLine2')} (${t('common.optional')})`, false],
          ['city', t('checkout.city'), true],
          ['region', `${t('checkout.region')} (${t('common.optional')})`, false],
          ['postal_code', t('checkout.postal'), true],
        ] as const
      ).map(([key, label, required]) => (
        <div key={key}>
          <label htmlFor={`af-${key}`} className="label-base">
            {label}
          </label>
          <input
            id={`af-${key}`}
            className="input-base"
            value={form[key]}
            onChange={set(key)}
            required={required}
          />
        </div>
      ))}
      <div>
        <label htmlFor="af-country" className="label-base">
          {t('checkout.country')}
        </label>
        <select
          id="af-country"
          className="input-base"
          value={form.country}
          onChange={set('country')}
        >
          {SHIPPABLE_COUNTRIES.map((c) => (
            <option key={c} value={c}>
              {new Intl.DisplayNames([locale], { type: 'region' }).of(c)}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor="af-phone" className="label-base">
          {t('checkout.phone')}
        </label>
        <input
          id="af-phone"
          className="input-base"
          value={form.phone}
          onChange={set('phone')}
          required
        />
      </div>
      <div className="sm:col-span-2 flex items-center gap-3">
        <Button type="submit" loading={saving}>
          {t('checkout.saveAddress')}
        </Button>
        {formError && <FieldError>{formError}</FieldError>}
      </div>
    </form>
  )
}
