/**
 * API client (TDD §4.3 / §5.1 + frontend AGENTS "Errors").
 *
 * A thin typed surface over a Transport. VITE_API_MODE=mock (default)
 * uses the in-process mock (src/mocks/transport.ts); `live` talks to the
 * sibling Fiber backend. The REAL wire contract (inventoried from the
 * backend):
 *   - success bodies are FLAT (no {data} wrapper) except
 *     PaginatedResponse {data,page,limit,total,total_pages} and the
 *     ad-hoc {data:[…]} wrappers (consent history, media assets);
 *   - errors are {"message": "..."} (+ optional "details" string) —
 *     there is NO code field today; Fiber's default 404/405/panics are
 *     plain text;
 *   - the ONE structured error is the login 2FA challenge: 401
 *     {"error":{code:"2fa_required"|"2fa_enrollment_required",
 *     message, pending_token}};
 *   - 204 means success-with-no-body (analytics consent drop,
 *     mark-read) — never an error.
 *
 * classifyApiError() turns that reality into stable domain codes
 * (snake_cased from models/errors.go) so UI code never switches on
 * HTTP status.
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
  /** the 2FA login challenge carries the pending token */
  pendingToken?: string

  constructor(
    code: string,
    message: string,
    status = 400,
    details?: Record<string, string>,
    pendingToken?: string,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.details = details
    this.pendingToken = pendingToken
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
/* Error classification (pure — unit-tested against real fixtures)     */
/* ------------------------------------------------------------------ */

const MESSAGE_CODE_RULES: Array<[RegExp, string]> = [
  [/invalid email or password|credential/i, 'invalid_credentials'],
  [/invalid or expired jwt|missing or malformed jwt|token revoked/i, 'unauthorized'],
  [/cart is empty/i, 'cart_empty'],
  [/not shippable/i, 'unshippable'],
  [/maximum shipping weight|exceeds the maximum weight|overweight/i, 'overweight'],
  [/consent/i, 'consent_required'],
  [/too many|rate limit/i, 'too_many_attempts'],
  [/stock/i, 'conflict'],
]

/**
 * Turn a raw HTTP failure into an ApiError using everything the backend
 * actually sends today: the 2FA challenge envelope, {message} JSON
 * bodies, or Fiber's plain-text defaults.
 */
export function classifyApiError(status: number, bodyText: string): ApiError {
  let body: unknown
  try {
    body = bodyText ? JSON.parse(bodyText) : null
  } catch {
    body = null
  }

  if (body && typeof body === 'object') {
    const b = body as {
      message?: string
      details?: string
      error?: { code?: string; message?: string; pending_token?: string }
      pending_token?: string
    }
    // structured envelope — only the 2FA login challenge today
    if (b.error?.code) {
      return new ApiError(
        b.error.code,
        b.error.message ?? b.error.code,
        status,
        undefined,
        b.error.pending_token ?? b.pending_token,
      )
    }
    if (typeof b.message === 'string') {
      for (const [re, code] of MESSAGE_CODE_RULES) {
        if (re.test(b.message)) return new ApiError(code, b.message, status, detailsOf(b.details))
      }
      return new ApiError(statusCodeFallback(status), b.message, status, detailsOf(b.details))
    }
  }

  // Fiber default bodies are plain text ("Cannot GET /x", 405 message…)
  const text = bodyText.trim().split('\n')[0]?.slice(0, 160) || `HTTP ${status}`
  return new ApiError(statusCodeFallback(status), text, status)
}

function detailsOf(details: string | undefined): Record<string, string> | undefined {
  // the backend's "details" is a free string (go-playground validator
  // output today) — surface it raw on the generic field key
  return details ? { form: details } : undefined
}

function statusCodeFallback(status: number): string {
  switch (status) {
    case 400:
      return 'bad_request'
    case 401:
      return 'unauthorized'
    case 403:
      return 'forbidden'
    case 404:
      return 'not_found'
    case 409:
      return 'conflict'
    case 422:
      return 'validation_failed'
    case 429:
      return 'too_many_attempts'
    default:
      return status >= 500 ? 'internal' : 'bad_request'
  }
}

/* ------------------------------------------------------------------ */
/* Live transport (fetch → Fiber API)                                  */
/* ------------------------------------------------------------------ */

const SSR_API_BASE =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? 'http://localhost:1323'

/** SSR loaders call Fiber directly (server-to-server, no CORS). In the
 *  browser: dev calls the API origin directly (CORS via CLIENT_ORIGIN —
 *  the vite /api proxy is shadowed by the Start dev middleware for
 *  extension-less paths); prod goes same-origin /api behind the reverse
 *  proxy. VITE_API_BROWSER_BASE overrides both. */
function liveBase(): string {
  if (typeof window === 'undefined') return SSR_API_BASE
  const explicit = import.meta.env.VITE_API_BROWSER_BASE as string | undefined
  if (explicit) return explicit
  return import.meta.env.PROD ? '/api' : SSR_API_BASE
}

export class ApiNetworkError extends ApiError {
  constructor() {
    super('network', 'Cannot reach the server', 0)
  }
}

class LiveTransport implements Transport {
  async call<T>(method: string, path: string, opts: CallOptions = {}): Promise<T> {
    const url = new URL(
      liveBase() + path,
      typeof window !== 'undefined' ? window.location.origin : 'http://localhost',
    )
    for (const [k, v] of Object.entries(opts.params ?? {})) {
      if (v !== undefined && v !== '') url.searchParams.set(k, String(v))
    }
    let res: Response
    try {
      res = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
          ...(opts.token ? { Authorization: `Bearer ${opts.token}` } : {}),
          ...(opts.guestId ? { 'X-Guest-Id': opts.guestId } : {}),
        },
        body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      })
    } catch {
      throw new ApiNetworkError()
    }
    const text = await res.text()
    if (!res.ok) throw classifyApiError(res.status, text)
    // 204 (and empty bodies) are successes with no payload
    if (res.status === 204 || text === '') return undefined as T
    try {
      return JSON.parse(text) as T
    } catch {
      return text as unknown as T
    }
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
  /** live /artists is PaginatedResponse-wrapped; mock matches */
  getArtists: (locale: string) =>
    t()
      .then((x) => x.call<Paginated<Artist> | Artist[]>('GET', '/artists', { params: { locale } }))
      .then((r) => (Array.isArray(r) ? r : r.data)),
  getArtist: (slug: string, locale: string) =>
    t().then((x) => x.call<Artist>('GET', `/artists/${slug}`, { params: { locale } })),
  getStories: (locale: string) =>
    t().then((x) => x.call<CeramicStory[]>('GET', '/ceramicstory', { params: { locale } })),
  getStory: (slug: string, locale: string) =>
    t().then((x) => x.call<CeramicStory>('GET', `/ceramicstory/${slug}`, { params: { locale } })),
  /** live /engage is PaginatedResponse-wrapped; mock matches */
  getActivities: (locale: string, type?: string) =>
    t()
      .then((x) =>
        x.call<Paginated<Activity> | Activity[]>('GET', '/engage', { params: { locale, type } }),
      )
      .then((r) => (Array.isArray(r) ? r : r.data)),
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
  /** OK → AuthResponse. A 2FA-enabled account throws
   *  ApiError('2fa_required'|'2fa_enrollment_required') with pendingToken. */
  login: (email: string, password: string) =>
    t().then((x) => x.call<AuthResponse>('POST', '/auth/login', { body: { email, password } })),
  verify2FA: (pendingToken: string, code: string) =>
    t().then((x) =>
      x.call<AuthResponse>('POST', '/auth/2fa/verify', {
        body: { pending_token: pendingToken, code },
      }),
    ),
  /** super_admin must-enroll: password proof → otpauth URI + secret */
  pending2FAEnroll: (pendingToken: string, password: string) =>
    t().then((x) =>
      x.call<{ otpauth_url: string; secret: string }>('POST', '/auth/2fa/pending-enroll', {
        body: { pending_token: pendingToken, password },
      }),
    ),
  /** confirms enrollment; backup codes are shown exactly once */
  pending2FAConfirm: (pendingToken: string, code: string) =>
    t().then((x) =>
      x.call<AuthResponse & { backup_codes: string[] }>('POST', '/auth/2fa/pending-confirm', {
        body: { pending_token: pendingToken, code },
      }),
    ),
  /** email-link activation: the API takes a JSON body, the link points here */
  activate: (token: string) =>
    t().then((x) => x.call<AuthResponse>('POST', '/auth/activate', { body: { token } })),
  resendActivation: (email: string) =>
    t().then((x) =>
      x.call<{ message: string }>('POST', '/auth/resend-activation', { body: { email } }),
    ),
  /** anti-enumeration: always 200 {message} */
  requestPasswordReset: (email: string) =>
    t().then((x) =>
      x.call<{ message: string; reset_token?: string }>('POST', '/auth/request-password-reset', {
        body: { email },
      }),
    ),
  resetPassword: (token: string, newPassword: string) =>
    t().then((x) =>
      x.call<AuthResponse>('POST', '/auth/reset-password', {
        body: { token, new_password: newPassword },
      }),
    ),
  signup: (body: { email: string; password: string; nickname: string }) =>
    t().then((x) =>
      x.call<AuthResponse & { activation_token?: string }>('POST', '/auth/signup', { body }),
    ),

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

/* ------------------------------------------------------------------ */
/* Media URLs                                                          */
/* ------------------------------------------------------------------ */

/**
 * Resolve a media public_url from the API for use in <img>/links.
 * Local-dev storage returns RELATIVE "/media/…" keys — in the browser
 * the dev proxy forwards them; during SSR they must be prefixed with
 * the API origin. Absolute URLs (OSS/CDN mode) pass through untouched.
 */
export function resolveMediaUrl(url: string | undefined | null): string | undefined {
  if (!url) return undefined
  if (/^https?:\/\//.test(url)) return url
  if (typeof window === 'undefined' && url.startsWith('/')) {
    return SSR_API_BASE + url
  }
  return url
}
