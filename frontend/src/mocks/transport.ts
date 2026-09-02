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
  ItineraryRequest,
  Order,
  OrderItem,
  OrderStatus,
  Product,
  SKU,
  Tag,
  User,
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
let idSeq = { order: 2000, item: 9100, address: 10, itinerary: 6000, token: 100 }

/* seed orders/itineraries with resolved addresses. Order titles are
   stored bilingual (title_snapshot) and resolved to the request locale
   on read — like the Go service does with its snapshot JSONB. */
for (const o of SEED_ORDERS) {
  const addr = addresses.find((a) => a.id === o.address_id)!
  orders.push({ ...o, address: addr })
}
for (const r of SEED_ITINERARIES) itineraries.push({ ...r })

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
        userCarts: [...userCarts],
        guestCarts: [...guestCarts],
        wishlists: [...wishlists],
        addresses,
        orders,
        itineraries,
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
      userCarts?: Array<[string, CartLine[]]>
      guestCarts?: Array<[string, CartLine[]]>
      wishlists?: Array<[string, number[]]>
      addresses?: StoredAddress[]
      orders?: StoredOrder[]
      itineraries?: ItineraryRequest[]
      idSeq?: typeof idSeq
      extraUsers?: Array<(typeof DEMO_USERS)[number]>
    }
    for (const [k, v] of s.sessions ?? []) sessions.set(k, v)
    for (const [k, v] of s.userCarts ?? []) userCarts.set(k, v)
    for (const [k, v] of s.guestCarts ?? []) guestCarts.set(k, v)
    for (const [k, v] of s.wishlists ?? []) wishlists.set(k, v)
    if (s.addresses) addresses.splice(0, addresses.length, ...s.addresses)
    if (s.orders) orders.splice(0, orders.length, ...s.orders)
    if (s.itineraries) itineraries.splice(0, itineraries.length, ...s.itineraries)
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
      if (e instanceof ApiError) throw e
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

  if (route === 'GET /artists') return ARTISTS.map((a) => toArtist(a, locale))
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
    let list = ACTIVITIES.map((a) => toActivity(a, locale))
    if (opts.params?.type) list = list.filter((a) => a.type === opts.params!.type)
    return list
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
    if (!found || found.password !== password) {
      throw new ApiError('invalid_credentials', 'invalid credentials', 401)
    }
    if (found.twoFA) {
      const pending = `pending_${found.id}_${idSeq.token++}`
      pending2FA.set(pending, { userId: found.id, fails: 0 })
      return { pending_2fa_token: pending, expires_in_seconds: 900 }
    }
    const token = `tok_${found.id}_${idSeq.token++}`
    sessions.set(token, found.id)
    return { access_token: token, user: found.user }
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
    const token = `tok_${user.id}_${idSeq.token++}`
    sessions.set(token, user.id)
    return { access_token: token, user }
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
    return addresses.filter((a) => a.user_id === user.id)
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
    return itineraries.filter((r) => r.user_id === user.id)
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
    return req
  }

  throw new ApiError('not_found', `no mock route for ${route}`, 404)
}

function pathRegex(pattern: string, path: string): boolean {
  const patternSegs = pattern.split('/')
  const pathSegs = path.split('/')
  if (patternSegs.length !== pathSegs.length) return false
  return patternSegs.every((s, i) => s.startsWith(':') || s === pathSegs[i])
}
