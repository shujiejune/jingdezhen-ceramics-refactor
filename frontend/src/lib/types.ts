/**
 * Domain types — TS mirrors of the Go DTOs in
 * ../backend/internal/models/*.go. These are the shapes the API client
 * (and the mock transport in PROTOTYPE mode) returns.
 *
 * Money rule (TDD §7): every amount is an integer in minor units
 * (fen/cents/pence). The frontend NEVER re-derives totals or FX — it
 * formats server-provided presentment values only (lib/money.ts).
 */

export type ContentStatus = 'draft' | 'in_review' | 'published' | 'rejected'

/* ------------------------------ catalog ------------------------------ */

export interface Tag {
  id: number
  key: string
  name: string
}

export interface ProductMediaItem {
  media_id: number
  public_url: string
  caption?: string
  /** prototype-only: drives the procedural figure */
  figure_seed: number
  figure_kind: 'vase' | 'bowl' | 'plate' | 'teapot' | 'jar'
}

export interface SKUAttributes {
  size?: string
  technique?: string
  glaze?: string
  edition_type?: 'one_of_a_kind' | 'limited_edition' | 'open_production'
  edition_number?: string
  year?: number
  kiln?: string
  [k: string]: unknown
}

export interface SKU {
  id: number
  product_id: number
  sku_code: string
  price_cny: number // minor units (fen)
  stock: number
  weight_grams: number
  low_stock_threshold: number
  attributes: SKUAttributes
  is_active: boolean
  /** presentment (populated when ?currency= supplied) */
  price?: number
  price_currency?: string
}

export interface Product {
  id: number
  artist_id?: number
  artist_name?: string
  artist_slug?: string
  category?: string
  figure_seed: number
  figure_kind: ProductMediaItem['figure_kind']
  title: string
  slug: string
  description?: string
  meta_title?: string
  meta_description?: string
  locale: string
  status: ContentStatus
  published_at?: string
  skus?: SKU[]
  gallery?: ProductMediaItem[]
  tags?: Tag[]
  /** digital-certificate code, when one exists for this product */
  cert_code?: string
  /** locale → slug for every other published translation (hreflang) */
  alternates?: Record<string, string>
  created_at: string
  updated_at: string
}

/* ------------------------------ artists ------------------------------ */

export interface GalleryItem {
  media_id: number
  public_url: string
  caption?: string
  figure_seed: number
  figure_kind: ProductMediaItem['figure_kind']
}

export interface Artist {
  id: number
  glyph: string // prototype: surname character for the medallion
  name: string
  slug: string
  bio?: string
  meta_title?: string
  meta_description?: string
  locale: string
  status: ContentStatus
  published_at?: string
  alternates?: Record<string, string>
  gallery?: GalleryItem[]
}

/* ------------------------------ content ------------------------------ */

export type ContentBlock =
  | { type: 'paragraph'; text: string }
  | { type: 'heading'; level: 2 | 3; text: string }
  | {
      type: 'image'
      figure_seed: number
      figure_kind: ProductMediaItem['figure_kind']
      caption?: string
    }
  | { type: 'quote'; text: string }

export interface CeramicStory {
  id: number
  dynasty_start_year: number
  figure_seed: number
  title: string
  slug: string
  summary?: string
  content: ContentBlock[]
  meta_title?: string
  meta_description?: string
  locale: string
  status: ContentStatus
  published_at?: string
  alternates?: Record<string, string>
}

export interface Activity {
  id: number
  type: 'destination' | 'lifestyle'
  lat?: number
  lng?: number
  address?: string
  opening_info?: string
  figure_seed: number
  title: string
  slug: string
  summary?: string
  content: ContentBlock[]
  meta_title?: string
  meta_description?: string
  locale: string
  status: ContentStatus
  published_at?: string
  alternates?: Record<string, string>
  gallery?: GalleryItem[]
}

/* ------------------------------ users/auth ------------------------------ */

export type StaffRole =
  'super_admin' | 'content_editor' | 'travel_planner' | 'ecommerce_operator' | 'customer_service'

export type UserRole = 'customer' | StaffRole

export type Permission =
  | 'users.manage'
  | 'content.write'
  | 'content.publish'
  | 'product.read'
  | 'product.write'
  | 'product.publish'
  | 'certificate.manage'
  | 'order.read'
  | 'order.write'
  | 'order.refund'
  | 'itinerary.read'
  | 'itinerary.write'
  | 'itinerary.confirm'
  | 'chat.handle'
  | 'dashboard.view'
  | 'settings.manage'

export interface User {
  id: string
  email: string
  nickname: string
  avatar_glyph?: string
  /** primary role (backward compat — backend JWT carries `roles: string[]`) */
  role: UserRole
  /** all staff roles the user holds (empty for customers) */
  roles?: StaffRole[]
  preferred_locale?: string
  preferred_currency?: string
  created_at: string
}

export interface AuthResponse {
  access_token: string
  user: User
}

export interface Address {
  id: number
  recipient: string
  line1: string
  line2?: string
  city: string
  region?: string
  postal_code: string
  country: string // ISO 3166-1 alpha-2
  phone: string
  is_default: boolean
}

/* ------------------------------ cart ------------------------------ */

export interface CartItem {
  sku_id: number
  sku_code: string
  qty: number
  unit_price_cny: number
  line_total_cny: number
  stock: number
  weight_grams: number
  product_id: number
  product_slug: string
  product_title: string
  figure_seed: number
  figure_kind: ProductMediaItem['figure_kind']
  artist_name?: string
  attributes?: SKUAttributes
  added_at: string
  unit_price?: number
  line_total?: number
}

export interface Cart {
  items: CartItem[]
  item_count: number
  total_cny: number
  total?: number
  currency?: string
}

/* ------------------------------ orders ------------------------------ */

export type OrderStatus = 'created' | 'paid' | 'shipped' | 'completed' | 'cancelled' | 'refunded'

export interface OrderItem {
  id: number
  order_id: number
  sku_id: number
  qty: number
  unit_price_minor: number
  unit_price_cny: number
  title_snapshot: string
  attributes_snapshot?: SKUAttributes
  figure_seed?: number
  figure_kind?: ProductMediaItem['figure_kind']
}

export interface Order {
  id: number
  user_id: string
  status: OrderStatus
  currency: string
  subtotal_minor: number
  shipping_minor: number
  total_minor: number
  subtotal_cny: number
  shipping_cny: number
  total_cny: number
  fx_rate_used?: number
  address: Address
  locale?: string
  carrier_name?: string
  tracking_number?: string
  placed_at: string
  paid_at?: string
  shipped_at?: string
  completed_at?: string
  cancelled_at?: string
  refunded_at?: string
  cancel_reason?: string
  items?: OrderItem[]
  hosted_url?: string
}

/* ------------------------------ wishlist ------------------------------ */

export interface WishlistItem {
  sku_id: number
  added_at: string
  product_id: number
  product_slug: string
  product_title: string
  artist_name?: string
  figure_seed: number
  figure_kind: ProductMediaItem['figure_kind']
  price?: number
  price_currency?: string
  stock: number
}

/* ------------------------------ itinerary ------------------------------ */

export type ItineraryStatus =
  'pending' | 'processing' | 'quoted' | 'deposit_paid' | 'confirmed' | 'cancelled' | 'closed'

export type QuoteStatus = 'sent' | 'deposit_paid' | 'fully_paid' | 'cancelled'

export interface QuoteLineItem {
  label: string
  detail?: string
  amount_minor: number
  amount?: number
}

export interface ItineraryQuote {
  id: number
  request_id: number
  line_items: QuoteLineItem[]
  total_cny: number
  currency: string
  total_minor: number
  deposit_minor: number
  fx_rate_used?: number
  status: QuoteStatus
  sent_at: string
  paid_at?: string
  pdf_key?: string
  created_at: string
  updated_at: string
}

export interface DepositPaidResponse {
  quote_id: number
  hosted_url?: string
}

export interface Budget {
  currency: 'USD' | 'EUR' | 'GBP'
  min_minor: number
  max_minor: number
}

export interface ItineraryServices {
  guide: 'none' | 'english' | 'other'
  hotel: boolean
  hotel_level?: 'budget' | 'comfort' | 'luxury'
  pickup: boolean
  experience: boolean
  dietary_accessibility?: string
}

export interface ItineraryContact {
  channel: 'email' | 'whatsapp'
  whatsapp_number?: string
  notes?: string
}

export interface ItineraryRequest {
  id: number
  user_id: string
  status: ItineraryStatus
  arrival_date: string
  duration_days: number
  flexible: boolean
  adults: number
  children: number
  interests: string[]
  budget?: Budget
  pace: 'relaxed' | 'balanced' | 'packed'
  services: ItineraryServices
  contact: ItineraryContact
  locale: string
  sla_deadline: string
  submitted_at: string
  quote?: ItineraryQuote
}

/* ------------------------------ certificate ------------------------------ */

export interface ProvenanceRecord {
  id: number
  kind: 'created' | 'sold' | 'transferred'
  detail: string
  at: string
}

export interface Certificate {
  id: number
  product_id: number
  cert_code: string
  issued_at: string
  product_title: string
  product_slug: string
  artist_name: string
  figure_seed: number
  figure_kind: ProductMediaItem['figure_kind']
  attributes?: SKUAttributes
  provenance: ProvenanceRecord[]
  qr_key?: string
  pdf_key?: string
}

/* ------------------------------ notifications ------------------------------ */

export interface Notification {
  notification_id: number
  recipient_user_id: string
  actor_user_id?: string
  actor_user?: string
  notification_type: string
  entity_type?: string
  entity_id?: number
  message: string
  is_read: boolean
  created_at: string
}

/* ------------------------------ consent ------------------------------ */

export type ConsentKind = 'privacy_policy' | 'tos' | 'cookie_analytics' | 'cookie_marketing'

export interface ConsentRecord {
  id: number
  user_id?: string
  kind: ConsentKind
  doc_version: string
  granted: boolean
  created_at: string
}

export interface ConsentState {
  kind: ConsentKind
  granted: boolean
  recorded: boolean
  doc_version?: string
  created_at?: string
}

/* ------------------------------ GDPR ------------------------------ */

export interface UserDataExport {
  exported_at: string
  user_id: string
  locale?: string
  profile?: User
  addresses?: Address[]
  consent_records?: ConsentRecord[]
  two_fa?: boolean
  wishlist?: WishlistItem[]
  notifications?: Notification[]
}

/* ------------------------------ shipping ------------------------------ */

export interface ShippingQuote {
  country: string
  weight_grams: number
  fee_cny: number
  fee?: number
  currency?: string
  /** blocked states per PRD §3.2.3 */
  blocked_reason?: 'unshippable' | 'overweight'
}

export interface ShippingTier {
  id: number
  country_code: string
  min_weight_grams: number
  max_weight_grams: number
  fee_cny: number
  created_at?: string
  updated_at?: string
}

export interface OptionRate {
  id: number
  option_key: string
  label?: string
  rate_cny: number
  rate?: number
  currency?: string
  created_at?: string
  updated_at?: string
}

/* ------------------------------ admin types ------------------------------ */

export interface AdminUser {
  id: string
  email: string
  nickname: string
  role: UserRole
  roles?: StaffRole[]
  preferred_locale?: string
  preferred_currency?: string
  two_fa_enabled?: boolean
  created_at: string
}

export interface MediaAsset {
  id: number
  public_url: string
  caption?: string
  mime_type?: string
  file_size?: number
  figure_seed?: number
  figure_kind?: ProductMediaItem['figure_kind']
  created_at: string
}

export interface BulkImportSummary {
  total_rows: number
  imported: number
  updated: number
  failed: number
  errors: Array<{ row: number; message: string }>
}

export interface AuditLog {
  id: number
  user_id: string
  user_email: string
  action: string
  entity_type: string
  entity_id?: number
  details?: Record<string, unknown>
  created_at: string
}

export interface DashboardTraffic {
  range: string
  from: string
  to: string
  data: Array<{ date: string; pageviews: number; unique_visitors: number }>
}

export interface DashboardSales {
  range: string
  from: string
  to: string
  data: Array<{
    date: string
    orders: number
    revenue_cny: number
    revenue?: number
    currency?: string
  }>
}

export interface DashboardFunnel {
  range: string
  from: string
  to: string
  steps: Array<{ step: string; label: string; count: number; rate: number }>
}

export interface QuoteLineItemInput {
  label: string
  detail?: string
  amount_minor: number
}

export interface AdminSendQuoteInput {
  line_items: QuoteLineItemInput[]
  pay_full: boolean
  currency?: string
}

export interface AdminItineraryNote {
  id: number
  itinerary_id: number
  author_id: string
  author_email: string
  body: string
  created_at: string
}

export interface AdminAssignInput {
  assignee_id: string
}

/* ------------------------------ chat (TDD §5.3) ------------------------------ */
/*
 * Frame protocol over the /ws WebSocket (backend M3 — chat endpoints not yet
 * implemented; the frontend runs against MockChatTransport until they land).
 *
 *   client→server: {"type":"chat.message","session_id":?,"body":"…"}
 *                  {"type":"chat.request_agent","session_id":…}
 *   server→client: {"type":"chat.token","body":"…"}   (LLM stream chunk)
 *                  {"type":"chat.message","sender":"bot|agent",…}
 *                  {"type":"chat.status","status":"waiting_agent|with_agent|closed"}
 *
 * Session lifecycle: bot →(escalate)→ waiting_agent →(agent claims)→ with_agent
 * → closed; waiting_agent → closed (offline fallback → email follow-up).
 */

export type ChatSessionStatus = 'bot' | 'waiting_agent' | 'with_agent' | 'closed'
export type ChatSender = 'user' | 'bot' | 'agent'

export interface ChatMessage {
  id: number
  session_id: number
  sender: ChatSender
  body: string
  created_at: string
}

export interface ChatSession {
  id: number
  user_id?: string
  user_email?: string
  locale: string
  status: ChatSessionStatus
  messages: ChatMessage[]
  updated_at: string
}

export type ChatClientFrame =
  | { type: 'chat.message'; session_id?: number; body: string }
  | { type: 'chat.request_agent'; session_id: number }

export type ChatServerFrame =
  | { type: 'chat.token'; body: string }
  | { type: 'chat.message'; sender: ChatSender; message: ChatMessage }
  | { type: 'chat.status'; session_id: number; status: ChatSessionStatus }

/* ------------------------------ envelope ------------------------------ */

export interface Paginated<T> {
  data: T[]
  page: number
  limit: number
  total: number
  total_pages: number
}

export interface ApiErrorEnvelope {
  error: {
    code: string
    message: string
    details?: Record<string, string>
  }
}
