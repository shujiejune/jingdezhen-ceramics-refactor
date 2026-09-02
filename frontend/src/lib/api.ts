/**
 * API client (TDD §4.3 / frontend AGENTS "Errors").
 *
 * A thin typed surface over a Transport. In PROTOTYPE mode the transport
 * is the in-process mock (src/mocks/transport.ts) which mirrors the Go
 * API's routes and error envelope. With VITE_API_MODE=live the fetch
 * transport talks to the sibling Fiber backend (routes live at the root,
 * e.g. /catalog/products — the reverse proxy maps /api/* → backend /*).
 *
 * Errors are keyed by stable domain codes (snake_cased from
 * ../backend/internal/models/errors.go), never by HTTP status.
 */
import type {
  Activity,
  Address,
  Artist,
  AuthResponse,
  Cart,
  Certificate,
  CeramicStory,
  ItineraryRequest,
  Order,
  Paginated,
  Pending2FAResponse,
  Product,
  ShippingQuote,
  SKU,
  Tag,
  User,
  WishlistItem,
} from './types'

export class ApiError extends Error {
  code: string
  status: number
  details?: Record<string, string>

  constructor(code: string, message: string, status = 400, details?: Record<string, string>) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.details = details
  }

  is(...codes: string[]): boolean {
    return codes.includes(this.code)
  }
}

export interface CallOptions {
  params?: Record<string, string | number | boolean | undefined>
  body?: unknown
  token?: string
  guestId?: string
}

export interface Transport {
  call<T>(
    method: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE',
    path: string,
    opts?: CallOptions,
  ): Promise<T>
}

/* ------------------------------------------------------------------ */
/* Live transport (fetch → Fiber API; unused until backend wiring)     */
/* ------------------------------------------------------------------ */

const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? ''

class LiveTransport implements Transport {
  async call<T>(method: string, path: string, opts: CallOptions = {}): Promise<T> {
    const url = new URL(API_BASE + path, window.location.origin)
    for (const [k, v] of Object.entries(opts.params ?? {})) {
      if (v !== undefined && v !== '') url.searchParams.set(k, String(v))
    }
    const res = await fetch(url, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(opts.token ? { Authorization: `Bearer ${opts.token}` } : {}),
        ...(opts.guestId ? { 'X-Guest-Id': opts.guestId } : {}),
      },
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    })
    const json = await res.json().catch(() => null)
    if (!res.ok) {
      const env = json as {
        error?: { code?: string; message?: string; details?: Record<string, string> }
      } | null
      throw new ApiError(
        env?.error?.code ?? 'unknown',
        env?.error?.message ?? res.statusText,
        res.status,
        env?.error?.details,
      )
    }
    return json as T
  }
}

/* ------------------------------------------------------------------ */
/* Client                                                              */
/* ------------------------------------------------------------------ */

async function transport(): Promise<Transport> {
  if (import.meta.env.VITE_API_MODE === 'live') {
    return new LiveTransport()
  }
  const { mockTransport } = await import('~/mocks/transport')
  return mockTransport
}

export interface CatalogQuery {
  locale: string
  currency?: string
  page?: number
  limit?: number
  tag?: string
  artist?: string
  edition?: string
  priceBand?: 'low' | 'mid' | 'high'
  sort?: 'featured' | 'price_asc' | 'price_desc' | 'newest'
  q?: string
}

async function t(): Promise<Transport> {
  return transport()
}

export const api = {
  /* ---- public catalog ---- */
  getProducts: (query: CatalogQuery) =>
    t().then((x) =>
      x.call<Paginated<Product>>('GET', '/catalog/products', { params: { ...query } }),
    ),
  getProduct: (slug: string, locale: string, currency?: string) =>
    t().then((x) =>
      x.call<Product>('GET', `/catalog/products/${slug}`, { params: { locale, currency } }),
    ),
  getTags: (locale: string) =>
    t().then((x) =>
      x.call<Array<Tag & { product_count: number }>>('GET', '/catalog/tags', {
        params: { locale },
      }),
    ),
  getArtists: (locale: string) =>
    t().then((x) => x.call<Artist[]>('GET', '/artists', { params: { locale } })),
  getArtist: (slug: string, locale: string) =>
    t().then((x) => x.call<Artist>('GET', `/artists/${slug}`, { params: { locale } })),
  getStories: (locale: string) =>
    t().then((x) => x.call<CeramicStory[]>('GET', '/ceramicstory', { params: { locale } })),
  getStory: (slug: string, locale: string) =>
    t().then((x) => x.call<CeramicStory>('GET', `/ceramicstory/${slug}`, { params: { locale } })),
  getActivities: (locale: string, type?: string) =>
    t().then((x) => x.call<Activity[]>('GET', '/engage', { params: { locale, type } })),
  getActivity: (slug: string, locale: string) =>
    t().then((x) => x.call<Activity>('GET', `/engage/${slug}`, { params: { locale } })),
  getCertificate: (code: string, locale: string) =>
    t().then((x) => x.call<Certificate>('GET', `/certificates/${code}`, { params: { locale } })),
  getShippingQuote: (country: string, weightGrams: number, currency?: string) =>
    t().then((x) =>
      x.call<ShippingQuote>('GET', '/shipping/quote', {
        params: { country, weight: weightGrams, currency },
      }),
    ),

  /* ---- auth ---- */
  login: (email: string, password: string) =>
    t().then((x) =>
      x.call<AuthResponse | Pending2FAResponse>('POST', '/auth/login', {
        body: { email, password },
      }),
    ),
  verify2FA: (pendingToken: string, code: string) =>
    t().then((x) =>
      x.call<AuthResponse>('POST', '/auth/2fa/verify', {
        body: { pending_token: pendingToken, code },
      }),
    ),
  signup: (body: { email: string; password: string; nickname: string }) =>
    t().then((x) => x.call<AuthResponse>('POST', '/auth/signup', { body })),

  /* ---- profile ---- */
  getProfile: (token: string) => t().then((x) => x.call<User>('GET', '/profile', { token })),
  updateProfile: (
    token: string,
    patch: Partial<Pick<User, 'nickname' | 'preferred_locale' | 'preferred_currency'>>,
  ) => t().then((x) => x.call<User>('PUT', '/profile', { token, body: patch })),
  getAddresses: (token: string) =>
    t().then((x) => x.call<Address[]>('GET', '/profile/addresses', { token })),
  createAddress: (
    token: string,
    body: Omit<Address, 'id' | 'is_default'> & { is_default?: boolean },
  ) => t().then((x) => x.call<Address>('POST', '/profile/addresses', { token, body })),

  /* ---- cart ---- */
  getCart: (o: { token?: string; guestId?: string; locale: string; currency?: string }) =>
    t().then((x) =>
      x.call<Cart>('GET', '/cart', {
        token: o.token,
        guestId: o.guestId,
        params: { locale: o.locale, currency: o.currency },
      }),
    ),
  addCartItem: (o: { token?: string; guestId?: string }, skuId: number, qty = 1) =>
    t().then((x) => x.call<Cart>('POST', '/cart/items', { ...o, body: { sku_id: skuId, qty } })),
  updateCartItem: (o: { token?: string; guestId?: string }, skuId: number, qty: number) =>
    t().then((x) => x.call<Cart>('PATCH', `/cart/items/${skuId}`, { ...o, body: { qty } })),
  removeCartItem: (o: { token?: string; guestId?: string }, skuId: number) =>
    t().then((x) => x.call<Cart>('DELETE', `/cart/items/${skuId}`, o)),
  bulkRemoveCartItems: (o: { token?: string; guestId?: string }, skuIds: number[]) =>
    t().then((x) => x.call<Cart>('DELETE', '/cart/items', { ...o, body: { sku_ids: skuIds } })),
  mergeCart: (
    token: string,
    guestId: string | undefined,
    guestItems: Array<{ sku_id: number; qty: number }>,
    locale: string,
    currency: string,
  ) =>
    t().then((x) =>
      x.call<Cart>('POST', '/cart/merge', {
        token,
        guestId,
        params: { locale, currency },
        body: { items: guestItems },
      }),
    ),

  /* ---- wishlist ---- */
  getWishlist: (token: string, locale: string, currency?: string) =>
    t().then((x) =>
      x.call<WishlistItem[]>('GET', '/wishlist', { token, params: { locale, currency } }),
    ),
  addToWishlist: (token: string, skuId: number) =>
    t().then((x) => x.call<void>('POST', '/wishlist', { token, body: { sku_id: skuId } })),
  removeFromWishlist: (token: string, skuId: number) =>
    t().then((x) => x.call<void>('DELETE', `/wishlist/${skuId}`, { token })),

  /* ---- orders / checkout ---- */
  checkout: (
    token: string,
    body: {
      address_id: number
      currency: string
      gateway: string
      locale: string
      consent: boolean
    },
  ) => t().then((x) => x.call<Order>('POST', '/checkout', { token, body })),
  listOrders: (token: string, locale: string) =>
    t().then((x) => x.call<Order[]>('GET', '/orders', { token, params: { locale } })),
  getOrder: (token: string, id: number, locale: string) =>
    t().then((x) => x.call<Order>('GET', `/orders/${id}`, { token, params: { locale } })),
  cancelOrder: (token: string, id: number, reason?: string) =>
    t().then((x) => x.call<Order>('POST', `/orders/${id}/cancel`, { token, body: { reason } })),
  /** mock-only: the sandbox gateway's webhook callback. */
  simulatePayment: (token: string, orderId: number) =>
    t().then((x) => x.call<Order>('POST', `/mock/pay/${orderId}`, { token })),

  /* ---- itineraries ---- */
  listItineraries: (token: string) =>
    t().then((x) => x.call<ItineraryRequest[]>('GET', '/itineraries', { token })),
  submitItinerary: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<ItineraryRequest>('POST', '/itineraries', { token, body })),
}

/** Pick a SKU's presentment price helper (server-provided only). */
export function skuPrice(sku: SKU): number {
  return sku.price ?? sku.price_cny
}
