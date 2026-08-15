/** Wishlist context — protected resource (router.go: /wishlist behind JWT). */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { api } from '~/lib/api'
import type { WishlistItem } from '~/lib/types'
import { useAuth } from '~/lib/auth'

export interface WishlistValue {
  items: WishlistItem[]
  /** sku ids currently wishlisted (empty when signed out) */
  ids: Set<number>
  ready: boolean
  has: (skuId: number) => boolean
  toggle: (skuId: number) => Promise<'added' | 'removed'>
}

const WishlistContext = createContext<WishlistValue | null>(null)

export function WishlistProvider({ locale, currency, children }: { locale: string; currency: string; children: React.ReactNode }) {
  const { token, ready: authReady } = useAuth()
  const [items, setItems] = useState<WishlistItem[]>([])

  useEffect(() => {
    if (!authReady) return
    if (!token) {
      setItems([])
      return
    }
    void api
      .getWishlist(token, locale, currency)
      .then(setItems)
      .catch(() => setItems([]))
  }, [authReady, token, locale, currency])

  const toggle = useCallback<WishlistValue['toggle']>(
    async (skuId) => {
      if (!token) throw new Error('unauthorized')
      if (items.some((i) => i.sku_id === skuId)) {
        await api.removeFromWishlist(token, skuId)
        setItems((prev) => prev.filter((i) => i.sku_id !== skuId))
        return 'removed'
      }
      await api.addToWishlist(token, skuId)
      setItems(await api.getWishlist(token, locale, currency))
      return 'added'
    },
    [token, items, locale, currency],
  )

  const value = useMemo<WishlistValue>(
    () => ({
      items,
      ids: new Set(items.map((i) => i.sku_id)),
      ready: authReady,
      has: (skuId) => items.some((i) => i.sku_id === skuId),
      toggle,
    }),
    [items, authReady, toggle],
  )

  return <WishlistContext.Provider value={value}>{children}</WishlistContext.Provider>
}

export function useWishlist(): WishlistValue {
  const ctx = useContext(WishlistContext)
  if (!ctx) throw new Error('useWishlist must be used inside WishlistProvider')
  return ctx
}
