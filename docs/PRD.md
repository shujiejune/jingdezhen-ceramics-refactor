# Product Requirements Document (PRD)

# Jingdezhen Ceramic Culture International Promotion & Tourism E-commerce Platform

**Abbreviation:** _Jingdezhen Ceramics Platform_

| | |
|---|---|
| **Version** | 0.17 (Draft) |
| **Date** | 2026-07-07 |
| **Status** | Draft — decisions recorded inline as **(confirmed/decided)**; remaining gaps as `<!-- TODO -->` and §8 |
| **Source** | `customer requirements.md` |

---

## 1. Overview

### 1.1 Background & Vision

Jingdezhen, the "Millennium Ceramic Capital", possesses over a thousand years of ceramic production history. This project builds an internationalized, comprehensive service platform targeting a global audience that:

1. Showcases Jingdezhen's ceramic history, heritage, destinations, and contemporary art ecosystem.
2. Provides an international e-commerce channel for ceramic artworks with digital authentication.
3. Offers custom travel itinerary services with AI-assisted, multi-lingual customer support.
4. Is operated through a unified CMS with role-based access control and a data dashboard.

### 1.2 Goals & Success Metrics

- Page load time < 3 seconds in North America, Europe, and Asia (via global CDN).
- Business KPIs (visitor volume, GMV, itinerary conversion) to be baselined **post-launch** from dashboard data rather than invented up front.
- **MVP launch: May 2027** (see §7). Legal review and payment merchant onboarding are **handled after the MVP** — the MVP runs payments in sandbox mode with placeholder policy texts.

### 1.3 Target Users

| Persona | Description |
|---|---|
| International Visitor | Browses culture/history content, plans a trip to Jingdezhen |
| International Buyer | Purchases ceramic artworks, verifies authenticity via digital certificates |
| Travel Customer | Submits custom itinerary requests, communicates with planners |
| Content Editor | Manages culture/travel content in CMS |
| Travel Planner | Handles itinerary requests, CRM follow-ups |
| E-commerce Operator | Manages products, inventory, orders |
| Customer Service | Handles escalated chat inquiries |
| Super Administrator | Full system administration |

### 1.4 Scope

**In scope:** Public-facing web app (EN/zh-CN), e-commerce, digital authentication, custom travel, AI chat support, admin CMS, dashboard, compliance features, SEO.

**Out of scope (confirmed):** Native mobile apps, offline POS, and **logistics/carrier integration**. Orders are shipped via a mail service chosen by the operator outside the platform; the operator records a **tracking number** on the order, and customers check delivery status on the third-party mail service's own website. No carrier APIs, shipping-rate calculation services, or tracking-status polling will be integrated.

---

## 2. Technical Architecture

### 2.1 Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go + [Fiber](https://gofiber.io/) web framework; WebSocket support (Fiber websocket middleware) for live chat |
| Frontend | **TanStack Start** (React SSR full-stack framework, built on Vite + Vinxi) + TypeScript + TanStack libraries (Router / Query / Form / Table) |
| Database | **PostgreSQL** (multi-currency, JSONB for rich content, full-text search) |
| Cache / Queue | **Redis** — sessions, cache, pub/sub for chat fan-out, and job queue (e.g. Asynq) for emails/WhatsApp/webhooks |
| Object storage | **Alibaba Cloud OSS (Hong Kong region)** — S3-compatible; built-in on-the-fly image processing (WebP conversion, resizing) for all uploaded media |
| CDN | **Alibaba Cloud CDN (outside-mainland edge network)** in front of OSS and the site. HK edges serve mainland-China users acceptably **without ICP filing**; if an ICP filing is obtained later (China entity exists), mainland CDN nodes can be enabled with no architecture change |
| Video | Self-hosted transcoding: **FFmpeg → HLS** segments uploaded to OSS, served via CDN |
| Search | PostgreSQL full-text search for content and product catalog (can migrate to Meilisearch/Typesense later if catalog scale requires) |
| AI Chatbot | **Qwen3.5** (via Alibaba Cloud Model Studio / DashScope-compatible API) for multi-lingual chat and translation; delivered over WebSockets (API region & budget deferred, see §3.3.1) |
| Deployment | Single VPS in **Hong Kong** (Alibaba Cloud / Tencent Cloud) running **Docker Compose** (Fiber API, TanStack Start, PostgreSQL, Redis) — accessible from both mainland China and abroad without ICP filing; minimal cost (~$40–80/mo incl. storage/CDN at launch traffic). Nightly `pg_dump` backups to OSS. Scale out later if needed. CI/CD: see §2.4. |

### 2.2 High-Level Architecture

```
[Browser] ⇄ [CDN] ⇄ [TanStack Start SSR (Node)] ⇄ [Fiber API (Go)]
                          │  (REST + WebSocket)    ├─ PostgreSQL
                          │                        ├─ Redis (cache/session/pub-sub/queue)
                          └──────────────────────► ├─ Object Storage (WebP images, HLS video)
                                                   └─ Third-party APIs:
                                                      Airwallex/PayPal, Brevo,
                                                      WhatsApp Business, Qwen3.5, OpenStreetMap,
                                                      Blockchain-authentication (reserved)
```

**Rendering strategy (decided):** **TanStack Start with SSR** for all public pages to satisfy SEO requirements (semantic URLs, per-locale meta tags, sitemap, `hreflang`). The admin CMS routes may be client-rendered. TanStack Start is chosen over Next.js to keep type-safe TanStack Router (file-based, type-safe search params via `validateSearch` + zod) and to avoid Vercel-lock-in on a single self-hosted HK VPS (TanStack Start runs on Vite + Vinxi, deployable anywhere). TanStack Query/Table/Form are used within TanStack Start. The SSR layer fetches data from the Fiber API; the browser talks to the Fiber API directly for interactive features (cart, chat WebSocket). *(Pivoted from the earlier SolidStart decision; see TDD §12 for rationale.)*

### 2.3 Internationalization (i18n)

- **Launch locales:** **English (US)** (default, `en-US`) and **Simplified Chinese** (`zh-CN`), switchable.
- **Future locales (planned, not in v1):** Traditional Chinese (`zh-TW`/`zh-Hant`), Japanese (`ja`), French (`fr`). The i18n architecture must be locale-extensible from day one: per-locale translation tables (not fixed columns), locale-aware slugs/meta/sitemaps, and UI string catalogs keyed by BCP 47 locale so new languages can be added without schema changes.
- All CMS content models must store per-locale fields (title, body, meta tags, slugs).
- **URL strategy (confirmed): locale path prefix** (`/en-US/...`, `/zh-CN/...`) with `hreflang` tags — one domain, one TLS cert, simplest TanStack Start routing, consolidated SEO authority.
- Currency display: multi-currency (see §3.2.3).

### 2.4 CI/CD & Quality Assurance

#### 2.4.1 CI/CD Pipeline

**Platform: GitHub Actions.**

```
PR:      lint → unit tests → integration tests → build images
main:    all of the above → push images to registry (Alibaba Cloud ACR, HK)
         → deploy to staging → smoke tests → manual approval → deploy to prod
         → smoke tests on prod → rollback to previous image tag on failure
nightly / pre-release: load tests against staging
```

- **Container registry:** Alibaba Cloud Container Registry (ACR), HK region — fast pulls from the HK VPS.
- **Deploy:** SSH-based step (`docker compose pull && docker compose up -d`) with health-check gating and automatic rollback to the previous image tag.
- **Lint / static analysis:** `golangci-lint` (Go); ESLint (or oxlint) + `tsc --noEmit` + Prettier (TS/React).
- **Migrations:** `golang-migrate` (or `goose`), executed in CI against test containers so schema drift fails fast.
- **Security scanning:** `govulncheck`, `osv-scanner`/`npm audit`, Dependabot.
- **Coverage:** `go test -cover` + Vitest coverage reported on PRs; pragmatic target of 70–80% on business-logic packages (not a blanket number).

#### 2.4.2 Testing Strategy

| Layer | Tooling | Scope |
|---|---|---|
| Unit (Go) | `testing` + testify; `mockery`/`gomock` for interfaces | Table-driven tests; adapters (payment gateway, LLM, email) mocked via interfaces |
| Unit (Frontend) | **Vitest** + React Testing Library | Components, hooks, i18n/currency formatting logic |
| Integration (Go) | **testcontainers-go** | Real PostgreSQL + Redis in Docker per run; real SQL/migrations/pub-sub; HTTP tests against the actual Fiber app |
| Integration (external APIs) | Gateway sandboxes (Airwallex/PayPal/Brevo/WhatsApp) + WireMock / Go `httptest` mocks | Webhook signature verification and failure paths; never call live gateways in CI |
| E2E | **Playwright** | Critical journeys: browse → wishlist → cart → checkout (mocked payment); sign-up (email/Google/WhatsApp); locale switching; chat WebSocket connect. Runs against staging |
| Smoke | k6 smoke suite or Playwright `@smoke` tag | Post-deploy gate, must complete < 2 min: homepage SSR 200 in both locales, API `/health`, DB/Redis connectivity, product page render, login, cart add, chat WS handshake, payment sandbox reachability |
| Load (**important**) | **k6** (Grafana) | Scenarios in §2.4.3; thresholds codified so failures fail the pipeline |

#### 2.4.3 Load-Test Scenarios (k6)

1. **Browse-heavy baseline** — SSR pages (home, gallery, article, product detail) at target RPS in both locales; validates the < 3s page-load NFR and PostgreSQL query performance.
2. **Checkout funnel** — login → cart operations → shipping-fee calculation → order creation (payment stubbed); catches locking/transaction issues around inventory decrement.
3. **WebSocket chat** — ramp concurrent chat sessions (k6 native WS support) to find the Fiber WS + Redis pub/sub ceiling.
4. **Spike test** — sudden 10× traffic burst (e.g. marketing campaign) to verify graceful degradation.
5. **Soak test** — moderate load for 2–4 hours to catch memory leaks (Go goroutine leaks, SSR memory growth).

- **Thresholds (fail the pipeline, not just report):** API `http_req_duration` p95 < 300 ms; error rate < 0.1%.
- Load tests run against a **staging environment on the same VPS spec as production**.
- **Launch-scale targets (defaults, revisable):** baseline **50 RPS** on SSR pages / **200 concurrent users**; checkout funnel **10 orders/min**; **500 concurrent WebSocket sessions**; spike = 10× baseline. These exist primarily to catch regressions and are cheap to raise later.

---

## 3. Functional Requirements

### 3.1 Culture & Travel

#### 3.1.1 History & Heritage (FR-CT-01)

- Systematically document Jingdezhen's ceramic production history from the late Tang Dynasty to present, including imperial/folk kiln history and industry-chain stories.
- **Content model:** article with rich text supporting mixed multimedia (text, images, embedded video).
- **Taxonomy:** category tree + free-form tags; both filterable/searchable.
- **Search:** keyword search plus tag/category filtering across culture content.
- **Editorial workflow (confirmed): approval required.** Content moves through **draft → submitted for review → approved/published** (with reject-with-comments back to draft). Content Editors author and submit; **only the Super Administrator can approve/publish**. Applies to all content types (articles, destinations, artist profiles).
- All content bilingual (EN / zh-CN) at launch; translations per locale can have independent workflow states.

#### 3.1.2 Destinations & Virtual Tour (FR-CT-02)

- Destination pages for local tourism resources (e.g. Taoxichuan, Sanbao Village) with image galleries, videos, descriptions, opening info.
- **Map & navigation:** embed OpenStreetMap with destination markers; provide site navigation/directions.
- **Virtual Tour (decided):** implemented as curated **image and video galleries** per destination (no 360°/VR panoramas in v1).

#### 3.1.3 Local Lifestyle (FR-CT-03)

- Editorial content showcasing "Jingdrifters" (resident artists), intangible-cultural-heritage craftsmen, and the contemporary ceramic art ecosystem.
- Reuses the article content model (rich text + multimedia + tags).
- Artist profile pages that can be cross-linked from Art Gallery products (see §3.2.1).

### 3.2 Art & E-commerce

#### 3.2.1 Art Gallery / Product Catalog (FR-EC-01)

- **Product model (confirmed):** multi-dimensional SKUs with launch attribute set: **size** (dimensions + display size class), **packed shipping weight** (for shipping-fee calculation, §3.2.3), **craftsmanship/technique**, **glaze type**, **artist**, **edition type** (one-of-a-kind / limited edition + number / open production), **year created**, **kiln/studio**. Stored as a JSONB attribute map plus a few indexed columns (artist, edition type, price) so new attributes require no migrations. **One-of-a-kind artworks** are ordinary SKUs with stock = 1 and no variant axes — no special flow.
- Multi-image galleries and product videos per product/SKU.
- Product detail pages link to artist profiles and digital certificates.
- Category/tag browsing, filtering, sorting, and keyword search.
- **Admin (E-commerce Operator):**
  - Bulk product management: bulk upload (CSV/Excel import + batch image upload) and bulk removal/unpublish.
  - Low-stock alerts: configurable threshold per SKU (**default 2**); alert via dashboard notification and email (Brevo) to E-commerce Operators.
  - Inventory tracking per SKU.

#### 3.2.2 Digital Authentication (FR-EC-02)

- Each artwork gets a digital certificate with a unique code and QR code.
- Buyers scan the QR code → public certificate page showing: certificate ID, artwork details, artist information, provenance records (creation, sale, transfer history).
- Purpose: authenticity assurance and intellectual property protection.
- **Reserved integration:** abstract the certificate service behind an interface so a third-party blockchain-based authentication platform can be plugged in later (see §5.4).
- **Generation (confirmed):** certificates are generated **automatically at product creation** (operators can regenerate); provenance records are appended at sale time.
- **Printable PDF certificate (confirmed):** generated from the same PDF pipeline as itinerary documents, included in the parcel.

#### 3.2.3 Checkout & Orders (FR-EC-03)

- **Cart:** guest cart (persisted locally) + logged-in cart (server-side, merged on login).
- **Checkout eligibility (confirmed): signed-in customers only.** Guests may browse and add items to a local cart, but must register/sign in before placing an order (cart merges on login).
- **Checkout:** address entry, shipping method, payment; mandatory Privacy Policy / Terms of Service checkboxes (see §4.3).
- **Payments (confirmed):**
  - **Airwallex** as PSP for card payments (Visa/MasterCard) — supports mainland-China merchant entities, settles to CNY. Integration via Airwallex Payment Intents / hosted checkout + webhooks.
  - **PayPal** as a parallel payment method via PayPal's own checkout (China cross-border merchant account). PayPal balances are withdrawn into an **Airwallex Global Account** and converted to CNY at Airwallex FX rates (avoiding PayPal's higher conversion fees) — an operational/treasury flow, not a platform feature.
  - Webhook-driven payment confirmation for both gateways.
- **Multi-currency (decided):** supported presentment currencies are **USD, EUR, GBP**; prices displayed and charged in the customer-selected currency.
- **Base currency & pricing (confirmed):** all prices (products, shipping fees) are authored in **CNY** in the CMS and **auto-converted** to USD/EUR/GBP for display and charging.
- **FX rates (confirmed):** fetched **daily from the ECB reference-rate API**, with an operator-configurable markup applied to cover FX volatility — **default 2%**. Converted prices are cached until the next refresh so a customer sees stable prices within a session.
- **Rounding rule (confirmed):** round **up to the nearest .50** for prices under 100, and up to the nearest whole unit at 100 and above (e.g. €183.47 → €184).
- **Settlement currency (confirmed): CNY** — the merchant bank account is in China; Airwallex settles card payments to CNY; PayPal funds reach CNY via the Airwallex Global Account flow above.
- **Order lifecycle:** created → paid → shipped → completed; plus cancelled/refunded (full refunds only; no partial refunds).
- **Refund policy (confirmed, pending legal review of item 4):**
  1. **Before shipment:** free cancellation; full refund (item + shipping) to original payment method.
  2. **Damaged in transit:** full refund or replacement; report within **7 days** of delivery with photos of item and packaging; no return shipment required.
  3. **Change of mind (standard items):** return within **14 days** of delivery (EU consumer-law minimum); item unused and in original packaging; customer pays return shipping; item price refunded after inspection (original shipping fee non-refundable).
  4. **One-of-a-kind / custom artworks:** marked **final sale**, except transit damage (EU withdrawal-right exclusion for unique artworks needs legal confirmation — post-MVP, §8).
  5. **Partial refunds: not supported.** Refunds are always for the full item amount per the rules above; no negotiated partial-refund flows.
  6. **Processing:** refunds issued to the original payment method, in the original payment currency, within **14 days** of approval.
- **Shipping insurance: dropped from v1 scope**; all transit damage is covered by refund rule 2 above.
- **Shipping (confirmed — no logistics integration):** the operator ships via an externally chosen mail service and manually enters the **carrier name + tracking number** on the order in the CMS (order moves to "shipped", customer notified by email via Brevo). Customers track delivery on the mail service's own website; the platform does not integrate carrier APIs or calculate carrier rates.
- **Shipping fee (confirmed): operator-configured per-country, weight-tiered fee table.**
  - E-commerce Operators maintain a shipping-fee table in the CMS keyed by **destination country**; **weight tiers are defined independently per country** (each country has its own tier boundaries and prices).
  - Each SKU must have a **shipping weight** attribute (packed weight).
  - **Built-in shipping fee calculator:** as soon as the customer's mailing address (country) is known — from their saved address book or entered during checkout — the fee is automatically calculated from the country's tier table and the summed weight of cart items, converted to the checkout currency, and added to the order total. The cart/checkout summary updates reactively when the address or cart contents change.
  - **Overweight handling:** if the total order weight exceeds the destination country's heaviest tier, checkout is **blocked** and a message is shown:
    > *"Unfortunately, your order exceeds the maximum weight for standard shipping to your destination. Please contact us by email or phone — we'll be happy to arrange a personalized shipping solution for you."*

    (zh-CN: “很抱歉，您的订单重量超出了标准配送范围。请通过邮件或电话联系我们，我们将为您安排专属配送方案。”) The message includes the contact email and phone number (configurable in CMS settings). Such orders are then **handled manually** outside the standard checkout flow.
  - Countries without a configured fee table are not shippable (checkout blocked with a clear message).
- **Customs disclosure (DDU):** standard text — *"Prices exclude import duties and taxes, which are the recipient's responsibility."* — shown at checkout and in the ToS; final wording pending legal review.
- Order confirmation emails via **Brevo**.
- Customer order history, order detail, and invoice/receipt pages.

### 3.3 Custom Travel & Support

#### 3.3.1 Smart Support — AI Chatbot (FR-TS-01)

- Multi-lingual AI customer service chatbot embedded site-wide, powered by **Qwen3.5** (strong multi-lingual capability, incl. EN/zh translation).
- **Transport (decided): WebSockets** — real-time chat between browser and the Fiber backend (Fiber websocket middleware), with streamed LLM responses; Redis pub/sub for multi-instance fan-out.
- Automatically answers FAQs on visas, transportation, accommodation.
- **Human handoff (decided): live chat** — complex inquiries are escalated within the same WebSocket session to Travel Planners / Customer Service via an **agent console** in the CMS; the agent sees the full conversation transcript. Offline fallback: leave-a-message → email follow-up via Brevo.
- Chatbot languages: **English and Simplified Chinese** at launch, matching site locales; architecture ready for future locales (zh-Hant, ja, fr).
- **Deferred specs (deliberately left open — to be decided during development):** LLM API region & token budget; knowledge base source (CMS-managed RAG vs static system prompt); support hours & offline behavior details.

#### 3.3.2 Custom Itinerary Builder (FR-TS-02)

- Modular, user-friendly customization form tailored to international preferences, implemented as a **4-step wizard** with a progress bar.
- **Submission requires sign-in (confirmed)** — consistent with checkout.
- **Form specification (confirmed):**
  - **Auto-attached (not shown as inputs):** `user_id`, `username`, account email; submission locale (so planners reply in the right language).
  - **Step 1 — Trip basics:**
    - Travel dates: arrival date + **visit duration (days)** (departure auto-computed)
    - Flexible-dates checkbox
    - Number of visitors: adults / children split
  - **Step 2 — Preferences:**
    - Interests (multi-select chips, list **editable in CMS**) — final launch list: pottery-making workshop, kiln/heritage sites, artist studio visits, ceramic shopping, museums, local food, photography, countryside (Sanbao).
    - Budget: range (per person) in USD/EUR/GBP
    - Pace: relaxed / balanced / packed
  - **Step 3 — Services:**
    - Tour guide: none / English-speaking / other language
    - Hotel reservation: yes/no + comfort level (budget / comfort / luxury)
    - Airport/station pickup: yes/no
    - Hands-on ceramic experience session: yes/no
    - Dietary requirements / accessibility needs (free text)
    (Final service-option set for launch.)
  - **Step 4 — Contact & consent:**
    - Preferred contact channel: email (prefilled) / WhatsApp (+ number)
    - Free-text "anything else we should know"
    - Privacy Policy consent checkbox (GDPR)
- **UX behaviors:**
  - **Save draft & resume** for signed-in users; draft/step events feed the dashboard conversion funnel (§3.4.2: views → started → submitted → confirmed).
  - **Summary review screen** before submission.
  - Immediate acknowledgment email via Brevo on submission, stating the **response-time SLA: 24 hours** (CMS-configurable); planner inbox flags requests approaching/past SLA.
- **Backend/CRM (Travel Planner role):**
  - Request inbox with status tracking: **pending → processing → quoted → deposit paid → confirmed** (plus cancelled/closed) — full lifecycle per the deposit flow below.
  - Assignment of requests to planners, internal notes, contact history (CRM management).
  - Data export (CSV/Excel) of form submissions.
  - Automated itinerary confirmation delivery via **email (Brevo)** and/or **WhatsApp Business API**, per customer preference, **including a generated itinerary PDF** (day-by-day plan, included services, pricing summary; branded template, in the customer's locale).
- **Travel deposit & payment (confirmed):**
  - The quoted price is **calculated from the selected options** (guide, hotel level, pickup, experience session, group size, duration), from an operator-configured per-option rate table in the CMS (priced in CNY like products). **Real rates are not yet defined — development proceeds with mocked values**; the rate table is data, so real rates can be entered later without code changes.
  - **Deposit (confirmed): 30%** of the quoted price; **balance due 14 days before arrival** (aligned with the refund tiers — the balance is collected as the 50%-refund window closes).
  - Customer pays the **deposit (or full amount)** online through the same payment stack as e-commerce (**Airwallex/PayPal**, USD/EUR/GBP presentment, CNY settlement); payment link included in the confirmation email/WhatsApp message.
- **Travel cancellation & refund policy (confirmed; legal review post-MVP alongside e-commerce policy):**
  1. **Cancellation by customer, ≥ 30 days before arrival:** full refund of all amounts paid (deposit and any balance).
  2. **15–29 days before arrival:** 50% refund of the total trip price; amounts paid beyond that are refunded in full.
  3. **7–14 days before arrival:** 25% refund of the total trip price.
  4. **< 7 days before arrival or no-show:** no refund.
  5. **Date changes:** one free reschedule if requested ≥ 15 days before arrival (subject to availability); reschedules after that are treated as a cancellation + new booking.
  6. **Cancellation by the operator** (e.g. safety, force majeure on our side): full refund, or free reschedule at the customer's choice.
  7. **Force majeure affecting the customer** (natural disaster, government travel restriction, visa refusal with proof): full refund minus non-recoverable third-party costs (e.g. non-refundable hotel deposits), assessed case by case.
  8. **Processing:** refunds to the original payment method, in the original payment currency, within **14 days** of approval.

### 3.4 Global CMS (Admin Console)

#### 3.4.1 Multi-Role Permission Management (FR-CMS-01)

- RBAC model with predefined roles:

| Role | Key permissions |
|---|---|
| Super Administrator | Everything: user/role management, settings, all modules; **sole approver for content publishing** |
| Content Editor | Culture/travel content CRUD, media library |
| Travel Planner | Itinerary requests, CRM, confirmations, live-chat agent console |
| E-commerce Operator | Products, inventory, orders, certificates |
| Customer Service | Live-chat agent console, customer inquiries, order lookup (read) |

- **Custom roles: not in v1** — the 5 fixed roles suffice; the RBAC table design keeps custom roles easy to add later.
- Admin audit log of sensitive actions (GDPR accountability).
- **2FA (confirmed): TOTP (authenticator app) mandatory for Super Administrator, optional for other staff roles.**

#### 3.4.2 Global Data Dashboard (FR-CMS-02)

Visualized statistics for core data:

- **Analytics approach (confirmed): in-house lightweight analytics** — first-party event endpoint + PostgreSQL, with IP geolocation via a local MaxMind GeoLite2 database. Rationale: Google Analytics is blocked in mainland China (blind spot), first-party aggregation simplifies GDPR, and the dashboard requirements are custom anyway.
- **Traffic analysis:** visits segmented by visitor IP geolocation (country/region), page views, top content.
- **Sales analysis:** cross-border product sales — GMV, orders, by currency/region/product/artist, time series.
- **Itinerary conversion:** funnel from form views → submissions → confirmed, conversion rates over time.
- Time-range filters (day/week/month/quarter/year + custom range) and **CSV export**.

### 3.5 User Accounts

- **Sign-up / sign-in (confirmed):** email + password, **Google** OAuth, and **WhatsApp** login. Note: Google and WhatsApp are blocked in mainland China — email + password is the only reliable method for mainland users; the login UI must not hard-depend on Google/WhatsApp SDKs loading.
- **Profile:** username, email, shipping address(es), optional avatar, etc. **Data schema deferred** — backend code partially exists; schema to be reviewed together later. <!-- TODO: Review existing backend user schema and align profile field list. -->
- **Wishlist (confirmed):**
  - Add to wishlist from the **gallery** (product listing) and the **item detail page**.
  - Add to cart (with quantity selection) from the **gallery**, **item detail page**, and **wishlist**.
  - Cart management: modify quantity, remove single items, and **bulk remove**.
- Registration flow must include mandatory Privacy Policy & ToS checkboxes (§4.3).
- Account area: profile, orders, wishlist, itinerary requests, certificates of purchased items, data export/deletion requests.

---

## 4. Non-Functional Requirements

### 4.1 International UI/UX Design (NFR-01)

- **Visual style (confirmed): modern "New Chinese"** — minimalist layout, generous whitespace, ink/celadon-inspired accent palette, large imagery. <!-- TODO: Designer deliverables (Figma/brand guideline) still to be sourced. -->
- Typography designed for English character spacing and reading habits — do not directly apply Chinese layout logic. Locale-appropriate fonts for zh-CN.
- Responsive design: desktop, tablet, mobile. **Browser support (confirmed):** last 2 versions of Chrome/Edge/Firefox/Safari + current−1 iOS/Android; no IE.
- **Accessibility (confirmed): WCAG 2.1 AA** target for public pages; best-effort for the admin CMS.

### 4.2 Global Performance Optimization (NFR-02)

- Global CDN deployment; page load < 3s in North America, Europe, Asia — measured as **LCP < 3s at p75** per region.
- All images served as **WebP** with **lazy loading** (build-time or on-the-fly conversion for uploads; responsive `srcset`).
- All video delivered via **HLS** adaptive streaming.
- API response target (confirmed): **p95 < 300 ms** (enforced by k6 thresholds, §2.4.3).
- Expected load: launch-scale defaults per §2.4.3 (200 concurrent users baseline); revisit when real traffic data exists.

### 4.3 International Compliance & Privacy (NFR-03)

- Comply with **GDPR** (EU) and **CCPA** (California):
  - Mandatory Privacy Policy & Terms of Service checkboxes in registration and payment flows.
  - **Cookie Consent Banner** with granular consent (necessary/analytics/marketing); analytics blocked until consent.
  - **User data export** (machine-readable) and **account/data deletion** self-service, with order-record retention per legal requirements.
  - Data-processing records, breach-notification procedure, DPA with third-party processors.
- Personal data hosted in **Hong Kong** (VPS + OSS). HK is a third country under GDPR — EU-customer data transfers will need Standard Contractual Clauses (SCCs) or equivalent safeguards documented in the Privacy Policy (legal review post-MVP, §8).
- PCI DSS: card data handled entirely by Airwallex/PayPal hosted elements (SAQ-A scope); no card numbers stored on our servers.

### 4.4 International SEO (NFR-04)

- Semantic URLs (slugs) for all public content, editable per locale in CMS.
- Automatic `sitemap.xml` generation (multi-locale, updated on publish).
- Customizable meta tags (title, description, Open Graph, Twitter cards) per page/locale via CMS.
- `hreflang` alternates, canonical URLs, `robots.txt`, structured data (Product, Article, BreadcrumbList JSON-LD — recommended).
- Server-side rendering of public pages via TanStack Start (see §2.2).

### 4.5 Security (implied)

- HTTPS everywhere; secure session management; rate limiting; input validation; OWASP Top 10 mitigations; webhook signature verification (Airwallex/PayPal/WhatsApp); encrypted secrets management.
- **Backups & availability (confirmed):** nightly `pg_dump` to OSS + weekly full VPS snapshot; **RPO 24 h, RTO 4 h**; uptime target **99.5%** (realistic for a single-VPS deployment; no formal SLA).

---

## 5. Third-Party Integrations

| # | Category | Service | Usage |
|---|---|---|---|
| 5.1 | Payments | **Airwallex** (Visa/MasterCard PSP + Global Account/FX treasury), PayPal | Checkout, multi-currency charging, CNY settlement, refunds, webhooks |
| 5.2 | Email | **Brevo** | Order confirmations, itinerary confirmations, low-stock alerts, transactional email |
| 5.2 | Messaging | WhatsApp Business API via **Meta Cloud API (direct)** | Itinerary confirmations, customer communication; no BSP middleman — pay only per-conversation rates (Meta business verification pending, §8) |
| 5.3 | Maps | OpenStreetMap | Destination maps, site navigation (e.g. via Leaflet/MapLibre) |
| 5.4 | Digital assets | Blockchain authentication platform (reserved) | Pre-reserved API interface; certificate module built behind an adapter interface; vendor selected only when a real business need appears |
| 5.5 | AI | **Qwen3.5** (Alibaba Cloud) | Smart Support chatbot, multi-lingual translation (details deferred, §3.3.1) |

---

## 6. Data Model (High-Level)

Key entities (non-exhaustive): `User`, `Role`, `Permission`, `Article` (+ translations, categories, tags), `Destination`, `ArtistProfile`, `Product`, `SKU`, `MediaAsset`, `Certificate`, `ProvenanceRecord`, `Wishlist`, `Cart`, `Order`, `OrderItem`, `Payment`, `ShippingFeeTable`, `ItineraryRequest`, `CRMNote`, `ChatSession`, `ChatMessage`, `ConsentRecord`, `AuditLog`, `AnalyticsEvent`.

Detailed schema to be produced in the technical design doc; user/profile schema partially exists in backend code and will be reviewed first (§8).

---

## 7. Milestones & Release Plan

**Target: MVP go-live May 2027.** The plan below overlaps milestones and defines a **Minimum Launchable Scope (MLS)** — items outside it may trail without blocking go-live. (Note: milestone date ranges below reflect the original Aug 2026 plan and will be rescaled to the May 2027 timeline separately.) **Deferred past MVP by decision:** legal review of all policy texts (placeholders used at launch) and live payment-merchant onboarding (**Airwallex/PayPal run in sandbox mode**; real payments enabled once accounts are approved).

### M0 — Foundations (Jul 7 – Jul 18)

- Repo setup, GitHub Actions CI/CD skeleton (§2.4), staging + prod environments on HK VPS, OSS/CDN provisioning.
- Fiber API skeleton, TanStack Start app skeleton, PostgreSQL/Redis via Docker Compose, migrations tooling.
- Auth (email+password; Google/WhatsApp OAuth can trail), RBAC model, user profile alignment with existing backend code (§3.5 review happens here).
- i18n framework (locale routing, string catalogs), base design system ("New Chinese" tokens: typography, palette, layout grid).
- **Exit criteria:** deploy pipeline green end-to-end; a signed-in user exists on staging.

### M1 — CMS & Content Platform (Jul 13 – Aug 1, overlaps M0)

- Admin CMS shell + RBAC enforcement; media library with OSS upload, WebP pipeline, FFmpeg→HLS video.
- Articles/destinations/artist profiles with translations, categories/tags, approval workflow (draft → review → Super Admin publish).
- Public site: home, history & heritage, destinations (OSM maps), local lifestyle, search; SEO (slugs, meta, sitemap, hreflang).
- **Exit criteria:** editors can publish bilingual content end-to-end; public pages pass SEO checks; LCP budget met on staging.

### M2 — E-commerce (Jul 27 – Aug 14, overlaps M1)

- Product/SKU model + attributes (§3.2.1), gallery browsing/filtering, bulk import, low-stock alerts.
- Wishlist + cart (incl. bulk ops), CNY→USD/EUR/GBP FX pipeline (ECB + 2% markup + rounding), shipping-fee calculator (per-country weight tiers, overweight block).
- Checkout (signed-in only), **Airwallex** + **PayPal** integration with webhooks, order lifecycle, Brevo order emails, tracking-number entry.
- Digital certificates: auto-generation, QR public page, provenance records, PDF.
- **Exit criteria:** a real sandbox payment completes in all three currencies; certificate scannable; refund flow works.

### M3 — Custom Travel & Support (Aug 10 – Aug 21, overlaps M2)

- Itinerary 4-step wizard with save-draft, review screen, 24h-SLA acknowledgment email.
- Planner CRM: inbox, statuses (pending → … → confirmed), assignment, notes, CSV export, SLA flagging.
- Quote builder with **mocked option rates**, 30% deposit payment via existing payment stack, itinerary PDF generation, confirmation via Brevo (+ WhatsApp if Meta verification completes in time — else trails).
- Chatbot: WebSocket chat + Qwen3.5 FAQ answering + live handoff to agent console (minimal viable version).
- **Exit criteria:** end-to-end request → quote → deposit paid → confirmed with PDF delivered.

### M4 — Compliance, Dashboard & Hardening (Aug 17 – Aug 28, overlaps M3)

- Cookie consent banner, data export/deletion, consent records, policy pages (**placeholder texts**; legal-reviewed versions swapped in post-MVP).
- In-house analytics + GeoLite2; dashboard (traffic, sales, itinerary funnel).
- k6 load tests against staging (thresholds per §2.4.3), performance fixes, backup/restore drill, security pass, 2FA for Super Admin.
- **Exit criteria:** launch checklist green; load-test thresholds pass; smoke suite green on prod.

### Launch — May 2027

DNS/CDN cutover, prod smoke tests, monitoring/alerting live, rollback plan rehearsed.

### Minimum Launchable Scope vs may-trail

| Must be in for May 2027 launch (MLS) | May trail post-launch |
|---|---|
| Content platform + public site + SEO + i18n | Google/WhatsApp OAuth (email+password at launch) |
| E-commerce incl. checkout, certificates, refund flows (sandbox payments) | Live payment processing (pending merchant onboarding) |
| Itinerary wizard + CRM + deposit flow (sandbox) | WhatsApp Business messaging (Brevo email covers confirmations) |
| Compliance basics (consent, export/deletion, placeholder policies) | Chatbot beyond minimal FAQ version; agent-console & dashboard polish |
| Smoke + load-test gates | Legal-reviewed policy texts; bulk-import UX polish; blockchain adapter (design-only) |

---

## 8. Open Items

All major decisions are recorded **inline** in their sections, marked *(confirmed)* / *(decided)*. What remains:

**Deferred by decision (post-MVP or during development):**

1. **Chatbot details** — LLM API region & budget, knowledge base source, support hours (§3.3.1) — decide during development.
2. **Itinerary per-option rates** — mocked during development; real rates entered as CMS data later (§3.3.2).
3. **Live payment merchant onboarding** — Airwallex + PayPal accounts; MVP uses sandbox (§3.2.3, §7).
4. **Legal review (post-MVP):** Privacy Policy & ToS; GDPR SCCs for HK hosting; both refund policies (incl. unique-artwork final-sale exclusion); DDU customs text (§3.2.3, §3.3.2, §4.3).
5. **Meta business verification** for WhatsApp Cloud API (§5.2).
6. **Post-launch KPI baselining** from dashboard data (§1.2).

**Pending input:**

7. **User profile schema** — review existing backend code and align field list (§3.5, §6).
8. **Design deliverables** — Figma/brand assets to be sourced (§4.1).
