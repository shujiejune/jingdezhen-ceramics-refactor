# Technical Design Document (TDD)

## 0. Jingdezhen Ceramics Platform

| | |
|---|---|
| **Version** | 0.2 (Draft) |
| **Date** | 2026-07-07 |
| **Status** | Living document — updated as implementation proceeds |
| **Companion docs** | `docs/PRD.md` (v0.17), `docs/REFACTOR-TODO.md` |

This document describes *how* the system specified in the PRD is built. It extends the existing `backend/` codebase (Go/Fiber, handler→service→repository pattern, pgx, JWT auth) rather than starting from scratch.

---

## 1. Overview & Constraints

- **MVP go-live Aug 31, 2026** (PRD §7, milestones M0–M4). Payments run in **sandbox**; itinerary option rates are **mocked CMS data**; policy pages carry placeholder texts.
- Single HK VPS, Docker Compose; PostgreSQL + Redis; Alibaba Cloud OSS + CDN. No Kubernetes, no microservices — a modular monolith.
- Existing code preserved where possible: `user`, `gallery`, `ceramicstory`, `engage`, `notification`, `ws` modules; migration baseline `000001_baseline` (11 tables).
- Design principles: **adapter interfaces for all external services** (swap sandbox→live, SES→Brevo, mock→real LLM without touching business logic); **schema decisions that avoid later migrations** (JSONB attributes, translation tables); **boring technology**.

## 2. System Architecture

### 2.1 Components

```
                        ┌────────────────────────── HK VPS (Docker Compose) ─────────────────────────┐
[Browser] ⇄ [Ali CDN] ⇄ │ [Caddy/Nginx: TLS, routing]                                                │
                        │   ├─ /            → [SolidStart SSR (Node)]                                │
                        │   ├─ /api/*       → [Fiber API (Go)] ── [PostgreSQL]                       │
                        │   ├─ /ws          → [Fiber API (WebSocket)]  └─ [Redis]                    │
                        │   └─ /webhooks/*  → [Fiber API]                                            │
                        │ [Asynq worker (same Go binary, worker mode)]                               │
                        └────────────────────────────────────────────────────────────────────────────┘
                          External: OSS (media), Airwallex, PayPal, Brevo, Qwen3.5, Meta WA, ECB, OSM
```

- **One Go binary, two modes:** `serve` (API + WS) and `worker` (Asynq jobs). Compose runs both from the same image.
- **SolidStart** renders public pages server-side, fetching from the Fiber API over the Compose network (`http://api:PORT`). The browser calls the API directly (via `/api` reverse-proxy path) for interactive features and WS.
- **Media** never flows through the VPS: browser → presigned OSS upload (admin), CDN → OSS (delivery).

### 2.2 Request flows

1. **Public page:** Browser → CDN → SolidStart → Fiber API → PG. Locale from URL prefix. **CDN HTML caching:** public content pages (History, Destinations, Local Lifestyle, product detail, artist profile) are identical for all users of a locale and change only on publish, so cache rendered HTML at the CDN keyed by `(path, locale)` with a short TTL + stale-while-revalidate, and purge on publish (piggyback the sitemap rebuild job). Personalized fragments (cart count, wishlist state) are loaded client-side after hydration so they don't break the shared cache. Admin routes are not SSR'd (see §6) and are never cached.
2. **API mutation (cart, wishlist):** Browser → `/api/...` with JWT → Fiber → PG/Redis.
3. **Chat:** Browser → `/ws` (JWT-authenticated upgrade) → Hub → Qwen adapter (streaming) or agent console. Fan-out via Redis pub/sub is only needed once the API runs >1 instance; on the single-VPS MVP the existing in-memory hub is lower-latency and pub/sub can be deferred (see §4.2).
4. **Payment:** Browser → gateway hosted checkout → gateway webhook → `/webhooks/airwallex|paypal` (signature verify → enqueue job → 200) → worker finalizes order → Brevo email.

## 3. Database Schema

### 3.1 Conventions

- IDs: `BIGSERIAL` for content/commerce entities, `UUID` for `users` (existing). Timestamps `TIMESTAMPTZ`, `created_at`/`updated_at` on every table.
- **Money: `BIGINT` minor units** (fen/cents/pence) + `currency CHAR(3)`. Never floats, never NUMERIC for arithmetic in Go.
- Soft state via `status` columns + CHECK constraints; hard deletes only for GDPR erasure.

### 3.2 i18n translation-table pattern (used by all localized content)

Every localized entity `X` gets a companion `x_translations`:

```sql
CREATE TABLE article_translations (
    id            BIGSERIAL PRIMARY KEY,
    article_id    BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    locale        VARCHAR(10) NOT NULL,        -- BCP 47: 'en-US', 'zh-CN' (later zh-Hant, ja, fr)
    slug          VARCHAR(255) NOT NULL,
    title         VARCHAR(255) NOT NULL,
    content       JSONB NOT NULL,              -- rich-content blocks (see §3.3)
    meta_title    VARCHAR(255),
    meta_description TEXT,
    status        VARCHAR(20) NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft','in_review','published','rejected')),
    reviewed_by   UUID REFERENCES users(id),
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (article_id, locale),
    UNIQUE (locale, slug)
);
```

Key points:
- **Workflow status lives on the translation**, not the parent (PRD §3.1.1: per-locale independent workflow states). Parent holds non-localized data (images, coordinates, display order, taxonomy links).
- Slugs unique **per locale**; sitemap/hreflang generated by joining published translations.
- Same pattern for: `articles`, `destinations`, `ceramic_stories`, `artist_profiles`, `products` (title/description), `interest_options`, `service_options`.
- UI strings (buttons, labels) are **not** in the DB — they live in frontend string catalogs (`en-US.json`, `zh-CN.json`).

### 3.3 Rich content representation

`content JSONB` = ordered array of typed blocks (portable-text style):

```json
[
  {"type":"paragraph","text":"..."},
  {"type":"image","asset_id":123,"caption":"..."},
  {"type":"video","asset_id":124},
  {"type":"heading","level":2,"text":"..."}
]
```

Rendered by SolidStart components; keeps media references as `media_assets` FKs (integrity + CDN URL resolution at render time).

### 3.4 Entity groups (target schema, built up by milestone)

**M0 — Identity & RBAC** (replaces the `users.role` string):

```
roles(id, key)                    -- super_admin, content_editor, travel_planner,
                                  --   ecommerce_operator, customer_service
permissions(id, key)              -- e.g. content.publish, product.write, order.refund
role_permissions(role_id, permission_id)
user_roles(user_id, role_id)      -- customers have NO row here (customer = no staff role)
user_addresses(id, user_id, recipient, line1, line2, city, region, postal_code,
               country CHAR(2), phone, is_default)
user_settings  → columns on users: preferred_locale, preferred_currency
user_2fa(user_id, totp_secret_enc, enabled, confirmed_at)
consent_records(id, user_id NULL, kind, doc_version, granted, ip_hash, created_at)
                                  -- kind: privacy_policy | tos | cookie_analytics | cookie_marketing
```

Middleware: `RequirePermission("content.publish")` replaces `AdminRequired()`. Super Admin bypasses checks. Permission set is code-seeded (migration), not editable in v1 (PRD: no custom roles).

**M1 — Content:**

```
media_assets(id, kind image|video, oss_key, mime, width, height, duration,
             hls_key NULL, uploaded_by, created_at)
categories(id, parent_id NULL, key) + category_translations(name)
tags(id) + tag_translations(name)          -- evolve existing tags table
articles / destinations / ceramic_stories / artist_profiles
  + *_translations (pattern §3.2) + *_media (ordered galleries) + *_tags
destinations extra: lat, lng, address, opening_info JSONB
artist_profiles: evolves existing artists (keep user_id link)
audit_log(id, actor_id, action, entity_type, entity_id, detail JSONB, created_at)
```

**M2 — Commerce** (evolves `artworks` → product/SKU):

```
products(id, artist_profile_id, category_id, status draft|in_review|published|archived)
  + product_translations(title, description, slug, meta_*)
  + product_media(ordered images/videos)
  + product_tags
skus(id, product_id, sku_code UNIQUE, price_cny BIGINT, stock INT,
     weight_grams INT NOT NULL,                 -- packed weight, shipping calc
     low_stock_threshold INT DEFAULT 2,
     attributes JSONB,                          -- size, technique, glaze, edition{type,number},
                                                --   year, kiln  (PRD §3.2.1)
     is_active BOOL)
  indexed: (product_id), GIN(attributes), price_cny
fx_rates(currency CHAR(3) PK, rate_to_cny NUMERIC(18,8), fetched_at)
shipping_fee_tiers(id, country CHAR(2), max_weight_grams INT, fee_cny BIGINT,
                   UNIQUE(country, max_weight_grams))
carts(id, user_id UNIQUE) / cart_items(cart_id, sku_id, qty)   -- guest cart = localStorage
wishlists(user_id, sku_id, created_at)          -- migrated from user_favorite_artworks
orders(id, user_id, status created|paid|shipped|completed|cancelled|refunded,
       currency, subtotal_minor, shipping_minor, total_minor,
       fx_rate_used NUMERIC(18,8), total_cny BIGINT,          -- settlement snapshot
       address JSONB,                                          -- immutable snapshot
       carrier_name NULL, tracking_number NULL, placed_at, ...)
order_items(order_id, sku_id, qty, unit_price_minor, title_snapshot JSONB,
            attributes_snapshot JSONB)                         -- survives product edits
payments(id, order_id, gateway airwallex|paypal, gateway_ref, status, amount_minor,
         currency, raw_webhook JSONB, idempotency_key UNIQUE)
certificates(id, product_id UNIQUE, cert_code UNIQUE, qr_key, pdf_key, issued_at)
provenance_records(id, certificate_id, kind created|sold|transferred, detail JSONB, at)
```

**M3 — Travel & Chat:**

```
itinerary_requests(id, user_id, status pending|processing|quoted|deposit_paid|
                   confirmed|cancelled|closed,
                   arrival_date, duration_days, flexible BOOL, adults, children,
                   interests JSONB, budget JSONB, pace, services JSONB,
                   contact JSONB, notes TEXT, locale, sla_deadline TIMESTAMPTZ,
                   assigned_to UUID NULL, submitted_at)
itinerary_drafts(user_id UNIQUE, form_state JSONB, step INT)   -- save & resume
itinerary_quotes(id, request_id, line_items JSONB, total_cny BIGINT, currency,
                 total_minor, deposit_minor, pdf_key, sent_at)
option_rates(id, option_key, rate_cny BIGINT, unit per_person|per_day|flat)  -- mocked values
crm_notes(id, request_id, author_id, body, created_at)
chat_sessions(id, user_id NULL, status bot|waiting_agent|with_agent|closed,
              agent_id NULL, locale, started_at, closed_at)
chat_messages(id, session_id, sender user|bot|agent, body, created_at)
```

Itinerary deposits reuse `payments` with `order_id NULL` + `itinerary_quote_id`.

**M4 — Analytics & compliance:**

```
analytics_events(id, ts, kind pageview|event, path, name NULL, country CHAR(2),
                 locale, visitor_hash, props JSONB)   -- partitioned by month
-- aggregates computed by nightly job into analytics_daily(date, metric, dims JSONB, value)
```

Visitor hash = HMAC(daily-rotating key, IP+UA) → no raw IPs stored (GDPR); GeoLite2 lookup at ingest.

## 4. Backend Design (Go/Fiber)

### 4.1 Module layout (extends existing pattern)

```
internal/modules/
  user/ gallery→catalog/ ceramicstory/ engage→content/   (existing, refactored)
  rbac/ media/ cart/ order/ payment/ shipping/ certificate/
  itinerary/ chat/ analytics/ admin/
internal/platform/            (new: cross-cutting)
  fx/ pdf/ i18ncontent/ jobs/
pkg/adapters/                 (external service interfaces + impls)
  emailer/   (Brevo + mock; replaces pkg/email SES)
  payments/  (Gateway interface: Airwallex, PayPal, Mock)
  llm/       (Qwen + Mock)
  storage/   (OSS presign/upload + local-dev impl)
  messaging/ (WhatsApp Meta Cloud + noop)
  certchain/ (blockchain adapter: Noop for v1 — PRD §5.4)
```

Every adapter is an interface consumed by services; Compose env selects impl (`PAYMENTS_MODE=sandbox|live|mock`).

### 4.2 Background jobs (Asynq on Redis)

| Job | Trigger | Notes |
|---|---|---|
| `email:send` | services | all Brevo sends, retry w/ backoff |
| `payment:finalize` | webhook handler | idempotent by gateway ref |
| `fx:refresh` | cron daily 16:05 CET | ECB fetch → apply 2% markup → round → upsert `fx_rates` |
| `media:transcode` | upload complete | FFmpeg → HLS → OSS; updates `media_assets.hls_key` |
| `pdf:generate` | certificate/quote created | chromedp or gofpdf; store to OSS |
| `sitemap:rebuild` | content published | writes sitemap.xml to OSS |
| `analytics:rollup` | cron nightly | events → daily aggregates |
| `stock:check` | order paid | fires low-stock notification/email |
| `sla:check` | cron 15min | flags itinerary requests near/past 24h SLA |

**Note on Redis pub/sub:** the chat hub's pub/sub fan-out (§2.2, §5.3) is only required once the API runs more than one instance. On the single-VPS MVP the existing in-memory hub is lower-latency; keep the code path but wire pub/sub when scaling out — a ~1-day change, not an architecture risk.

### 4.3 Error & transaction conventions

- Services return typed errors (`models.ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrValidation{Fields}`); one Fiber error-mapper middleware converts to the API envelope (§5.1). Replaces per-handler `if errors.Is` chains.
- Multi-table writes use `pgx.Tx` passed through repository methods (existing `executor` field already supports this).
- Stock decrement: `UPDATE skus SET stock = stock - $1 WHERE id = $2 AND stock >= $1` inside the order transaction; zero rows → `ErrConflict("insufficient stock")`.

## 5. API Specification

### 5.1 Conventions

- Base path `/api/v1`. JSON envelope:
  - Success: `{"data": ..., "meta": {"page":1,"limit":20,"total":57}}`
  - Error: `{"error": {"code":"validation_failed","message":"...","fields":{...}}}`
- Locale: `?locale=` overrides `Accept-Language`; default `en-US`. Currency: `?currency=` / `X-Currency` header; default USD.
- Auth: JWT access token (15 min) + rotating refresh token in `HttpOnly` cookie (30 d). SSR forwards the cookie; browser JS never sees the refresh token. (Change from current long-lived single JWT.)
- Pagination: `page`/`limit` (max 100), consistent with existing handlers.

### 5.2 Endpoint inventory (summary; per-module detail added as built)

```
Public:   GET /content/{articles|destinations|stories|artists}[/:slug]
          GET /catalog/products[/:slug]   GET /catalog/categories|tags
          GET /certificates/:code                    (QR target, no auth)
          GET /shipping/quote?country=&weight=       (calculator preview)
          POST /analytics/events                     (consent-gated)
Auth:     POST /auth/{signup|login|refresh|logout|activate|reset-password}
          GET  /auth/google/*  /auth/whatsapp/*      (OAuth)
Customer: GET/PUT /profile      CRUD /profile/addresses
          GET/POST/DELETE /wishlist
          GET/POST/PATCH/DELETE /cart/items
          POST /checkout → {order_id, payment_intent}   GET /orders[/:id]
          POST /itineraries (submit)  GET/PUT /itineraries/draft
          GET /itineraries[/:id]      POST /itineraries/:id/pay-deposit
          POST /privacy/{export|delete-account}
Admin:    /admin/... per module, gated by RequirePermission:
          content CRUD + submit/approve/reject, media presign,
          products/skus CRUD + bulk import + certificates,
          orders (list, mark-shipped, refund),
          shipping-fee tiers CRUD, itinerary CRM (inbox, assign, quote, confirm),
          chat agent console endpoints, dashboard stats, users & roles
Webhooks: POST /webhooks/{airwallex|paypal|brevo}    (signature-verified, enqueue-and-ack)
```

### 5.3 WebSocket protocol (`/ws`)

JSON frames, `type`-discriminated:

```
client→server: {"type":"chat.message","session_id":?,"body":"..."}
               {"type":"chat.request_agent","session_id":...}
server→client: {"type":"chat.token","body":"..."}        (LLM stream chunk)
               {"type":"chat.message","sender":"bot|agent",...}
               {"type":"chat.status","status":"waiting_agent|with_agent|closed"}
               {"type":"notification", ...}               (existing notification push)
agent console: same socket, agent frames gated by RBAC
```

Hub keys connections by userID (existing); adds session routing + Redis pub/sub channel per session for multi-instance safety.

## 6. Frontend Design (SolidStart)

- **Routes:** `src/routes/[locale]/...` for public (en/zh); `src/routes/admin/...` client-rendered behind auth. Locale param validated against supported list; `hreflang` + canonical emitted in root layout.
- **Data:** SSR loaders (`createAsync`/`query`) call the API server-to-server for initial render; TanStack Solid Query takes over on the client (hydrated cache) for mutations/refetch. TanStack Table for all admin lists; TanStack Form for wizard + CMS forms.
- **State:** cart for guests in `localStorage` (merged via `POST /cart/merge` on login); locale/currency in cookie + context.
- **i18n:** string catalogs per locale (flat JSON, ICU-style interpolation via `@solid-primitives/i18n` or similar); content comes localized from the API.
- **SEO:** meta from API per entity; `sitemap.xml` served from OSS via CDN; JSON-LD components (Product, Article, BreadcrumbList).
- **Design tokens:** CSS variables (celadon/ink palette, spacing, type scale) + a small component library; WebP `srcset` via OSS image-processing URL params; `loading="lazy"`; hls.js for video.

## 7. Money & Pricing

- **Representation:** minor units `BIGINT` everywhere; CNY authored prices are integers of fen.
- **FX pipeline:** daily job fetches ECB EUR-base rates → derive CNY→{USD,EUR,GBP} → multiply by `(1 + markup)` (default 0.02, CMS-configurable) → store in `fx_rates`. Display conversion at read time; cached per day.
- **Rounding (display & charge):** convert to major units, then: `< 100` → ceil to next 0.50; `>= 100` → ceil to next 1.00. Implemented once in `platform/fx`, unit-tested against PRD examples (€183.47 → €184).
- **Order snapshot:** at checkout, order stores the converted `total_minor`, `currency`, `fx_rate_used`, and `total_cny`. Later rate changes never affect placed orders. Refunds use the stored gateway charge amount (original currency) — full refunds only (PRD §3.2.3).
- **Shipping calc:** `fee = tier(country, ceil(Σ item.weight_grams * qty))`; no tier ≥ weight → overweight block; no tiers for country → not-shippable block. Fee converted with the same FX+rounding path.
- **Itinerary quotes:** line items priced in CNY from `option_rates` (mocked), converted identically; deposit = `round(total * 0.30)` in presentment currency.

## 8. State Machines

**Order:** `created →(webhook: payment succeeded)→ paid →(operator enters tracking)→ shipped →(auto after N days or customer confirm)→ completed`; `created→cancelled` (customer/timeout); `paid|shipped→refunded` (operator). Side effects on transitions: emails (paid, shipped, refunded), stock decrement (at `created`, restored on `cancelled`), provenance record + low-stock check (at `paid`).

**Itinerary:** `pending →(planner opens)→ processing →(quote sent)→ quoted →(deposit webhook)→ deposit_paid →(planner confirms; PDF + email/WA)→ confirmed`; `any→cancelled|closed`. SLA timer runs from submission (24 h, `sla:check` job).

**Content translation:** `draft →(editor submits)→ in_review →(super admin)→ published | rejected(comments)→ draft`. Publishing fires sitemap rebuild; unpublish allowed to Super Admin.

**Chat session:** `bot →(escalate)→ waiting_agent →(agent claims)→ with_agent → closed`; `waiting_agent→closed` (offline fallback → email follow-up).

All transitions enforced in services (single `Transition(from, to, actor)` helper per machine + permission check); invalid transitions → `ErrConflict`.

## 9. Auth & Security

- Access JWT (15 min; claims: user_id, roles) + refresh rotation with reuse detection (revoke family on reuse). Refresh cookie `Secure; HttpOnly; SameSite=Lax; Path=/api/v1/auth`.
- OAuth: Google (existing) + WhatsApp (post-MVP); account linking by verified email.
- TOTP 2FA: enrollment (QR, secret encrypted at rest with app key), verify step inserted into login when enabled; **mandatory for super_admin** (enforced at login).
- Rate limits (Fiber limiter + Redis): auth endpoints 5/min/IP, chat messages 20/min/user, analytics 60/min/IP, global 100/min/IP.
- Webhooks: Airwallex HMAC / PayPal cert verification before enqueue; idempotency keys on `payments`.
- Secrets: `.env` on VPS (root-only) for MVP; move to SOPS/age-encrypted repo file when convenient. Never in git plaintext.
- Headers: CSP, HSTS, X-Frame-Options via reverse proxy; CORS locked to site origin.

## 10. Integrations

| Service | Interface | Sandbox/dev impl | Failure handling |
|---|---|---|---|
| Airwallex | `payments.Gateway` (CreateIntent, Refund, VerifyWebhook) | Airwallex demo env (MVP) / `MockGateway` in tests | Webhook retries handled by idempotency; intent creation errors → user-visible retry |
| PayPal | same interface | PayPal sandbox / mock | same |
| Brevo | `emailer.Sender` (SendTemplate) | mock logging sender in dev | Asynq retry ×5, then dead-letter + admin notification |
| Qwen3.5 | `llm.Client` (StreamChat) | canned-response mock | timeout 30 s; on failure bot apologizes + offers agent/leave-message |
| OSS | `storage.Store` (PresignUpload, PublicURL, Put) | MinIO or local-dir impl in dev | uploads are client-side; API only signs |
| WhatsApp | `messaging.Sender` | noop (post-MVP) | fallback: Brevo email always sent |
| ECB FX | `fx.RateSource` | fixture rates in dev | keep last-known rates; alert if stale > 48 h |
| GeoLite2 | local `.mmdb`, monthly refresh job | bundled test db | unknown country → 'ZZ' |

## 11. Testing & Environments

- Per PRD §2.4: testify units (adapters mocked), testcontainers-go integration (real PG+Redis; migrations applied), Playwright E2E vs staging, k6 smoke/load.
- Priority test targets (highest bug value): `platform/fx` rounding, shipping calculator (tiers/overweight), order state machine + stock, webhook idempotency, RBAC middleware matrix, i18n slug resolution.
- **Environments:** `dev` (Compose: PG, Redis, MinIO, mock adapters), `staging` (VPS, sandbox gateways, real OSS/Brevo test), `prod`. Config via env vars only (extend `config.go`; remove AWS fields).

### 11.1 Performance priorities (MVP-sized, highest ROI)

No message broker is required: Asynq-on-Redis (§4.2) covers all deferred/flaky/heavy/scheduled work; cross-service domain events stay in-process (modular monolith); Redis pub/sub is deferred to multi-instance (§4.2). Go's goroutines provide request concurrency natively — no async framework. Latency targets: SSR LCP < 3s p75 (PRD §4.2), API p95 < 300 ms (PRD §4.2, enforced by k6 §2.4.3).

1. **Parallel SSR data loads** — SolidStart loaders `Promise.all` multiple API calls (API is over Compose localhost); avoid serial waterfalls. *(M1 frontend)*
2. **Redis hot-data cache** — FX rates, category tree, product detail (per locale+currency), shipping tiers, nav menus; short TTL + invalidate-on-edit. FX is daily — cache in Redis *and* in-process, convert/round in Go. *(M2)*
3. **Indexes on the hot paths** — `(entity_id, locale)` and `(locale, slug)` on translation tables; product filters (`artist, edition_type, price_cny`); `tsvector` for search. Add as migrations land, not after. *(M1/M2)*
4. **Caddy edge wins** — HTTP/3, brotli/gzip, and `Cache-Control: immutable` for SolidStart's hashed assets; long-cache. *(M0 infra)*
5. **CDN HTML caching for public content pages** — keyed by `(path, locale)`, short TTL + SWR, purge on publish via the sitemap job; personalized fragments load client-side post-hydration (see §2.2). *(M1)*
6. **Outbound HTTP client hygiene** — reuse clients for Airwallex/Brevo/Qwen/OSS with custom transports, sane timeouts, pooled connections; bound concurrency with semaphores so a slow gateway can't exhaust goroutines. *(M2/M3)*

Items 1–4 are essentially free given the chosen stack; 5 is a deliberate (small) architecture decision; 6 is hygiene. Chat: stream Qwen tokens straight to the socket, never buffer the full response.

## 12. Open Technical Decisions

- [ ] PDF engine: chromedp (HTML→PDF, easy branded templates, heavier) vs gofpdf/maroto (lighter, more layout code). Leaning chromedp for quote/certificate templates.
- [ ] Search: PG `tsvector` config for zh-CN (use `pg_jieba`? or simple + trigram) — decide in M1.
- [ ] Reverse proxy: Caddy (auto-TLS, simpler) vs Nginx (team familiarity) — decide in M0.
- [ ] SolidStart deploy target: node-server preset behind proxy (assumed) vs static+CSR for admin.
- [ ] Qwen3.5 endpoint region + prompt/RAG design (PRD deferral).
- [ ] Exact Brevo template IDs & email design.
- [ ] `articles` vs `destinations` split: current `engage` mixes activities/articles — final entity split lands with M1 migration.
- [x] i18n content infrastructure: `platform/i18ncontent` package codifies the per-locale translation-table pattern — locale constants (en-US, zh-CN + future zh-Hant/ja/fr), `ContentStatus` workflow enum, rich-content `ContentBlock` types (§3.3), `NormalizeLocale` (validate for CMS writes, fallback for public reads), `Transition`/`CanEdit` workflow state machine (editor submit/reopen; super-admin-only approve/reject/unpublish), `PublishedFilter`. Unit-tested transition matrix. `models/i18n.go` holds the shared constants/types every content module imports.
- [x] Product tags (translatable): migration `000021_product_tags` evolves the dead baseline `tags(name)` into `tags(id, key UNIQUE)` + `tag_translations(tag_id, locale, name)` + `product_tags` join. Tags are taxonomy (no workflow status, unlike content translations) — visible iff attached to a published product. Canonical language-neutral `key` (lowercase kebab-case CHECK) is the admin/CSV identifier; display `name` resolves per-locale with en-US → key fallback (COALESCE). Admin assigns by key (absolute set-replace: nil=unchanged, `[]`=clear); unknown keys created inline with en-US name=key default. Category tree stays a separate deferred track (bare `products.category` unchanged); tags are an orthogonal discovery facet. PRD §3.2.1 line 173; TDD §3.2 line 130.
- [x] `product.publish` permission: added to the RBAC seed (migration `000012`). Mirrors `content.publish` — only super_admin can approve/publish products (PRD §3.1.1 editorial workflow applies to all content types including commerce content). E-commerce Operators get `product.read` + `product.write` (author/edit/submit/SKU management) but cannot publish. The TDD §3.4 M2 schema diagram listed `status` on the `products` parent, but products follow the i18n pattern (status on the translation row, per-locale independent workflow states) — the translation-table implementations (ceramicstory/engage/artist/product) are the source of truth.
