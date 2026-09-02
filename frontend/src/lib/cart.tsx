/**
 * Cart context — one server cart per signed-in user; guests keep a cart
 * keyed by a localStorage guest id that merges on login via
 * POST /cart/merge (PRD §3.2.3, TDD §6). All totals come from the API.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { api } from '~/lib/api'
import type { Cart } from '~/lib/types'
import { useAuth } from '~/lib/auth'

const GUEST_KEY = 'jdz.guest'

export function getGuestId(): string | undefined {
  if (typeof localStorage === 'undefined') return undefined
  let g = localStorage.getItem(GUEST_KEY)
  if (!g) {
    g = `guest_${Math.random().toString(36).slice(2, 10)}`
    localStorage.setItem(GUEST_KEY, g)
  }
  return g
}

interface Owner {
  token?: string
  guestId?: string
}

export interface CartValue {
  cart: Cart | null
  busy: boolean
  /** header badge — sum of distinct SKUs like the API's item_count */
  count: number
  refresh: () => Promise<void>
  add: (skuId: number, qty?: number) => Promise<void>
  setQty: (skuId: number, qty: number) => Promise<void>
  remove: (skuId: number) => Promise<void>
  bulkRemove: (skuIds: number[]) => Promise<void>
}

const CartContext = createContext<CartValue | null>(null)

export function CartProvider({
  locale,
  currency,
  children,
}: {
  locale: string
  currency: string
  children: React.ReactNode
}) {
  const { token, ready } = useAuth()
  const [cart, setCart] = useState<Cart | null>(null)
  const [busy, setBusy] = useState(false)

  const owner: Owner = token ? { token } : { guestId: getGuestId() }

  const load = useCallback(async () => {
    try {
      setCart(await api.getCart({ ...owner, locale, currency }))
    } catch {
      setCart(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, locale, currency])

  // initial load + reload when the session changes (incl. login merge)
  useEffect(() => {
    if (!ready) return
    let cancelled = false
    void (async () => {
      // login just happened (token present, guest cart may hold items) → merge
      if (token) {
        try {
          const guestId = getGuestId()
          const guest = await api.getCart({ guestId, locale, currency })
          if (guest && guest.items.length > 0) {
            const merged = await api.mergeCart(
              token,
              guestId,
              guest.items.map((i) => ({ sku_id: i.sku_id, qty: i.qty })),
              locale,
              currency,
            )
            if (!cancelled) setCart(merged)
            return
          }
        } catch {
          /* fall through to normal load */
        }
      }
      await load()
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, token, locale, currency])

  const mutate = useCallback(
    async (fn: (o: Owner) => Promise<Cart>) => {
      setBusy(true)
      try {
        setCart(await fn(owner))
      } finally {
        setBusy(false)
      }
    },
    // owner identity (token/guestId) is captured via closure
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [token],
  )

  // mutations carry locale+currency so responses include presentment totals
  const withCtx = useCallback(
    (o: Owner): Owner & { locale: string; currency: string } => ({ ...o, locale, currency }),
    [locale, currency],
  )

  const value = useMemo<CartValue>(
    () => ({
      cart,
      busy,
      count: cart?.item_count ?? 0,
      refresh: load,
      add: (skuId, qty = 1) => mutate((o) => api.addCartItem(withCtx(o), skuId, qty)),
      setQty: (skuId, qty) => mutate((o) => api.updateCartItem(withCtx(o), skuId, qty)),
      remove: (skuId) => mutate((o) => api.removeCartItem(withCtx(o), skuId)),
      bulkRemove: (skuIds) => mutate((o) => api.bulkRemoveCartItems(withCtx(o), skuIds)),
    }),
    [cart, busy, load, mutate, withCtx],
  )

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>
}

export function useCart(): CartValue {
  const ctx = useContext(CartContext)
  if (!ctx) throw new Error('useCart must be used inside CartProvider')
  return ctx
}
