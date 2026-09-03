/**
 * Mock transport (PROTOTYPE) — implements the API surface defined by
 * ../backend/internal/api/router.go against the in-memory dataset.
 *
 * - Mutable state (sessions, carts, orders, …) lives at module scope, so
 *   it persists during a browser session. SSR loaders only read static
 *   public content, so server/client module instances never disagree
 *   about what matters.
 * - Errors use the target envelope { error: { code, message, details } }
 *   with codes snake_cased from models/errors.go.
 * - Money math (FX + rounding + shipping tiers) happens HERE, never in
 *   the UI (TDD §7).
 */
import { ApiError, type CallOptions, type Transport } from '~/lib/api'
import type {
  Activity,
  Address,
  Artist,
  Cart,
  CartItem,
  Certificate,
  CeramicStory,
  BulkImportSummary,
  ConsentKind,
  ConsentRecord,
  ConsentState,
  DepositPaidResponse,
  ItineraryQuote,
  ItineraryRequest,
  ItineraryStatus,
  Notification,
  Order,
  OrderItem,
  OrderStatus,
  Product,
  QuoteLineItem,
  SKU,
  Tag,
  User,
  UserDataExport,
  WishlistItem,
} from '~/lib/types'
import {
  ACTIVITIES,
  ARTISTS,
  CERTIFICATES,
  DEMO_USERS,
  PRODUCTS,
  SEED_ADDRESSES,
  SEED_ITINERARIES,
  SEED_ORDERS,
  SEED_WISHLIST,
  STORIES,
  TAGS,
  type ProductRecord,
} from './data'
import { convertMinor, effectiveRate, pick, quoteShipping, type MockLocale } from './engine'

type Currency = 'USD' | 'EUR' | 'GBP'

/* ------------------------------------------------------------------ */
/* Mutable module state                                                */
/* ------------------------------------------------------------------ */

interface CartLine {
  sku_id: number
  qty: number
  added_at: string
}

type StoredAddress = Address & { user_id: string }

const sessions = new Map<string, string>() // token → userId
const pending2FA = new Map<string, { userId: string; fails: number }>()
const activationTokens = new Map<string, string>() // token → userId (inactive users)
const resetTokens = new Map<string, string>() // token → userId
const userCarts = new Map<string, CartLine[]>()
const guestCarts = new Map<string, CartLine[]>()
const wishlists = new Map<string, number[]>(SEED_WISHLIST.map((w) => [w.user_id, [w.sku_id]]))
const addresses: StoredAddress[] = SEED_ADDRESSES.map((a) => ({ ...a }))
/** Stored orders keep the bilingual title snapshot; reads resolve it. */
type StoredOrderItem = Omit<OrderItem, 'title_snapshot'> & {
  title_snapshot: { enUS: string; zhCN: string }
}
type StoredOrder = Omit<Order, 'items'> & { items?: StoredOrderItem[] }
const orders: StoredOrder[] = []
const itineraries: ItineraryRequest[] = []
const notifications: Notification[] = []
const consentRecords: ConsentRecord[] = []
const itineraryDrafts = new Map<string, Record<string, unknown>>()
const itineraryQuotes = new Map<number, ItineraryQuote>()
const itineraryNotes = new Map<
  number,
  Array<{
    id: number
    itinerary_id: number
    author_id: string
    author_email: string
    body: string
    created_at: string
  }>
>()

/* admin mutable state */
const mediaAssets: Array<{
  id: number
  public_url: string
  caption?: string
  mime_type: string
  file_size: number
  created_at: string
}> = [
  {
    id: 1,
    public_url: 'mock://media/asset-1.png',
    mime_type: 'image/png',
    file_size: 240000,
    created_at: '2025-06-01T00:00:00Z',
  },
  {
    id: 2,
    public_url: 'mock://media/asset-2.png',
    mime_type: 'image/png',
    file_size: 180000,
    created_at: '2025-06-15T00:00:00Z',
  },
]

const shippingTiers: Array<{
  id: number
  country_code: string
  min_weight_grams: number
  max_weight_grams: number
  fee_cny: number
}> = [
  { id: 1, country_code: 'US', min_weight_grams: 0, max_weight_grams: 500, fee_cny: 12000 },
  { id: 2, country_code: 'US', min_weight_grams: 501, max_weight_grams: 2000, fee_cny: 22000 },
  { id: 3, country_code: 'GB', min_weight_grams: 0, max_weight_grams: 500, fee_cny: 14000 },
  { id: 4, country_code: 'GB', min_weight_grams: 501, max_weight_grams: 2000, fee_cny: 26000 },
]

const optionRates: Array<{
  id: number
  option_key: string
  label: string
  rate_cny: number
}> = [
  { id: 1, option_key: 'guide_english', label: 'English guide', rate_cny: 50000 },
  { id: 2, option_key: 'guide_other', label: 'Other-language guide', rate_cny: 60000 },
  { id: 3, option_key: 'hotel_luxury', label: 'Luxury hotel upgrade', rate_cny: 80000 },
  { id: 4, option_key: 'pickup', label: 'Airport pickup', rate_cny: 30000 },
]
let idSeq = {
  order: 2000,
  item: 9100,
  address: 10,
  itinerary: 6000,
  token: 100,
  notif: 100,
  consent: 100,
  quote: 7000,
}

/* seed orders/itineraries with resolved addresses. Order titles are
   stored bilingual (title_snapshot) and resolved to the request locale
   on read — like the Go service does with its snapshot JSONB. */
for (const o of SEED_ORDERS) {
  const addr = addresses.find((a) => a.id === o.address_id)!
  orders.push({ ...o, address: addr })
}
for (const r of SEED_ITINERARIES) itineraries.push({ ...r })
// seed a demo notification for the admin user
notifications.push({
  notification_id: 1,
  recipient_user_id: DEMO_USERS[1].id,
  notification_type: 'order_status',
  message: 'Your order #1001 has been paid and is being prepared.',
  is_read: false,
  created_at: new Date(Date.now() - 3600_000).toISOString(),
})

/* ------------------------------------------------------------------ */
/* Persistence — keeps the demo session alive across page reloads      */
/* (browser only; SSR loaders only read static public content).        */
/* ------------------------------------------------------------------ */

const STATE_KEY = 'jdz.mockstate'

function persist() {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.setItem(
      STATE_KEY,
      JSON.stringify({
        sessions: [...sessions],
        activationTokens: [...activationTokens],
        resetTokens: [...resetTokens],
        pending2FA: [...pending2FA],
        userCarts: [...userCarts],
        guestCarts: [...guestCarts],
        wishlists: [...wishlists],
        addresses,
        orders,
        itineraries,
        notifications,
        consentRecords,
        idSeq,
        extraUsers: DEMO_USERS.slice(2),
      }),
    )
  } catch {
    /* storage full/blocked — non-fatal for the prototype */
  }
}

function hydrate() {
  if (typeof localStorage === 'undefined') return
  try {
    const raw = localStorage.getItem(STATE_KEY)
    if (!raw) return
    const s = JSON.parse(raw) as {
      sessions?: Array<[string, string]>
      activationTokens?: Array<[string, string]>
      resetTokens?: Array<[string, string]>
      pending2FA?: Array<[string, { userId: string; fails: number }]>
      userCarts?: Array<[string, CartLine[]]>
      guestCarts?: Array<[string, CartLine[]]>
      wishlists?: Array<[string, number[]]>
      addresses?: StoredAddress[]
      orders?: StoredOrder[]
      itineraries?: ItineraryRequest[]
      notifications?: Notification[]
      consentRecords?: ConsentRecord[]
      idSeq?: typeof idSeq
      extraUsers?: Array<(typeof DEMO_USERS)[number]>
    }
    for (const [k, v] of s.sessions ?? []) sessions.set(k, v)
    for (const [k, v] of s.activationTokens ?? []) activationTokens.set(k, v)
    for (const [k, v] of s.resetTokens ?? []) resetTokens.set(k, v)
    for (const [k, v] of s.pending2FA ?? []) pending2FA.set(k, v)
    for (const [k, v] of s.userCarts ?? []) userCarts.set(k, v)
    for (const [k, v] of s.guestCarts ?? []) guestCarts.set(k, v)
    for (const [k, v] of s.wishlists ?? []) wishlists.set(k, v)
    if (s.addresses) addresses.splice(0, addresses.length, ...s.addresses)
    if (s.orders) orders.splice(0, orders.length, ...s.orders)
    if (s.itineraries) itineraries.splice(0, itineraries.length, ...s.itineraries)
    if (s.notifications) notifications.splice(0, notifications.length, ...s.notifications)
    if (s.consentRecords) consentRecords.splice(0, consentRecords.length, ...s.consentRecords)
    if (s.idSeq) idSeq = s.idSeq
    for (const u of s.extraUsers ?? []) DEMO_USERS.push(u)
  } catch {
    /* corrupted state — fall back to seeds */
  }
}
hydrate()

function presentOrder(o: StoredOrder, locale: MockLocale): Order {
  return {
    ...o,
    items: o.items?.map((i) => ({
      ...i,
      title_snapshot:
        typeof i.title_snapshot === 'object' && i.title_snapshot !== null
          ? locale === 'zh-CN'
            ? (i.title_snapshot as { zhCN: string }).zhCN
            : (i.title_snapshot as { enUS: string }).enUS
          : i.title_snapshot,
    })),
  }
}

/* ------------------------------------------------------------------ */
/* Lookups                                                             */
/* ------------------------------------------------------------------ */

function localeOf(opts?: CallOptions): MockLocale {
  const l = String(opts?.params?.locale ?? 'en-US')
  return l === 'zh-CN' ? 'zh-CN' : 'en-US'
}

function currencyOf(opts?: CallOptions): Currency | undefined {
  const c = String(opts?.params?.currency ?? '')
  return c === 'USD' || c === 'EUR' || c === 'GBP' ? c : undefined
}

function authUser(opts?: CallOptions): (typeof DEMO_USERS)[number] {
  const token = opts?.token
  const userId = token ? sessions.get(token) : undefined
  if (!userId) {
    throw new ApiError('unauthorized', 'token not found or expired', 401)
  }
  const found = DEMO_USERS.find((u) => u.id === userId)
  if (!found) {
    throw new ApiError('unauthorized', 'token not found or expired', 401)
  }
  return found
}

/** Auto-generate a demo itinerary quote so the "pay deposit" flow is
 *  exercisable without waiting for a planner. The real backend's planner
 *  sends the quote; the mock flips status to 'quoted' immediately. */
function makeDemoQuote(req: ItineraryRequest, loc: MockLocale): ItineraryQuote {
  const totalCny = 880000 // ¥8,800
  const cur = req.budget?.currency ?? 'USD'
  const totalMinor = convertMinor(totalCny, cur)
  const depositMinor = Math.ceil(totalMinor * 0.3)
  const lineItems: QuoteLineItem[] = [
    {
      label: loc === 'zh-CN' ? '5日行程设计与全程陪同' : '5-day itinerary design + full escort',
      amount_minor: totalMinor,
      amount: totalMinor,
    },
  ]
  return {
    id: idSeq.quote++,
    request_id: req.id,
    line_items: lineItems,
    total_cny: totalCny,
    currency: cur,
    total_minor: totalMinor,
    deposit_minor: depositMinor,
    fx_rate_used: effectiveRate(cur),
    status: 'sent',
    sent_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  }
}

function findSku(id: number): { sku: ProductRecord['skus'][number]; product: ProductRecord } {
  for (const p of PRODUCTS) {
    const sku = p.skus.find((s) => s.id === id)
    if (sku) return { sku, product: p }
  }
  throw new ApiError('not_found', 'requested resource not found', 404)
}

/* ------------------------------------------------------------------ */
/* Mappers (record → API DTO, per-locale)                              */
/* ------------------------------------------------------------------ */

function toSKU(
  sku: ProductRecord['skus'][number],
  productId: number,
  locale: MockLocale,
  currency?: Currency,
): SKU {
  return {
    id: sku.id,
    product_id: productId,
    sku_code: sku.skuCode,
    price_cny: sku.priceCny,
    stock: sku.stock,
    weight_grams: sku.weightGrams,
    low_stock_threshold: sku.lowStockThreshold,
    attributes: pick(sku.attributes, locale),
    is_active: true,
    ...(currency ? { price: convertMinor(sku.priceCny, currency), price_currency: currency } : {}),
  }
}

function toProduct(
  rec: ProductRecord,
  locale: MockLocale,
  currency?: Currency,
  detail = false,
): Product {
  const tr = pick(rec.translations, locale)
  const other = locale === 'zh-CN' ? rec.translations.enUS : rec.translations.zhCN
  const artist = ARTISTS.find((a) => a.id === rec.artistId)
  const tags: Tag[] = rec.tags
    .map((key) => {
      const t = TAGS.find((x) => x.key === key)
      return t ? { id: t.id, key: t.key, name: pick(t.translations, locale) } : null
    })
    .filter((x): x is Tag => x !== null)

  const product: Product = {
    id: rec.id,
    artist_id: rec.artistId,
    artist_name: artist ? pick(artist.translations, locale).name : undefined,
    artist_slug: artist ? artist.translations.enUS.slug : undefined,
    figure_seed: rec.figureSeed,
    figure_kind: rec.figureKind,
    title: tr.title,
    slug: tr.slug,
    description: tr.description,
    meta_title: tr.metaTitle,
    meta_description: tr.metaDescription,
    locale,
    status: 'published',
    published_at: rec.publishedAt,
    tags,
    cert_code: CERTIFICATES.find((c) => c.product_id === rec.id)?.cert_code,
    alternates: detail ? { [locale === 'zh-CN' ? 'en-US' : 'zh-CN']: other.slug } : undefined,
    created_at: rec.createdAt,
    updated_at: rec.publishedAt,
  }

  // SKUs are attached on list + detail (the crate/gallery cards need
  // presentment prices without a detail round-trip); gallery stays
  // detail-only.
  product.skus = rec.skus.map((s) => toSKU(s, rec.id, locale, currency))
  if (detail) {
    product.gallery = [0, 1, 2].map((i) => ({
      media_id: rec.id * 10 + i,
      public_url: `mock://media/product/${rec.id}/${i}`,
      caption: i === 0 ? tr.title : undefined,
      figure_seed: rec.figureSeed + i * 97,
      figure_kind: rec.figureKind,
    }))
  }
  return product
}

function toArtist(rec: (typeof ARTISTS)[number], locale: MockLocale, detail = false): Artist {
  const tr = pick(rec.translations, locale)
  const other = locale === 'zh-CN' ? rec.translations.enUS : rec.translations.zhCN
  const a: Artist = {
    id: rec.id,
    glyph: rec.glyph,
    name: tr.name,
    slug: tr.slug,
    bio: tr.bio,
    locale,
    status: 'published',
    alternates: detail ? { [locale === 'zh-CN' ? 'en-US' : 'zh-CN']: other.slug } : undefined,
  }
  if (detail) {
    const works = PRODUCTS.filter((p) => p.artistId === rec.id)
    a.gallery = works.slice(0, 3).map((w, i) => ({
      media_id: rec.id * 100 + i,
      public_url: `mock://media/artist/${rec.id}/${i}`,
      figure_seed: w.figureSeed,
      figure_kind: w.figureKind,
    }))
  }
  return a
}

function toStory(rec: (typeof STORIES)[number], locale: MockLocale, detail = false): CeramicStory {
  const tr = pick(rec.translations, locale)
  const other = locale === 'zh-CN' ? rec.translations.enUS : rec.translations.zhCN
  const s: CeramicStory = {
    id: rec.id,
    dynasty_start_year: rec.startYear,
    figure_seed: rec.figureSeed,
    title: tr.title,
    slug: tr.slug,
    summary: tr.summary,
    content: detail ? tr.content : [],
    locale,
    status: 'published',
    alternates: detail ? { [locale === 'zh-CN' ? 'en-US' : 'zh-CN']: other.slug } : undefined,
  }
  return s
}

function toActivity(
  rec: (typeof ACTIVITIES)[number],
  locale: MockLocale,
  detail = false,
): Activity {
  const tr = pick(rec.translations, locale)
  const other = locale === 'zh-CN' ? rec.translations.enUS : rec.translations.zhCN
  const a: Activity = {
    id: rec.id,
    type: rec.type,
    lat: rec.lat,
    lng: rec.lng,
    address: rec.address ? pick(rec.address, locale) : undefined,
    opening_info: rec.opening ? pick(rec.opening, locale) : undefined,
    figure_seed: rec.figureSeed,
    title: tr.title,
    slug: tr.slug,
    summary: tr.summary,
    content: detail ? tr.content : [],
    locale,
    status: 'published',
    alternates: detail ? { [locale === 'zh-CN' ? 'en-US' : 'zh-CN']: other.slug } : undefined,
  }
  if (detail) {
    a.gallery = [0, 1].map((i) => ({
      media_id: rec.id * 100 + i,
      public_url: `mock://media/activity/${rec.id}/${i}`,
      figure_seed: rec.figureSeed + i * 53,
      figure_kind: 'vase' as const,
    }))
  }
  return a
}

/* ------------------------------------------------------------------ */
/* Cart                                                                */
/* ------------------------------------------------------------------ */

function cartKey(opts?: CallOptions): { kind: 'user'; id: string } | { kind: 'guest'; id: string } {
  if (opts?.token && sessions.has(opts.token)) {
    return { kind: 'user', id: sessions.get(opts.token)! }
  }
  const g = opts?.guestId
  if (!g) throw new ApiError('unauthorized', 'no session', 401)
  return { kind: 'guest', id: g }
}

function getLines(key: ReturnType<typeof cartKey>): CartLine[] {
  const m = key.kind === 'user' ? userCarts : guestCarts
  if (!m.has(key.id)) m.set(key.id, [])
  return m.get(key.id)!
}

function buildCart(opts?: CallOptions): Cart {
  const locale = localeOf(opts)
  const currency = currencyOf(opts)
  const lines = getLines(cartKey(opts))
  const items: CartItem[] = lines.map((line) => {
    const { sku, product } = findSku(line.sku_id)
    const tr = pick(product.translations, locale)
    const artist = ARTISTS.find((a) => a.id === product.artistId)
    const item: CartItem = {
      sku_id: sku.id,
      sku_code: sku.skuCode,
      qty: line.qty,
      unit_price_cny: sku.priceCny,
      line_total_cny: sku.priceCny * line.qty,
      stock: sku.stock,
      weight_grams: sku.weightGrams,
      product_id: product.id,
      product_slug: tr.slug,
      product_title: tr.title,
      figure_seed: product.figureSeed,
      figure_kind: product.figureKind,
      artist_name: artist ? pick(artist.translations, locale).name : undefined,
      attributes: pick(sku.attributes, locale),
      added_at: line.added_at,
    }
    if (currency) {
      item.unit_price = convertMinor(sku.priceCny, currency)
      item.line_total = item.unit_price * line.qty // presentment × qty (integer minor units)
    }
    return item
  })
  const totalCny = items.reduce((s, i) => s + i.line_total_cny, 0)
  return {
    items,
    item_count: items.length,
    total_cny: totalCny,
    ...(currency ? { total: convertMinor(totalCny, currency), currency } : {}),
  }
}

/* ------------------------------------------------------------------ */
/* Transport                                                           */
/* ------------------------------------------------------------------ */

const delay = () => new Promise((r) => setTimeout(r, 90 + Math.random() * 160))

export const mockTransport: Transport = {
  async call<T>(method: string, path: string, opts: CallOptions = {}): Promise<T> {
    await delay()
    const locale = localeOf(opts)
    const currency = currencyOf(opts)
    const p = method === 'GET' ? path : path.replace(/\/+$/, '')
    const route = `${method} ${p}`

    try {
      const result = (await handle(route, { method, path: p, opts, locale, currency })) as T
      if (method !== 'GET') persist()
      return result
    } catch (e) {
      // ApiError throws (e.g. 2FA challenge) mutate Maps too — persist so
      // the pending token survives HMR module re-evaluation and reloads.
      if (e instanceof ApiError) {
        if (method !== 'GET') persist()
        throw e
      }
      throw new ApiError('internal', 'mock transport failure', 500)
    }
  },
}

interface Ctx {
  method: string
  path: string
  opts: CallOptions
  locale: MockLocale
  currency?: Currency
}

async function handle(route: string, ctx: Ctx): Promise<unknown> {
  const { opts, locale, currency } = ctx
  const body = (opts.body ?? {}) as Record<string, unknown>

  /* ---------------- public catalog ---------------- */

  if (route === 'GET /catalog/products') {
    const q = opts.params ?? {}
    let list = PRODUCTS.map((r) => toProduct(r, locale, currency))

    if (q.tag) list = list.filter((pr) => pr.tags?.some((t) => t.key === q.tag))
    if (q.artist) {
      const artistRec = ARTISTS.find(
        (a) => a.translations.enUS.slug === q.artist || a.translations.zhCN.slug === q.artist,
      )
      if (artistRec) list = list.filter((pr) => pr.artist_id === artistRec.id)
    }
    if (q.edition) {
      // edition_type lives on SKU attributes (PRD §3.2.1)
      list = list.filter((pr) => {
        const r = PRODUCTS.find((x) => x.id === pr.id)!
        return r.skus.some((s) => s.attributes.enUS.edition_type === q.edition)
      })
    }
    if (q.priceBand) {
      // band thresholds authored in USD majors; compare in presentment USD
      const band = String(q.priceBand)
      const conv = (usd: number) => convertMinor(usd * 100, 'USD')
      list = list.filter((pr) => {
        const r = PRODUCTS.find((x) => x.id === pr.id)!
        const lo = r.skus.reduce((m, s) => Math.min(m, s.priceCny), Infinity)
        const price = convertMinor(lo, 'USD')
        if (band === 'low') return price < conv(500)
        if (band === 'mid') return price >= conv(500) && price < conv(1500)
        return price >= conv(1500)
      })
    }
    if (q.q) {
      const needle = String(q.q).toLowerCase()
      list = list.filter((pr) => {
        const r = PRODUCTS.find((x) => x.id === pr.id)!
        return (
          pr.title.toLowerCase().includes(needle) ||
          r.translations.enUS.title.toLowerCase().includes(needle) ||
          r.translations.zhCN.title.includes(String(q.q)) ||
          (pr.artist_name?.toLowerCase().includes(needle) ?? false)
        )
      })
    }

    const sort = String(q.sort ?? 'featured')
    const priceOf = (pr: Product) => {
      const r = PRODUCTS.find((x) => x.id === pr.id)!
      return r.skus.reduce((m, s) => Math.min(m, s.priceCny), Infinity)
    }
    if (sort === 'price_asc') list.sort((a, b) => priceOf(a) - priceOf(b))
    else if (sort === 'price_desc') list.sort((a, b) => priceOf(b) - priceOf(a))
    else if (sort === 'newest') list.sort((a, b) => b.created_at.localeCompare(a.created_at))
    else
      list.sort((a, b) => {
        const ra = PRODUCTS.find((x) => x.id === a.id)!
        const rb = PRODUCTS.find((x) => x.id === b.id)!
        return Number(rb.featured ?? false) - Number(ra.featured ?? false) || ra.id - rb.id
      })

    const page = Math.max(1, Number(q.page ?? 1))
    const limit = Math.min(48, Math.max(1, Number(q.limit ?? 9)))
    const total = list.length
    const slice = list.slice((page - 1) * limit, page * limit)
    return { data: slice, page, limit, total, total_pages: Math.ceil(total / limit) || 1 }
  }

  if (ctx.method === 'GET' && pathRegex('/catalog/products/:slug', ctx.path)) {
    const slug = ctx.path.split('/').pop()!
    const rec = PRODUCTS.find(
      (r) => r.translations[locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'requested resource not found', 404)
    return toProduct(rec, locale, currency, true)
  }

  if (route === 'GET /catalog/tags') {
    return TAGS.map((t) => {
      const count = PRODUCTS.filter((p) => p.tags.includes(t.key)).length
      return { id: t.id, key: t.key, name: pick(t.translations, locale), product_count: count }
    }).filter((t) => t.product_count > 0)
  }

  if (route === 'GET /artists') {
    // real contract: PaginatedResponse-wrapped
    const list = ARTISTS.map((a) => toArtist(a, locale))
    return { data: list, page: 1, limit: 20, total: list.length, total_pages: 1 }
  }
  if (ctx.method === 'GET' && pathRegex('/artists/:slug', ctx.path)) {
    const slug = ctx.path.split('/').pop()!
    const rec = ARTISTS.find(
      (a) => a.translations[locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'requested resource not found', 404)
    return toArtist(rec, locale, true)
  }

  if (route === 'GET /ceramicstory')
    return STORIES.map((s) => toStory(s, locale)).sort(
      (a, b) => a.dynasty_start_year - b.dynasty_start_year,
    )
  if (ctx.method === 'GET' && pathRegex('/ceramicstory/:slug', ctx.path)) {
    const slug = ctx.path.split('/').pop()!
    const rec = STORIES.find(
      (s) => s.translations[locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'requested resource not found', 404)
    return toStory(rec, locale, true)
  }

  if (route === 'GET /engage') {
    // real contract: PaginatedResponse-wrapped
    let list = ACTIVITIES.map((a) => toActivity(a, locale))
    if (opts.params?.type) list = list.filter((a) => a.type === opts.params!.type)
    return { data: list, page: 1, limit: 20, total: list.length, total_pages: 1 }
  }
  if (ctx.method === 'GET' && pathRegex('/engage/:slug', ctx.path)) {
    const slug = ctx.path.split('/').pop()!
    const rec = ACTIVITIES.find(
      (a) => a.translations[locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'requested resource not found', 404)
    return toActivity(rec, locale, true)
  }

  if (ctx.method === 'GET' && pathRegex('/certificates/:code', ctx.path)) {
    const code = ctx.path.split('/').pop()!
    const cert = CERTIFICATES.find((c) => c.cert_code.toLowerCase() === code.toLowerCase())
    if (!cert) throw new ApiError('not_found', 'requested resource not found', 404)
    const rec = PRODUCTS.find((p) => p.id === cert.product_id)!
    const artistRec = ARTISTS.find((a) => a.id === rec.artistId)!
    const sku = rec.skus[0]
    const dto: Certificate = {
      id: cert.id,
      product_id: cert.product_id,
      cert_code: cert.cert_code,
      issued_at: cert.issued_at,
      product_title: pick(rec.translations, locale).title,
      product_slug: rec.translations[locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug,
      artist_name: pick(artistRec.translations, locale).name,
      figure_seed: rec.figureSeed,
      figure_kind: rec.figureKind,
      attributes: sku ? pick(sku.attributes, locale) : undefined,
      provenance: cert.provenance.map((pv) => ({ ...pv })),
    }
    return dto
  }

  if (route === 'GET /shipping/quote') {
    const country = String(opts.params?.country ?? '')
    const weight = Number(opts.params?.weight ?? 0)
    const q = quoteShipping(country, weight)
    return {
      country,
      weight_grams: weight,
      fee_cny: q.fee_cny,
      blocked_reason: q.blocked_reason,
      ...(currency && !q.blocked_reason
        ? { fee: convertMinor(q.fee_cny, currency), currency }
        : {}),
    }
  }

  /* ---------------- auth ---------------- */

  if (route === 'POST /auth/login') {
    const email = String(body.email ?? '').toLowerCase()
    const password = String(body.password ?? '')
    const found = DEMO_USERS.find((u) => u.email === email)
    // demo affordance: the literal password 'enroll-me' triggers the
    // must-enroll challenge; the REAL password is still required at the
    // pending-enroll step on the enroll page
    const demoEnroll = found !== undefined && password === 'enroll-me'
    if (!found || (found.password !== password && !demoEnroll)) {
      throw new ApiError('invalid_credentials', 'Invalid email or password', 401)
    }
    if (demoEnroll || found.twoFA) {
      const code = demoEnroll ? '2fa_enrollment_required' : '2fa_required'
      // real contract: 401 {"error":{code:"2fa_required", message, pending_token}}
      const pending = `pending_${found.id}_${idSeq.token++}`
      pending2FA.set(pending, { userId: found.id, fails: 0 })
      throw new ApiError(
        code,
        code === '2fa_required'
          ? 'two-factor authentication code required'
          : 'two-factor enrollment required',
        401,
        undefined,
        pending,
      )
    }
    const token = `tok_${found.id}_${idSeq.token++}`
    sessions.set(token, found.id)
    return { access_token: token, user: found.user }
  }

  if (route === 'POST /auth/2fa/enroll-portal') {
    // unused placeholder guard (kept for readability)
  }

  if (route === 'POST /auth/activate') {
    const token = String(body.token ?? '')
    const userId = activationTokens.get(token)
    if (!userId) throw new ApiError('unauthorized', 'token not found or expired', 401)
    activationTokens.delete(token)
    const found = DEMO_USERS.find((u) => u.id === userId)!
    const tok = `tok_${found.id}_${idSeq.token++}`
    sessions.set(tok, found.id)
    return { access_token: tok, user: found.user }
  }

  if (route === 'POST /auth/resend-activation') {
    const email = String(body.email ?? '').toLowerCase()
    const found = DEMO_USERS.find((u) => u.email === email)
    if (found) {
      const act = (found as { activationToken?: string }).activationToken
      if (act) activationTokens.delete(act)
      const fresh = `activate_${found.id}_${idSeq.token++}`
      activationTokens.set(fresh, found.id)
      ;(found as { activationToken?: string }).activationToken = fresh
    }
    return { message: 'If the address exists, an activation email has been sent.' }
  }

  if (route === 'POST /auth/request-password-reset') {
    const email = String(body.email ?? '').toLowerCase()
    const found = DEMO_USERS.find((u) => u.email === email)
    if (found) {
      const tok = `reset_${found.id}_${idSeq.token++}`
      resetTokens.set(tok, found.id)
      ;(found as { resetToken?: string }).resetToken = tok
      return {
        message: 'If the address exists, a password reset link has been sent.',
        reset_token: tok, // mock-only dev affordance
      }
    }
    return { message: 'If the address exists, a password reset link has been sent.' }
  }

  if (route === 'POST /auth/reset-password') {
    const token = String(body.token ?? '')
    const userId = resetTokens.get(token)
    if (!userId) throw new ApiError('unauthorized', 'token not found or expired', 401)
    resetTokens.delete(token)
    const found = DEMO_USERS.find((u) => u.id === userId)!
    found.password = String(body.new_password ?? '')
    const tok = `tok_${found.id}_${idSeq.token++}`
    sessions.set(tok, found.id)
    return { access_token: tok, user: found.user }
  }

  if (route === 'POST /auth/2fa/pending-enroll') {
    const pending = String(body.pending_token ?? '')
    const entry = pending2FA.get(pending)
    if (!entry) throw new ApiError('unauthorized', 'token not found or expired', 401)
    const found = DEMO_USERS.find((u) => u.id === entry.userId)!
    if (found.password !== String(body.password ?? '')) {
      throw new ApiError('invalid_credentials', 'invalid credentials', 401)
    }
    const secret = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP'
    return {
      otpauth_url: `otpauth://totp/JDZ:${found.email}?secret=${secret}&issuer=Jingdezhen`,
      secret,
    }
  }

  if (route === 'POST /auth/2fa/pending-confirm') {
    const pending = String(body.pending_token ?? '')
    const entry = pending2FA.get(pending)
    if (!entry) throw new ApiError('unauthorized', 'token not found or expired', 401)
    if (String(body.code ?? '') !== '123456') {
      throw new ApiError('invalid_credentials', 'invalid credentials', 401)
    }
    pending2FA.delete(pending)
    const found = DEMO_USERS.find((u) => u.id === entry.userId)!
    found.twoFA = true
    found.demoCode = '123456'
    const tok = `tok_${found.id}_${idSeq.token++}`
    sessions.set(tok, found.id)
    const backup = Array.from({ length: 5 }, (_, i) => `${found.id.slice(-4)}-${1000 + i}`)
    return { access_token: tok, user: found.user, backup_codes: backup }
  }

  if (route === 'POST /auth/2fa/verify') {
    const pending = String(body.pending_token ?? '')
    const code = String(body.code ?? '')
    const entry = pending2FA.get(pending)
    if (!entry) throw new ApiError('unauthorized', 'token not found or expired', 401)
    if (entry.fails >= 5)
      throw new ApiError('too_many_attempts', 'too many failed attempts, try again later', 429)
    const found = DEMO_USERS.find((u) => u.id === entry.userId)!
    if (code !== found.demoCode) {
      entry.fails += 1
      throw new ApiError('invalid_credentials', 'invalid credentials', 401)
    }
    pending2FA.delete(pending)
    const token = `tok_${found.id}_${idSeq.token++}`
    sessions.set(token, found.id)
    return { access_token: token, user: found.user }
  }

  if (route === 'POST /auth/signup') {
    const email = String(body.email ?? '').toLowerCase()
    if (!email.includes('@')) {
      throw new ApiError('validation_failed', 'invalid email', 422, { email: 'errors.required' })
    }
    if (String(body.password ?? '').length < 8) {
      throw new ApiError('validation_failed', 'password too short', 422, {
        password: 'errors.passwordShort',
      })
    }
    if (DEMO_USERS.some((u) => u.email === email)) {
      throw new ApiError('conflict', 'resource conflict, item already exists', 409)
    }
    const user: User = {
      id: `u_${Math.random().toString(36).slice(2, 6)}`,
      email,
      nickname: String(body.nickname ?? email.split('@')[0]),
      avatar_glyph: String(body.nickname ?? 'U')
        .slice(0, 1)
        .toUpperCase(),
      role: 'customer',
      preferred_locale: locale,
      preferred_currency: 'USD',
      created_at: new Date().toISOString(),
    }
    const demo = {
      id: user.id,
      email,
      password: String(body.password),
      twoFA: false,
      demoCode: undefined,
      user,
    } as (typeof DEMO_USERS)[number]
    DEMO_USERS.push(demo)
    // real contract: 201 with an EMPTY access_token — the account is
    // inactive until the email-link activation completes
    const activationToken = `activate_${user.id}_${idSeq.token++}`
    activationTokens.set(activationToken, user.id)
    ;(demo as { activationToken?: string }).activationToken = activationToken
    return { access_token: '', user, activation_token: activationToken }
  }

  /* ---------------- profile ---------------- */

  if (route === 'GET /profile') return authUser(opts).user

  if (route === 'PUT /profile') {
    const demo = authUser(opts)
    if (body.nickname != null) demo.user.nickname = String(body.nickname)
    if (body.preferred_locale != null) demo.user.preferred_locale = String(body.preferred_locale)
    if (body.preferred_currency != null)
      demo.user.preferred_currency = String(body.preferred_currency)
    return demo.user
  }

  if (route === 'GET /profile/addresses') {
    const user = authUser(opts)
    return { data: addresses.filter((a) => a.user_id === user.id) }
  }

  if (route === 'POST /profile/addresses') {
    const user = authUser(opts)
    const addr: StoredAddress = {
      id: idSeq.address++,
      recipient: String(body.recipient ?? ''),
      line1: String(body.line1 ?? ''),
      line2: body.line2 ? String(body.line2) : undefined,
      city: String(body.city ?? ''),
      region: body.region ? String(body.region) : undefined,
      postal_code: String(body.postal_code ?? ''),
      country: String(body.country ?? ''),
      phone: String(body.phone ?? ''),
      is_default: Boolean(body.is_default),
      user_id: user.id,
    }
    if (addr.is_default) {
      for (const a of addresses) if (a.user_id === user.id) a.is_default = false
    }
    addresses.push(addr)
    return addr
  }

  /* ---------------- cart ---------------- */

  if (route === 'GET /cart') return buildCart(opts)

  if (route === 'POST /cart/items') {
    const lines = getLines(cartKey(opts))
    const skuId = Number(body.sku_id ?? 0)
    const qty = Math.max(1, Number(body.qty ?? 1))
    const { sku } = findSku(skuId)
    const existing = lines.find((l) => l.sku_id === skuId)
    const newQty = (existing?.qty ?? 0) + qty
    if (newQty > sku.stock) {
      throw new ApiError('conflict', 'insufficient stock', 409)
    }
    if (existing) existing.qty = newQty
    else lines.push({ sku_id: skuId, qty, added_at: new Date().toISOString() })
    return buildCart(opts)
  }

  if (ctx.method === 'PATCH' && pathRegex('/cart/items/:skuId', ctx.path)) {
    const lines = getLines(cartKey(opts))
    const skuId = Number(ctx.path.split('/').pop())
    const qty = Math.max(1, Number(body.qty ?? 1))
    const { sku } = findSku(skuId)
    const line = lines.find((l) => l.sku_id === skuId)
    if (!line) throw new ApiError('not_found', 'requested resource not found', 404)
    if (qty > sku.stock) throw new ApiError('conflict', 'insufficient stock', 409)
    line.qty = qty
    return buildCart(opts)
  }

  if (ctx.method === 'DELETE' && pathRegex('/cart/items/:skuId', ctx.path)) {
    const key = cartKey(opts)
    const skuId = Number(ctx.path.split('/').pop())
    const m = key.kind === 'user' ? userCarts : guestCarts
    m.set(
      key.id,
      getLines(key).filter((l) => l.sku_id !== skuId),
    )
    return buildCart(opts)
  }

  if (route === 'DELETE /cart/items') {
    const key = cartKey(opts)
    const ids = (body.sku_ids ?? []) as number[]
    const m = key.kind === 'user' ? userCarts : guestCarts
    m.set(
      key.id,
      getLines(key).filter((l) => !ids.includes(l.sku_id)),
    )
    return buildCart(opts)
  }

  if (route === 'POST /cart/merge') {
    const user = authUser(opts)
    const guestId = opts.guestId
    // the guest cart arrives in the body (PRD §3.2.3 merge contract)
    const items = (body.items ?? []) as Array<{ sku_id: number; qty: number; added_at?: string }>
    const userLines = getLines({ kind: 'user', id: user.id })
    for (const g of items) {
      const { sku } = findSku(g.sku_id)
      const existing = userLines.find((l) => l.sku_id === g.sku_id)
      const merged = Math.min(sku.stock, (existing?.qty ?? 0) + g.qty)
      if (existing) existing.qty = merged
      else
        userLines.push({
          sku_id: g.sku_id,
          qty: merged,
          added_at: g.added_at ?? new Date().toISOString(),
        })
    }
    if (guestId) guestCarts.delete(guestId)
    return buildCart(opts)
  }

  /* ---------------- wishlist ---------------- */

  if (route === 'GET /wishlist') {
    const user = authUser(opts)
    const ids = wishlists.get(user.id) ?? []
    const out: WishlistItem[] = ids.flatMap((skuId) => {
      try {
        const { sku, product } = findSku(skuId)
        const tr = pick(product.translations, locale)
        const artistRec = ARTISTS.find((a) => a.id === product.artistId)
        return [
          {
            sku_id: skuId,
            added_at: new Date().toISOString(),
            product_id: product.id,
            product_slug: tr.slug,
            product_title: tr.title,
            artist_name: artistRec ? pick(artistRec.translations, locale).name : undefined,
            figure_seed: product.figureSeed,
            figure_kind: product.figureKind,
            stock: sku.stock,
            ...(currency
              ? { price: convertMinor(sku.priceCny, currency), price_currency: currency }
              : {}),
          },
        ]
      } catch {
        return []
      }
    })
    return out
  }

  if (route === 'POST /wishlist') {
    const user = authUser(opts)
    const skuId = Number(body.sku_id ?? 0)
    findSku(skuId)
    const ids = wishlists.get(user.id) ?? []
    if (!ids.includes(skuId)) ids.push(skuId)
    wishlists.set(user.id, ids)
    return { ok: true }
  }

  if (ctx.method === 'DELETE' && pathRegex('/wishlist/:skuId', ctx.path)) {
    const user = authUser(opts)
    const skuId = Number(ctx.path.split('/').pop())
    wishlists.set(
      user.id,
      (wishlists.get(user.id) ?? []).filter((id) => id !== skuId),
    )
    return { ok: true }
  }

  /* ---------------- orders / checkout ---------------- */

  if (route === 'POST /checkout') {
    const user = authUser(opts)
    const key = cartKey(opts)
    const cart = buildCart(opts)
    if (cart.items.length === 0)
      throw new ApiError('cart_empty', 'cart is empty; cannot check out', 422)
    if (body.consent !== true)
      throw new ApiError('consent_required', 'privacy policy consent is required', 422)

    const addressId = Number(body.address_id ?? 0)
    const address = addresses.find((a) => a.id === addressId && a.user_id === user.id)
    if (!address) throw new ApiError('not_found', 'requested resource not found', 404)

    const cur = (String(body.currency ?? 'USD') as Currency) ?? 'USD'
    const weight = cart.items.reduce((s, i) => s + i.weight_grams * i.qty, 0)
    const q = quoteShipping(address.country, weight)
    if (q.blocked_reason === 'unshippable') {
      throw new ApiError('unshippable', 'destination country is not shippable', 422)
    }
    if (q.blocked_reason === 'overweight') {
      throw new ApiError(
        'overweight',
        'order exceeds the maximum shipping weight for the destination',
        422,
      )
    }

    // atomic stock decrement (TDD §4.3): all-or-nothing
    const decrements: Array<() => void> = []
    for (const item of cart.items) {
      const { sku } = findSku(item.sku_id)
      if (sku.stock < item.qty) throw new ApiError('conflict', 'insufficient stock', 409)
      decrements.push(() => {
        sku.stock -= item.qty
      })
    }
    decrements.forEach((d) => d())

    const subtotalCny = cart.total_cny
    const totalCny = subtotalCny + q.fee_cny
    const order: StoredOrder = {
      id: idSeq.order++,
      user_id: user.id,
      status: 'created',
      currency: cur,
      subtotal_minor: convertMinor(subtotalCny, cur),
      shipping_minor: convertMinor(q.fee_cny, cur),
      total_minor: 0,
      total_cny: totalCny,
      subtotal_cny: subtotalCny,
      shipping_cny: q.fee_cny,
      fx_rate_used: effectiveRate(cur),
      address: { ...address },
      locale: String(body.locale ?? locale),
      placed_at: new Date().toISOString(),
      items: cart.items.map((i) => {
        const rec = findSku(i.sku_id).product
        return {
          id: idSeq.item++,
          order_id: idSeq.order - 1,
          sku_id: i.sku_id,
          qty: i.qty,
          unit_price_minor: i.unit_price ?? convertMinor(i.unit_price_cny, cur),
          unit_price_cny: i.unit_price_cny,
          title_snapshot: {
            enUS: rec.translations.enUS.title,
            zhCN: rec.translations.zhCN.title,
          },
          attributes_snapshot: i.attributes,
          figure_seed: i.figure_seed,
          figure_kind: i.figure_kind,
        }
      }),
      hosted_url: 'mock://sandbox-checkout',
    }
    order.total_minor = order.subtotal_minor + order.shipping_minor
    orders.unshift(order)

    const m = key.kind === 'user' ? userCarts : guestCarts
    m.set(key.id, [])
    return presentOrder(order, locale)
  }

  /** mock-only: the sandbox gateway webhook → order created → paid. */
  if (ctx.method === 'POST' && pathRegex('/mock/pay/:id', ctx.path)) {
    const user = authUser(opts)
    const order = orders.find((o) => o.id === Number(ctx.path.split('/').pop()))
    if (!order || order.user_id !== user.id)
      throw new ApiError('not_found', 'requested resource not found', 404)
    if (order.status === 'created') {
      order.status = 'paid'
      order.paid_at = new Date().toISOString()
      // provenance: sold (certificates of the purchased works)
      for (const item of order.items ?? []) {
        const { product } = findSku(item.sku_id)
        const cert = CERTIFICATES.find((c) => c.product_id === product.id)
        if (cert) {
          cert.provenance.push({
            id: cert.provenance.length + 1,
            kind: 'sold',
            detail: `Sold via Jingdezhen Ceramics Platform · Order #${order.id}`,
            at: order.paid_at,
          })
        }
      }
    }
    return presentOrder(order, locale)
  }

  if (route === 'GET /orders') {
    const user = authUser(opts)
    return orders.filter((o) => o.user_id === user.id).map((o) => presentOrder(o, locale))
  }

  if (ctx.method === 'GET' && pathRegex('/orders/:id', ctx.path)) {
    const user = authUser(opts)
    const order = orders.find((o) => o.id === Number(ctx.path.split('/').pop()))
    if (!order || order.user_id !== user.id)
      throw new ApiError('not_found', 'requested resource not found', 404)
    return presentOrder(order, locale)
  }

  if (ctx.method === 'POST' && pathRegex('/orders/:id/cancel', ctx.path)) {
    const user = authUser(opts)
    const order = orders.find((o) => o.id === Number(ctx.path.split('/').pop()))
    if (!order || order.user_id !== user.id)
      throw new ApiError('not_found', 'requested resource not found', 404)
    if (order.status !== 'created') {
      throw new ApiError(
        'invalid_operation',
        'the requested operation is not valid for the target resource',
        409,
      )
    }
    order.status = 'cancelled' as OrderStatus
    order.cancelled_at = new Date().toISOString()
    order.cancel_reason = body.reason ? String(body.reason) : undefined
    for (const item of order.items ?? []) {
      try {
        const { sku } = findSku(item.sku_id)
        sku.stock += item.qty
      } catch {
        /* product removed */
      }
    }
    return presentOrder(order, locale)
  }

  /* ---------------- itineraries ---------------- */

  if (route === 'GET /itineraries') {
    const user = authUser(opts)
    const list = itineraries.filter((r) => r.user_id === user.id)
    return { data: list, page: 1, limit: 20, total: list.length, total_pages: 1 }
  }

  if (ctx.method === 'GET' && pathRegex('/itineraries/:id', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const req = itineraries.find((r) => r.id === id && r.user_id === user.id)
    if (!req) throw new ApiError('not_found', 'requested resource not found', 404)
    const quote = itineraryQuotes.get(id)
    return { ...req, quote }
  }

  if (route === 'POST /itineraries') {
    const user = authUser(opts)
    if (body.consent !== true)
      throw new ApiError('consent_required', 'privacy policy consent is required', 422)
    if (!body.arrival_date || !body.duration_days || !body.adults) {
      throw new ApiError('validation_failed', 'missing trip basics', 422, {
        arrival_date: 'errors.required',
      })
    }
    const now = new Date()
    const sla = new Date(now.getTime() + 24 * 3600 * 1000)
    const req: ItineraryRequest = {
      id: idSeq.itinerary++,
      user_id: user.id,
      status: 'pending',
      arrival_date: String(body.arrival_date),
      duration_days: Number(body.duration_days),
      flexible: Boolean(body.flexible),
      adults: Number(body.adults),
      children: Number(body.children ?? 0),
      interests: (body.interests ?? []) as string[],
      budget: body.budget as ItineraryRequest['budget'],
      pace: (body.pace ?? 'balanced') as ItineraryRequest['pace'],
      services: body.services as ItineraryRequest['services'],
      contact: body.contact as ItineraryRequest['contact'],
      locale: locale,
      sla_deadline: sla.toISOString(),
      submitted_at: now.toISOString(),
    }
    itineraries.unshift(req)
    // demo: auto-generate a quote and flip to 'quoted' so the pay-deposit
    // flow is exercisable immediately (real backend: planner sends later)
    const quote = makeDemoQuote(req, locale)
    itineraryQuotes.set(req.id, quote)
    req.status = 'quoted' as ItineraryStatus
    return req
  }

  if (route === 'GET /itineraries/draft') {
    const user = authUser(opts)
    const draft = itineraryDrafts.get(user.id)
    if (!draft) throw new ApiError('not_found', 'no draft', 404)
    return draft as unknown
  }

  if (route === 'PUT /itineraries/draft') {
    const user = authUser(opts)
    itineraryDrafts.set(user.id, body)
    return body as unknown
  }

  if (route === 'DELETE /itineraries/draft') {
    const user = authUser(opts)
    itineraryDrafts.delete(user.id)
    return
  }

  if (ctx.method === 'GET' && pathRegex('/itineraries/:id/quote', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id && r.user_id === user.id)
    if (!req) throw new ApiError('not_found', 'requested resource not found', 404)
    const quote = itineraryQuotes.get(id)
    if (!quote) throw new ApiError('not_found', 'quote not yet generated', 404)
    return quote
  }

  if (ctx.method === 'POST' && pathRegex('/itineraries/:id/pay-deposit', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id && r.user_id === user.id)
    if (!req) throw new ApiError('not_found', 'requested resource not found', 404)
    const quote = itineraryQuotes.get(id)
    if (!quote || req.status !== 'quoted')
      throw new ApiError('invalid_operation', 'quote is not available for payment', 409)
    req.status = 'deposit_paid' as ItineraryStatus
    quote.status = 'deposit_paid'
    quote.paid_at = new Date().toISOString()
    return {
      quote_id: quote.id,
      hosted_url: 'mock://sandbox-deposit',
    } satisfies DepositPaidResponse
  }

  if (ctx.method === 'POST' && pathRegex('/itineraries/:id/cancel', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id && r.user_id === user.id)
    if (!req) throw new ApiError('not_found', 'requested resource not found', 404)
    if (req.status === 'deposit_paid' || req.status === 'confirmed') {
      throw new ApiError('invalid_operation', 'cannot cancel after deposit', 409)
    }
    req.status = 'cancelled' as ItineraryStatus
    return
  }

  /* ---------------- address CRUD ---------------- */

  if (ctx.method === 'GET' && pathRegex('/profile/addresses/:id', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const addr = addresses.find((a) => a.id === id && a.user_id === user.id)
    if (!addr) throw new ApiError('not_found', 'requested resource not found', 404)
    return addr
  }

  if (ctx.method === 'PUT' && pathRegex('/profile/addresses/:id', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const addr = addresses.find((a) => a.id === id && a.user_id === user.id)
    if (!addr) throw new ApiError('not_found', 'requested resource not found', 404)
    if (body.recipient != null) addr.recipient = String(body.recipient)
    if (body.line1 != null) addr.line1 = String(body.line1)
    if (body.line2 !== undefined) addr.line2 = body.line2 ? String(body.line2) : undefined
    if (body.city != null) addr.city = String(body.city)
    if (body.region !== undefined) addr.region = body.region ? String(body.region) : undefined
    if (body.postal_code != null) addr.postal_code = String(body.postal_code)
    if (body.country != null) addr.country = String(body.country)
    if (body.phone != null) addr.phone = String(body.phone)
    if (body.is_default) {
      for (const a of addresses) if (a.user_id === user.id) a.is_default = false
      addr.is_default = true
    }
    return addr
  }

  if (ctx.method === 'DELETE' && pathRegex('/profile/addresses/:id', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const idx = addresses.findIndex((a) => a.id === id && a.user_id === user.id)
    if (idx < 0) throw new ApiError('not_found', 'requested resource not found', 404)
    addresses.splice(idx, 1)
    return
  }

  if (ctx.method === 'POST' && pathRegex('/profile/addresses/:id/default', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const addr = addresses.find((a) => a.id === id && a.user_id === user.id)
    if (!addr) throw new ApiError('not_found', 'requested resource not found', 404)
    for (const a of addresses) if (a.user_id === user.id) a.is_default = false
    addr.is_default = true
    return { message: 'Default address updated' }
  }

  /* ---------------- notifications ---------------- */

  if (route === 'GET /notifications') {
    const user = authUser(opts)
    const list = notifications.filter((n) => n.recipient_user_id === user.id)
    return { data: list, page: 1, limit: 20, total: list.length, total_pages: 1 }
  }

  if (route === 'GET /notifications/unread-count') {
    const user = authUser(opts)
    const count = notifications.filter((n) => n.recipient_user_id === user.id && !n.is_read).length
    return { count }
  }

  if (route === 'POST /notifications/mark-all-read') {
    const user = authUser(opts)
    for (const n of notifications) if (n.recipient_user_id === user.id) n.is_read = true
    return
  }

  if (ctx.method === 'POST' && pathRegex('/notifications/:id/mark-read', ctx.path)) {
    const user = authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const n = notifications.find((x) => x.notification_id === id && x.recipient_user_id === user.id)
    if (!n) throw new ApiError('not_found', 'requested resource not found', 404)
    n.is_read = true
    return
  }

  /* ---------------- consent ---------------- */

  if (route === 'POST /consent') {
    const rec: ConsentRecord = {
      id: idSeq.consent++,
      user_id: opts.token ? authUser(opts).id : undefined,
      kind: String(body.kind ?? 'cookie_analytics') as ConsentKind,
      doc_version: String(body.doc_version ?? '1.0'),
      granted: Boolean(body.granted),
      created_at: new Date().toISOString(),
    }
    consentRecords.unshift(rec)
    return rec
  }

  if (route === 'GET /profile/consent') {
    const user = authUser(opts)
    return { data: consentRecords.filter((r) => r.user_id === user.id) }
  }

  if (ctx.method === 'GET' && pathRegex('/profile/consent/:kind', ctx.path)) {
    const user = authUser(opts)
    const kind = ctx.path.split('/').pop() as ConsentKind
    const rec = consentRecords.find((r) => r.user_id === user.id && r.kind === kind)
    if (!rec) return { kind, granted: false, recorded: false } satisfies ConsentState
    return {
      kind: rec.kind,
      granted: rec.granted,
      recorded: true,
      doc_version: rec.doc_version,
      created_at: rec.created_at,
    } satisfies ConsentState
  }

  /* ---------------- GDPR ---------------- */

  if (route === 'GET /profile/export') {
    const user = authUser(opts)
    const exportData: UserDataExport = {
      exported_at: new Date().toISOString(),
      user_id: user.id,
      locale,
      profile: user.user,
      addresses: addresses.filter((a) => a.user_id === user.id),
      consent_records: consentRecords.filter((r) => r.user_id === user.id),
      two_fa: user.twoFA,
    }
    return exportData
  }

  if (route === 'POST /privacy/delete-account') {
    const user = authUser(opts)
    if (String(body.confirm ?? '') !== 'DELETE')
      throw new ApiError('validation_failed', 'must confirm with "DELETE"', 422)
    // anonymize user
    user.user.email = `deleted_${user.id}@anon.local`
    user.user.nickname = 'Deleted user'
    user.user.avatar_glyph = 'D'
    // remove from sessions
    for (const [k, v] of sessions) if (v === user.id) sessions.delete(k)
    return
  }

  /* ---------------- certificates QR/PDF ---------------- */

  if (ctx.method === 'GET' && pathRegex('/certificates/:code/qr', ctx.path)) {
    // real backend returns a PNG; mock returns a data URL placeholder
    const code = ctx.path.split('/').slice(-2, -1)[0]
    const cert = CERTIFICATES.find((c) => c.cert_code === code)
    if (!cert) throw new ApiError('not_found', 'requested resource not found', 404)
    // signal to the LiveTransport that this is a media response
    return { qr_url: `/media/cert-${code}-qr.png` }
  }

  /* ---------------- analytics ---------------- */

  if (route === 'POST /analytics/events') {
    // 201 {id} on success; 204 if consent not granted (silently dropped)
    return { id: idSeq.notif++ }
  }

  /* ------------------------------ admin ------------------------------ */

  // Dashboard analytics (stubs with deterministic-ish data)
  if (route === 'GET /admin/analytics/traffic') {
    authUser(opts) // require auth
    const today = new Date()
    const data = Array.from({ length: 30 }, (_, i) => {
      const d = new Date(today)
      d.setDate(d.getDate() - (29 - i))
      return {
        date: d.toISOString().slice(0, 10),
        pageviews: Math.floor(200 + Math.random() * 800),
        unique_visitors: Math.floor(80 + Math.random() * 300),
      }
    })
    return { range: '30d', from: data[0]!.date, to: data[29]!.date, data }
  }

  if (route === 'GET /admin/analytics/sales') {
    authUser(opts)
    const today = new Date()
    const data = Array.from({ length: 30 }, (_, i) => {
      const d = new Date(today)
      d.setDate(d.getDate() - (29 - i))
      return {
        date: d.toISOString().slice(0, 10),
        orders: Math.floor(1 + Math.random() * 12),
        revenue_cny: Math.floor(100000 + Math.random() * 800000),
      }
    })
    return { range: '30d', from: data[0]!.date, to: data[29]!.date, data }
  }

  if (route === 'GET /admin/analytics/funnel') {
    authUser(opts)
    return {
      range: '30d',
      from: new Date().toISOString().slice(0, 10),
      to: new Date().toISOString().slice(0, 10),
      steps: [
        { step: 'pageview', label: 'Page views', count: 12450, rate: 100 },
        { step: 'product_view', label: 'Product views', count: 3200, rate: 25.7 },
        { step: 'cart_add', label: 'Add to cart', count: 890, rate: 7.1 },
        { step: 'checkout_start', label: 'Checkout started', count: 420, rate: 3.4 },
        { step: 'order_paid', label: 'Order paid', count: 310, rate: 2.5 },
      ],
    }
  }

  /* ---- admin: content lists (stories / activities / artists / products) ---- */

  if (route === 'GET /admin/ceramicstory') {
    authUser(opts)
    const loc = ctx.locale
    const items = STORIES.map((s) => toStory(s, loc ?? 'en-US'))
    return paginate(items, 1, 50)
  }

  if (route === 'GET /admin/engage') {
    authUser(opts)
    const loc = ctx.locale
    const items = ACTIVITIES.map((a) => toActivity(a, loc ?? 'en-US'))
    return paginate(items, 1, 50)
  }

  if (route === 'GET /admin/artists') {
    authUser(opts)
    const loc = ctx.locale
    const items = ARTISTS.map((a) => toArtist(a, loc ?? 'en-US'))
    return paginate(items, 1, 50)
  }

  if (route === 'GET /admin/products') {
    authUser(opts)
    const loc = ctx.locale
    const items = PRODUCTS.map((p) => toProduct(p, loc ?? 'en-US'))
    return paginate(items, 1, 50)
  }

  /* ---- admin: content detail by slug + workflow ---- */
  // stories
  if (ctx.method === 'GET' && pathRegex('/admin/ceramicstory/:slug', ctx.path)) {
    authUser(opts)
    const slug = ctx.path.split('/').pop()!
    const rec = STORIES.find(
      (s) => s.translations.enUS.slug === slug || s.translations.zhCN.slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'story not found', 404)
    return toStory(rec, ctx.locale, true)
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/ceramicstory/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const rec = STORIES.find((s) => s.id === id)
    if (!rec) throw new ApiError('not_found', 'story not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.title === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].title = b.title
    if (typeof b.slug === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug = b.slug
    if (typeof b.summary === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].summary = b.summary
    return toStory(rec, ctx.locale, true)
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/ceramicstory/:id', ctx.path)) {
    authUser(opts)
    return
  }
  for (const act of ['submit', 'approve', 'reject', 'unpublish'] as const) {
    if (ctx.method === 'POST' && pathRegex(`/admin/ceramicstory/:id/${act}`, ctx.path)) {
      authUser(opts)
      const id = Number(ctx.path.split('/').slice(-2, -1)[0])
      const rec = STORIES.find((s) => s.id === id)
      if (!rec) throw new ApiError('not_found', 'story not found', 404)
      return toStory(rec, ctx.locale, true)
    }
  }

  // activities (engage)
  if (ctx.method === 'GET' && pathRegex('/admin/engage/:slug', ctx.path)) {
    authUser(opts)
    const slug = ctx.path.split('/').pop()!
    const rec = ACTIVITIES.find(
      (a) => a.translations.enUS.slug === slug || a.translations.zhCN.slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'activity not found', 404)
    return toActivity(rec, ctx.locale, true)
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/engage/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const rec = ACTIVITIES.find((a) => a.id === id)
    if (!rec) throw new ApiError('not_found', 'activity not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.title === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].title = b.title
    if (typeof b.slug === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug = b.slug
    if (typeof b.summary === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].summary = b.summary
    return toActivity(rec, ctx.locale, true)
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/engage/:id', ctx.path)) {
    authUser(opts)
    return
  }
  for (const act of ['submit', 'approve', 'reject', 'unpublish'] as const) {
    if (ctx.method === 'POST' && pathRegex(`/admin/engage/:id/${act}`, ctx.path)) {
      authUser(opts)
      const id = Number(ctx.path.split('/').slice(-2, -1)[0])
      const rec = ACTIVITIES.find((a) => a.id === id)
      if (!rec) throw new ApiError('not_found', 'activity not found', 404)
      return toActivity(rec, ctx.locale, true)
    }
  }

  // artists
  if (ctx.method === 'GET' && pathRegex('/admin/artists/:slug', ctx.path)) {
    authUser(opts)
    const slug = ctx.path.split('/').pop()!
    const rec = ARTISTS.find(
      (a) => a.translations.enUS.slug === slug || a.translations.zhCN.slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'artist not found', 404)
    return toArtist(rec, ctx.locale, true)
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/artists/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const rec = ARTISTS.find((a) => a.id === id)
    if (!rec) throw new ApiError('not_found', 'artist not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.name === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].name = b.name
    if (typeof b.slug === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug = b.slug
    if (typeof b.bio === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].bio = b.bio
    return toArtist(rec, ctx.locale, true)
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/artists/:id', ctx.path)) {
    authUser(opts)
    return
  }
  for (const act of ['submit', 'approve', 'reject', 'unpublish'] as const) {
    if (ctx.method === 'POST' && pathRegex(`/admin/artists/:id/${act}`, ctx.path)) {
      authUser(opts)
      const id = Number(ctx.path.split('/').slice(-2, -1)[0])
      const rec = ARTISTS.find((a) => a.id === id)
      if (!rec) throw new ApiError('not_found', 'artist not found', 404)
      return toArtist(rec, ctx.locale, true)
    }
  }

  // products
  if (ctx.method === 'GET' && pathRegex('/admin/products/:slug', ctx.path)) {
    authUser(opts)
    const slug = ctx.path.split('/').pop()!
    const rec = PRODUCTS.find(
      (p) => p.translations.enUS.slug === slug || p.translations.zhCN.slug === slug,
    )
    if (!rec) throw new ApiError('not_found', 'product not found', 404)
    return toProduct(rec, ctx.locale, undefined, true)
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/products/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const rec = PRODUCTS.find((p) => p.id === id)
    if (!rec) throw new ApiError('not_found', 'product not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.title === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].title = b.title
    if (typeof b.slug === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].slug = b.slug
    if (typeof b.description === 'string')
      rec.translations[ctx.locale === 'zh-CN' ? 'zhCN' : 'enUS'].description = b.description
    return toProduct(rec, ctx.locale, undefined, true)
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/products/:id', ctx.path)) {
    authUser(opts)
    return
  }
  for (const act of ['submit', 'approve', 'reject', 'unpublish'] as const) {
    if (ctx.method === 'POST' && pathRegex(`/admin/products/:id/${act}`, ctx.path)) {
      authUser(opts)
      const id = Number(ctx.path.split('/').slice(-2, -1)[0])
      const rec = PRODUCTS.find((p) => p.id === id)
      if (!rec) throw new ApiError('not_found', 'product not found', 404)
      return toProduct(rec, ctx.locale, undefined, true)
    }
  }
  if (route === 'POST /admin/products') {
    authUser(opts)
    return toProduct(PRODUCTS[0]!, ctx.locale, undefined, true)
  }
  if (route === 'POST /admin/products/import') {
    authUser(opts)
    const b = body as { csv?: string; rows?: unknown[] }
    const count = b.rows?.length ?? b.csv?.split('\n').filter((l) => l.trim()).length ?? 0
    return {
      total_rows: count,
      imported: Math.max(0, count - 1),
      updated: 1,
      failed: 0,
      errors: [],
    } satisfies BulkImportSummary
  }
  if (ctx.method === 'POST' && pathRegex('/admin/products/:id/skus', ctx.path)) {
    authUser(opts)
    const productId = Number(ctx.path.split('/').slice(-2, -1)[0])
    const b = body as Record<string, unknown>
    return {
      id: Math.floor(Math.random() * 10000) + 1,
      product_id: productId,
      sku_code: String(b.sku_code ?? ''),
      price_cny: Number(b.price_cny) || 0,
      stock: Number(b.stock) || 0,
      weight_grams: Number(b.weight_grams) || 0,
      low_stock_threshold: Number(b.low_stock_threshold) || 5,
      attributes: (b.attributes as Record<string, unknown>) ?? {},
      is_active: true,
    }
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/skus/:id', ctx.path)) {
    authUser(opts)
    const b = body as Record<string, unknown>
    const id = Number(ctx.path.split('/').pop())
    return {
      id,
      product_id: 1,
      sku_code: String(b.sku_code ?? ''),
      price_cny: Number(b.price_cny) || 0,
      stock: Number(b.stock) || 0,
      weight_grams: Number(b.weight_grams) || 0,
      low_stock_threshold: Number(b.low_stock_threshold) || 5,
      attributes: (b.attributes as Record<string, unknown>) ?? {},
      is_active: true,
    }
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/skus/:id', ctx.path)) {
    authUser(opts)
    return
  }

  /* ---- admin: orders ---- */
  if (route === 'GET /admin/orders') {
    authUser(opts)
    const loc = ctx.locale
    return paginate(
      orders.map((o) => presentOrder(o, loc)),
      1,
      50,
    )
  }

  if (ctx.method === 'GET' && pathRegex('/admin/orders/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const loc = ctx.locale
    const order = orders.find((o) => o.id === id)
    if (!order) throw new ApiError('not_found', 'order not found', 404)
    return presentOrder(order, loc)
  }

  if (ctx.method === 'POST' && pathRegex('/admin/orders/:id/ship', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const b = body as { carrier_name?: string; tracking_number?: string }
    const order = orders.find((o) => o.id === id)
    if (!order) throw new ApiError('not_found', 'order not found', 404)
    order.status = 'shipped' as OrderStatus
    order.tracking_number = b.tracking_number ?? ''
    order.carrier_name = b.carrier_name ?? ''
    return presentOrder(order, ctx.locale)
  }
  if (ctx.method === 'POST' && pathRegex('/admin/orders/:id/complete', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const order = orders.find((o) => o.id === id)
    if (!order) throw new ApiError('not_found', 'order not found', 404)
    order.status = 'completed' as OrderStatus
    return presentOrder(order, ctx.locale)
  }
  if (ctx.method === 'POST' && pathRegex('/admin/orders/:id/refund', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const order = orders.find((o) => o.id === id)
    if (!order) throw new ApiError('not_found', 'order not found', 404)
    order.status = 'refunded' as OrderStatus
    return presentOrder(order, ctx.locale)
  }

  /* ---- admin: itineraries CRM ---- */
  if (route === 'GET /admin/itineraries') {
    authUser(opts)
    return paginate(itineraries, 1, 50)
  }

  if (ctx.method === 'GET' && pathRegex('/admin/itineraries/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const req = itineraries.find((r) => r.id === id)
    if (!req) throw new ApiError('not_found', 'itinerary not found', 404)
    return req
  }

  if (ctx.method === 'GET' && pathRegex('/admin/itineraries/:id/notes', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    return { data: itineraryNotes.get(id) ?? [] }
  }

  if (ctx.method === 'POST' && pathRegex('/admin/itineraries/:id/notes', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const b = body as { body?: string }
    const note = {
      id: (itineraryNotes.get(id)?.length ?? 0) + 1,
      itinerary_id: id,
      author_id: 'admin',
      author_email: 'admin@demo.dev',
      body: b.body ?? '',
      created_at: new Date().toISOString(),
    }
    const existing = itineraryNotes.get(id) ?? []
    itineraryNotes.set(id, [...existing, note])
    return note
  }

  if (ctx.method === 'POST' && pathRegex('/admin/itineraries/:id/assign', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id)
    if (!req) throw new ApiError('not_found', 'itinerary not found', 404)
    const b = body as { assignee_id?: string }
    ;(req as unknown as Record<string, unknown>).assignee_id = b.assignee_id ?? ''
    return req
  }

  if (ctx.method === 'POST' && pathRegex('/admin/itineraries/:id/quote', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const b = body as { line_items?: QuoteLineItem[]; pay_full?: boolean }
    const items = b.line_items ?? []
    const total = items.reduce((sum, li) => sum + (li.amount_minor ?? 0), 0)
    const quote: ItineraryQuote = {
      id: Date.now(),
      request_id: id,
      line_items: items,
      total_cny: total,
      currency: 'CNY',
      total_minor: total,
      deposit_minor: Math.round(total * 0.3),
      status: 'sent',
      sent_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    itineraryQuotes.set(id, quote)
    return quote
  }

  if (ctx.method === 'POST' && pathRegex('/admin/itineraries/:id/confirm', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id)
    if (!req) throw new ApiError('not_found', 'itinerary not found', 404)
    ;(req as unknown as Record<string, unknown>).status = 'confirmed'
    return req
  }

  if (ctx.method === 'POST' && pathRegex('/admin/itineraries/:id/refund-deposit', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = itineraries.find((r) => r.id === id)
    if (!req) throw new ApiError('not_found', 'itinerary not found', 404)
    ;(req as unknown as Record<string, unknown>).status = 'deposit_refunded'
    return req
  }

  if (ctx.method === 'GET' && pathRegex('/admin/itineraries/:id/quote', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const req = SEED_ITINERARIES.find((r) => r.id === id)
    if (!req) throw new ApiError('not_found', 'itinerary not found', 404)
    const q = itineraryQuotes.get(id)
    if (!q) throw new ApiError('not_found', 'quote not yet generated', 404)
    return q
  }

  if (route === 'GET /admin/itineraries/planners') {
    authUser(opts)
    const planners = DEMO_USERS.filter(
      (u) =>
        (u.user.roles ?? []).some((r: string) => r === 'travel_planner') ||
        u.user.role === 'super_admin',
    )
    return { data: planners.map((p) => p.user) }
  }

  if (route === 'GET /admin/itineraries/option-rates') {
    authUser(opts)
    return { data: optionRates as typeof optionRates }
  }

  /* ---- admin: media ---- */
  if (route === 'GET /admin/media/assets') {
    authUser(opts)
    return { data: [...mediaAssets] }
  }
  if (route === 'POST /admin/media/assets') {
    authUser(opts)
    const b = body as {
      public_url?: string
      caption?: string
      mime_type?: string
      file_size?: number
    }
    const asset = {
      id: Math.max(0, ...mediaAssets.map((a) => a.id)) + 1,
      public_url: b.public_url ?? `mock://media/asset-${Date.now()}.png`,
      caption: b.caption,
      mime_type: b.mime_type ?? 'image/png',
      file_size: b.file_size ?? 0,
      created_at: new Date().toISOString(),
    }
    mediaAssets.push(asset)
    return asset
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/media/assets/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const idx = mediaAssets.findIndex((a) => a.id === id)
    if (idx >= 0) mediaAssets.splice(idx, 1)
    return
  }
  if (route === 'POST /admin/media/upload') {
    authUser(opts)
    const asset = {
      id: Math.max(0, ...mediaAssets.map((a) => a.id)) + 1,
      public_url: `mock://media/asset-${Date.now()}.png`,
      mime_type: 'image/png',
      file_size: 200000,
      created_at: new Date().toISOString(),
    }
    mediaAssets.push(asset)
    return asset
  }

  /* ---- admin: certificates ---- */
  if (route === 'GET /admin/certificates') {
    authUser(opts)
    return paginate([...CERTIFICATES], 1, 50)
  }
  if (ctx.method === 'POST' && pathRegex('/admin/certificates/:id/regenerate', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').slice(-2, -1)[0])
    const cert = CERTIFICATES.find((c) => c.id === id)
    if (!cert) throw new ApiError('not_found', 'certificate not found', 404)
    return cert
  }

  /* ---- admin: shipping tiers ---- */
  if (route === 'GET /admin/shipping/tiers') {
    authUser(opts)
    return { data: [...shippingTiers] }
  }
  if (route === 'POST /admin/shipping/tiers') {
    authUser(opts)
    const b = body as Record<string, unknown>
    const tier = {
      id: Math.max(0, ...shippingTiers.map((t) => t.id)) + 1,
      country_code: String(b.country_code ?? ''),
      min_weight_grams: Number(b.min_weight_grams) || 0,
      max_weight_grams: Number(b.max_weight_grams) || 0,
      fee_cny: Number(b.fee_cny) || 0,
    }
    shippingTiers.push(tier)
    return tier
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/shipping/tiers/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const tier = shippingTiers.find((t) => t.id === id)
    if (!tier) throw new ApiError('not_found', 'shipping tier not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.country_code === 'string') tier.country_code = b.country_code
    if (typeof b.min_weight_grams === 'number') tier.min_weight_grams = b.min_weight_grams
    if (typeof b.max_weight_grams === 'number') tier.max_weight_grams = b.max_weight_grams
    if (typeof b.fee_cny === 'number') tier.fee_cny = b.fee_cny
    return tier
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/shipping/tiers/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const idx = shippingTiers.findIndex((t) => t.id === id)
    if (idx >= 0) shippingTiers.splice(idx, 1)
    return
  }

  /* ---- admin: option rates ---- */
  if (route === 'GET /admin/itineraries/option-rates') {
    authUser(opts)
    return { data: [...optionRates] }
  }
  if (route === 'POST /admin/itineraries/option-rates') {
    authUser(opts)
    const b = body as Record<string, unknown>
    const rate = {
      id: Math.max(0, ...optionRates.map((r) => r.id)) + 1,
      option_key: String(b.option_key ?? ''),
      label: String(b.label ?? ''),
      rate_cny: Number(b.rate_cny) || 0,
    }
    optionRates.push(rate)
    return rate
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/itineraries/option-rates/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const rate = optionRates.find((r) => r.id === id)
    if (!rate) throw new ApiError('not_found', 'option rate not found', 404)
    const b = body as Record<string, unknown>
    if (typeof b.option_key === 'string') rate.option_key = b.option_key
    if (typeof b.label === 'string') rate.label = b.label
    if (typeof b.rate_cny === 'number') rate.rate_cny = b.rate_cny
    return rate
  }
  if (ctx.method === 'DELETE' && pathRegex('/admin/itineraries/option-rates/:id', ctx.path)) {
    authUser(opts)
    const id = Number(ctx.path.split('/').pop())
    const idx = optionRates.findIndex((r) => r.id === id)
    if (idx >= 0) optionRates.splice(idx, 1)
    return
  }

  /* ---- admin: users ---- */
  if (route === 'GET /admin/users') {
    authUser(opts)
    const users = DEMO_USERS.map((u) => ({
      ...u.user,
      two_fa_enabled: u.twoFA ?? false,
    }))
    return paginate(users, 1, 50)
  }
  if (ctx.method === 'PUT' && pathRegex('/admin/users/:id/role', ctx.path)) {
    authUser(opts)
    const userId = ctx.path.split('/').slice(-2, -1)[0]
    const b = body as { role?: string }
    const user = DEMO_USERS.find((u) => u.user.id === userId)
    if (user) {
      ;(user.user as unknown as { role: string }).role = b.role ?? user.user.role
    }
    return user?.user ?? null
  }

  /* ---- admin: audit log ---- */
  if (route === 'GET /admin/audit-log') {
    authUser(opts)
    return paginate([], 1, 50)
  }

  /* ---- admin: settings (FX refresh) ---- */
  if (route === 'POST /admin/fx/refresh') {
    authUser(opts)
    return { ok: true }
  }

  throw new ApiError('not_found', `no mock route for ${route}`, 404)
}

function paginate<T>(
  data: T[],
  page: number,
  limit: number,
): { data: T[]; page: number; limit: number; total: number; total_pages: number } {
  const total = data.length
  const total_pages = Math.max(1, Math.ceil(total / limit))
  return { data, page, limit, total, total_pages }
}

function pathRegex(pattern: string, path: string): boolean {
  const patternSegs = pattern.split('/')
  const pathSegs = path.split('/')
  if (patternSegs.length !== pathSegs.length) return false
  return patternSegs.every((s, i) => s.startsWith(':') || s === pathSegs[i])
}
