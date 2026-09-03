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
  AdminAssignInput,
  AdminItineraryNote,
  AdminSendQuoteInput,
  AdminUser,
  Artist,
  AuditLog,
  AuthResponse,
  BulkImportSummary,
  Cart,
  Certificate,
  CeramicStory,
  ConsentKind,
  ConsentRecord,
  ConsentState,
  DashboardFunnel,
  DashboardSales,
  DashboardTraffic,
  DepositPaidResponse,
  ItineraryQuote,
  ItineraryRequest,
  MediaAsset,
  Notification,
  OptionRate,
  Order,
  Paginated,
  Product,
  ShippingQuote,
  ShippingTier,
  SKU,
  Tag,
  User,
  UserDataExport,
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
    t()
      .then((x) => x.call<{ data: Address[] }>('GET', '/profile/addresses', { token }))
      .then((res) => res.data),
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
    t().then((x) => x.call<Paginated<ItineraryRequest>>('GET', '/itineraries', { token })),
  getItinerary: (token: string, id: number) =>
    t().then((x) => x.call<ItineraryRequest>('GET', `/itineraries/${id}`, { token })),
  submitItinerary: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<ItineraryRequest>('POST', '/itineraries', { token, body })),
  getItineraryDraft: (token: string) =>
    t().then((x) => x.call<ItineraryRequest>('GET', '/itineraries/draft', { token })),
  saveItineraryDraft: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<void>('PUT', '/itineraries/draft', { token, body })),
  deleteItineraryDraft: (token: string) =>
    t().then((x) => x.call<void>('DELETE', '/itineraries/draft', { token })),
  getItineraryQuote: (token: string, id: number) =>
    t().then((x) => x.call<ItineraryQuote>('GET', `/itineraries/${id}/quote`, { token })),
  payItineraryDeposit: (token: string, id: number, gateway: string) =>
    t().then((x) =>
      x.call<DepositPaidResponse>('POST', `/itineraries/${id}/pay-deposit`, {
        token,
        body: { gateway },
      }),
    ),
  cancelItinerary: (token: string, id: number, reason?: string) =>
    t().then((x) => x.call<void>('POST', `/itineraries/${id}/cancel`, { token, body: { reason } })),

  /* ---- addresses (CRUD + set-default) ---- */
  getAddress: (token: string, id: number) =>
    t().then((x) => x.call<Address>('GET', `/profile/addresses/${id}`, { token })),
  updateAddress: (
    token: string,
    id: number,
    body: Partial<Omit<Address, 'id' | 'is_default'>> & { is_default?: boolean },
  ) => t().then((x) => x.call<Address>('PUT', `/profile/addresses/${id}`, { token, body })),
  deleteAddress: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/profile/addresses/${id}`, { token })),
  setDefaultAddress: (token: string, id: number) =>
    t().then((x) => x.call<void>('POST', `/profile/addresses/${id}/default`, { token })),

  /* ---- notifications ---- */
  listNotifications: (token: string, page = 1) =>
    t().then((x) =>
      x.call<Paginated<Notification>>('GET', '/notifications', {
        token,
        params: { page },
      }),
    ),
  getUnreadNotificationCount: (token: string) =>
    t().then((x) => x.call<{ count: number }>('GET', '/notifications/unread-count', { token })),
  markNotificationRead: (token: string, id: number) =>
    t().then((x) => x.call<void>('POST', `/notifications/${id}/mark-read`, { token })),
  markAllNotificationsRead: (token: string) =>
    t().then((x) => x.call<void>('POST', '/notifications/mark-all-read', { token })),

  /* ---- consent ---- */
  recordConsent: (
    body: { kind: ConsentKind; doc_version: string; granted: boolean },
    token?: string,
  ) => t().then((x) => x.call<ConsentRecord>('POST', '/consent', { body, token })),
  getConsentHistory: (token: string) =>
    t()
      .then((x) => x.call<{ data: ConsentRecord[] }>('GET', '/profile/consent', { token }))
      .then((res) => res.data),
  getConsentState: (token: string, kind: ConsentKind) =>
    t().then((x) => x.call<ConsentState>('GET', `/profile/consent/${kind}`, { token })),

  /* ---- GDPR ---- */
  exportUserData: (token: string, locale?: string) =>
    t().then((x) =>
      x.call<UserDataExport>('GET', '/profile/export', {
        token,
        params: locale ? { locale } : undefined,
      }),
    ),
  deleteAccount: (token: string) =>
    t().then((x) =>
      x.call<void>('POST', '/privacy/delete-account', { token, body: { confirm: 'DELETE' } }),
    ),

  /* ---- analytics ---- */
  trackAnalytics: (body: {
    kind: 'pageview' | 'event'
    path: string
    name?: string
    locale?: string
    props?: Record<string, unknown>
  }) => t().then((x) => x.call<{ id: number } | void>('POST', '/analytics/events', { body })),

  /* ---- admin: content (stories) ---- */
  adminListStories: (token: string, locale?: string) =>
    t().then((x) =>
      x.call<Paginated<CeramicStory>>('GET', '/admin/ceramicstory', {
        token,
        params: locale ? { locale } : undefined,
      }),
    ),
  adminGetStory: (token: string, slug: string) =>
    t().then((x) => x.call<CeramicStory>('GET', `/admin/ceramicstory/${slug}`, { token })),
  adminCreateStory: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<CeramicStory>('POST', '/admin/ceramicstory', { token, body })),
  adminUpdateStory: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<CeramicStory>('PUT', `/admin/ceramicstory/${id}`, { token, body })),
  adminDeleteStory: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/ceramicstory/${id}`, { token })),
  adminSubmitStory: (token: string, id: number) =>
    t().then((x) => x.call<CeramicStory>('POST', `/admin/ceramicstory/${id}/submit`, { token })),
  adminApproveStory: (token: string, id: number) =>
    t().then((x) => x.call<CeramicStory>('POST', `/admin/ceramicstory/${id}/approve`, { token })),
  adminRejectStory: (token: string, id: number) =>
    t().then((x) => x.call<CeramicStory>('POST', `/admin/ceramicstory/${id}/reject`, { token })),
  adminUnpublishStory: (token: string, id: number) =>
    t().then((x) => x.call<CeramicStory>('POST', `/admin/ceramicstory/${id}/unpublish`, { token })),

  /* ---- admin: content (activities) ---- */
  adminListActivities: (token: string, locale?: string) =>
    t().then((x) =>
      x.call<Paginated<Activity>>('GET', '/admin/engage', {
        token,
        params: locale ? { locale } : undefined,
      }),
    ),
  adminGetActivity: (token: string, slug: string) =>
    t().then((x) => x.call<Activity>('GET', `/admin/engage/${slug}`, { token })),
  adminCreateActivity: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<Activity>('POST', '/admin/engage', { token, body })),
  adminUpdateActivity: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<Activity>('PUT', `/admin/engage/${id}`, { token, body })),
  adminDeleteActivity: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/engage/${id}`, { token })),
  adminSubmitActivity: (token: string, id: number) =>
    t().then((x) => x.call<Activity>('POST', `/admin/engage/${id}/submit`, { token })),
  adminApproveActivity: (token: string, id: number) =>
    t().then((x) => x.call<Activity>('POST', `/admin/engage/${id}/approve`, { token })),
  adminRejectActivity: (token: string, id: number) =>
    t().then((x) => x.call<Activity>('POST', `/admin/engage/${id}/reject`, { token })),
  adminUnpublishActivity: (token: string, id: number) =>
    t().then((x) => x.call<Activity>('POST', `/admin/engage/${id}/unpublish`, { token })),

  /* ---- admin: content (artists) ---- */
  adminListArtists: (token: string, locale?: string) =>
    t().then((x) =>
      x.call<Paginated<Artist>>('GET', '/admin/artists', {
        token,
        params: locale ? { locale } : undefined,
      }),
    ),
  adminGetArtist: (token: string, slug: string) =>
    t().then((x) => x.call<Artist>('GET', `/admin/artists/${slug}`, { token })),
  adminCreateArtist: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<Artist>('POST', '/admin/artists', { token, body })),
  adminUpdateArtist: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<Artist>('PUT', `/admin/artists/${id}`, { token, body })),
  adminDeleteArtist: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/artists/${id}`, { token })),
  adminSubmitArtist: (token: string, id: number) =>
    t().then((x) => x.call<Artist>('POST', `/admin/artists/${id}/submit`, { token })),
  adminApproveArtist: (token: string, id: number) =>
    t().then((x) => x.call<Artist>('POST', `/admin/artists/${id}/approve`, { token })),
  adminRejectArtist: (token: string, id: number) =>
    t().then((x) => x.call<Artist>('POST', `/admin/artists/${id}/reject`, { token })),
  adminUnpublishArtist: (token: string, id: number) =>
    t().then((x) => x.call<Artist>('POST', `/admin/artists/${id}/unpublish`, { token })),

  /* ---- admin: products + SKUs ---- */
  adminListProducts: (token: string, locale?: string) =>
    t().then((x) =>
      x.call<Paginated<Product>>('GET', '/admin/products', {
        token,
        params: locale ? { locale } : undefined,
      }),
    ),
  adminGetProduct: (token: string, slug: string) =>
    t().then((x) => x.call<Product>('GET', `/admin/products/${slug}`, { token })),
  adminCreateProduct: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<Product>('POST', '/admin/products', { token, body })),
  adminUpdateProduct: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<Product>('PUT', `/admin/products/${id}`, { token, body })),
  adminDeleteProduct: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/products/${id}`, { token })),
  adminSubmitProduct: (token: string, id: number) =>
    t().then((x) => x.call<Product>('POST', `/admin/products/${id}/submit`, { token })),
  adminApproveProduct: (token: string, id: number) =>
    t().then((x) => x.call<Product>('POST', `/admin/products/${id}/approve`, { token })),
  adminRejectProduct: (token: string, id: number) =>
    t().then((x) => x.call<Product>('POST', `/admin/products/${id}/reject`, { token })),
  adminUnpublishProduct: (token: string, id: number) =>
    t().then((x) => x.call<Product>('POST', `/admin/products/${id}/unpublish`, { token })),
  adminBulkImportProducts: (token: string, body: { csv: string }) =>
    t().then((x) => x.call<BulkImportSummary>('POST', '/admin/products/import', { token, body })),
  adminCreateSKU: (token: string, productId: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<SKU>('POST', `/admin/products/${productId}/skus`, { token, body })),
  adminUpdateSKU: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<SKU>('PUT', `/admin/skus/${id}`, { token, body })),
  adminDeleteSKU: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/skus/${id}`, { token })),

  /* ---- admin: orders ---- */
  adminListOrders: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) => t().then((x) => x.call<Paginated<Order>>('GET', '/admin/orders', { token, params })),
  adminGetOrder: (token: string, id: number) =>
    t().then((x) => x.call<Order>('GET', `/admin/orders/${id}`, { token })),
  adminShipOrder: (
    token: string,
    id: number,
    body: { carrier_name: string; tracking_number: string },
  ) => t().then((x) => x.call<Order>('POST', `/admin/orders/${id}/ship`, { token, body })),
  adminCompleteOrder: (token: string, id: number) =>
    t().then((x) => x.call<Order>('POST', `/admin/orders/${id}/complete`, { token })),
  adminRefundOrder: (token: string, id: number) =>
    t().then((x) => x.call<Order>('POST', `/admin/orders/${id}/refund`, { token })),

  /* ---- admin: itineraries CRM ---- */
  adminListItineraries: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) =>
    t().then((x) =>
      x.call<Paginated<ItineraryRequest>>('GET', '/admin/itineraries', { token, params }),
    ),
  adminGetItinerary: (token: string, id: number) =>
    t().then((x) => x.call<ItineraryRequest>('GET', `/admin/itineraries/${id}`, { token })),
  adminListItineraryNotes: (token: string, id: number) =>
    t().then((x) =>
      x.call<{ data: AdminItineraryNote[] }>('GET', `/admin/itineraries/${id}/notes`, { token }),
    ),
  adminAddItineraryNote: (token: string, id: number, body: { body: string }) =>
    t().then((x) =>
      x.call<AdminItineraryNote>('POST', `/admin/itineraries/${id}/notes`, { token, body }),
    ),
  adminAssignItinerary: (token: string, id: number, body: AdminAssignInput) =>
    t().then((x) =>
      x.call<ItineraryRequest>('POST', `/admin/itineraries/${id}/assign`, { token, body }),
    ),
  adminSendQuote: (token: string, id: number, body: AdminSendQuoteInput) =>
    t().then((x) =>
      x.call<ItineraryQuote>('POST', `/admin/itineraries/${id}/quote`, { token, body }),
    ),
  adminConfirmItinerary: (token: string, id: number) =>
    t().then((x) =>
      x.call<ItineraryRequest>('POST', `/admin/itineraries/${id}/confirm`, { token }),
    ),
  adminRefundDeposit: (token: string, id: number) =>
    t().then((x) =>
      x.call<ItineraryRequest>('POST', `/admin/itineraries/${id}/refund-deposit`, { token }),
    ),
  adminListPlanners: (token: string) =>
    t().then((x) => x.call<{ data: AdminUser[] }>('GET', '/admin/itineraries/planners', { token })),
  adminListOptionRates: (token: string) =>
    t().then((x) =>
      x.call<{ data: OptionRate[] }>('GET', '/admin/itineraries/option-rates', { token }),
    ),
  adminCreateOptionRate: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<OptionRate>('POST', '/admin/itineraries/option-rates', { token, body })),
  adminUpdateOptionRate: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) =>
      x.call<OptionRate>('PUT', `/admin/itineraries/option-rates/${id}`, { token, body }),
    ),
  adminDeleteOptionRate: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/itineraries/option-rates/${id}`, { token })),

  /* ---- admin: media ---- */
  adminListMediaAssets: (token: string) =>
    t().then((x) => x.call<{ data: MediaAsset[] }>('GET', '/admin/media/assets', { token })),
  adminRegisterAsset: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<MediaAsset>('POST', '/admin/media/assets', { token, body })),
  adminDeleteAsset: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/media/assets/${id}`, { token })),
  adminUploadLocal: (token: string, body: FormData) =>
    t().then((x) => x.call<MediaAsset>('POST', '/admin/media/upload', { token, body })),

  /* ---- admin: certificates ---- */
  adminListCertificates: (token: string) =>
    t().then((x) => x.call<Paginated<Certificate>>('GET', '/admin/certificates', { token })),
  adminRegenerateCertificate: (token: string, id: number) =>
    t().then((x) => x.call<Certificate>('POST', `/admin/certificates/${id}/regenerate`, { token })),

  /* ---- admin: settings ---- */
  adminListShippingTiers: (token: string) =>
    t().then((x) => x.call<{ data: ShippingTier[] }>('GET', '/admin/shipping/tiers', { token })),
  adminCreateShippingTier: (token: string, body: Record<string, unknown>) =>
    t().then((x) => x.call<ShippingTier>('POST', '/admin/shipping/tiers', { token, body })),
  adminUpdateShippingTier: (token: string, id: number, body: Record<string, unknown>) =>
    t().then((x) => x.call<ShippingTier>('PUT', `/admin/shipping/tiers/${id}`, { token, body })),
  adminDeleteShippingTier: (token: string, id: number) =>
    t().then((x) => x.call<void>('DELETE', `/admin/shipping/tiers/${id}`, { token })),
  adminRefreshFX: (token: string) =>
    t().then((x) => x.call<{ ok: boolean }>('POST', '/admin/fx/refresh', { token })),

  /* ---- admin: dashboard ---- */
  adminDashboardTraffic: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) =>
    t().then((x) => x.call<DashboardTraffic>('GET', '/admin/analytics/traffic', { token, params })),
  adminDashboardSales: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) => t().then((x) => x.call<DashboardSales>('GET', '/admin/analytics/sales', { token, params })),
  adminDashboardFunnel: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) =>
    t().then((x) => x.call<DashboardFunnel>('GET', '/admin/analytics/funnel', { token, params })),

  /* ---- admin: users ---- */
  adminListUsers: (token: string, params?: Record<string, string | number | boolean | undefined>) =>
    t().then((x) => x.call<Paginated<AdminUser>>('GET', '/admin/users', { token, params })),
  adminAssignRole: (token: string, userId: string, role: string) =>
    t().then((x) =>
      x.call<AdminUser>('PUT', `/admin/users/${userId}/role`, { token, body: { role } }),
    ),

  /* ---- admin: audit log ---- */
  adminListAuditLog: (
    token: string,
    params?: Record<string, string | number | boolean | undefined>,
  ) => t().then((x) => x.call<Paginated<AuditLog>>('GET', '/admin/audit-log', { token, params })),
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

/**
 * Return a usable <img src> for a media URL, or undefined when the URL
 * is a mock-mode placeholder (e.g. `mock://media/…`) that can't be
 * loaded as a real image. Callers fall back to PorcelainFigure.
 */
export function mediaImageUrl(url: string | undefined | null): string | undefined {
  if (!url) return undefined
  if (url.startsWith('mock://')) return undefined
  return resolveMediaUrl(url)
}

/**
 * Build a srcSet for responsive image loading. For OSS/CDN URLs this
 * requests multiple widths; for local /media/ paths it returns the
 * single resolved URL (the dev proxy doesn't support image processing).
 */
export function mediaSrcSet(url: string | undefined | null): string | undefined {
  const resolved = mediaImageUrl(url)
  if (!resolved) return undefined
  if (/^https?:\/\//.test(resolved)) {
    // OSS image processing params (if available); otherwise just 1x
    return `${resolved} 1x`
  }
  return undefined
}
