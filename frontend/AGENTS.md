# AGENTS.md

> **See also:** the root `../AGENTS.md` for the Go + Fiber backend, repo-wide
> conventions, and the PRD/TDD/REFACTOR-TODO locations.
>
> Guidance for AI agents working on this frontend. Read this first.

This is the **React + TanStack Start frontend** for the Jingdezhen Ceramics
Platform, tracked as a regular subdirectory (`frontend/`) of the root
monorepo and backed by the Go + Fiber API in the sibling `backend/` directory.

> **Stack pivot (recorded in PRD §2.2 / TDD §6 / TDD §12):** the original
> inherited frontend was SolidStart + TanStack _Solid_ libraries. It was
> **deleted wholesale** (commit `2168a18`) and is being rebuilt from scratch on
> React 19 + TanStack Start. The backend (Go + Fiber) is unchanged — only the
> frontend stack changed. See TDD §12 for the decision rationale (ecosystem
> breadth, hiring legibility, TanStack-family coherence, type-safe routing on
> a single HK VPS via Vite/Vinxi rather than Vercel-lock-in).

## Project

**Jingdezhen Ceramics Platform — frontend.** Internationalized culture /
e-commerce / custom-travel storefront + admin CMS, rendered by TanStack Start
SSR and backed by the Go + Fiber API in the sibling `backend/` directory.

**Current phase:** greenfield build complete through **M-F5** (chat,
polish & launch readiness). All 7 milestones (M-F0 through M-F5)
are done — storefront + admin CMS + E2E suite + deploy config, all
gates green. See `frontend/TODO.md` for the milestone plan and
`../docs/REFACTOR-TODO.md` "Frontend" section for completion entries.

- **Source of truth for _what/why_:** `../docs/PRD.md`
- **Source of truth for _frontend design_:** `../docs/TDD.md` §6 (and §7 money,
  §8 state machines, §9 auth, §4.4 SEO)
- **Backend API contract:** the live, Swagger-annotated Go handlers in
  `../backend/internal/api/router.go` + the generated spec at
  `../backend/internal/docs/` (UI at `GET /admin/swagger/*` behind auth).
  When in doubt about a route shape, **read the handler**, not assumptions.

## Stack

- **Language:** TypeScript (strict), React 19 JSX, `moduleResolution: bundler`
- **Runtime / package manager:** Node ≥22 + **pnpm** (v11.19.0 pinned via
  `packageManager`). Lockfile is `pnpm-lock.yaml`.
- **Framework:** **TanStack Start** (the TanStack team's full-stack React
  framework, built on Vite + Vinxi). **SSR enabled** by default for public
  pages (SEO); admin CMS routes are client-rendered. NOT Next.js — see TDD §12
  for why TanStack Start was chosen over Next.js (keep type-safe TanStack
  Router + avoid Vercel-lock-in on a single self-hosted HK VPS).
- **Routing:** **TanStack Router** (file-based, type-safe, code-generated route
  tree via the TanStack Router Vite plugin). This is the _React_ TanStack
  Router — the same family the Solid version used, now on its primary
  (React-first) platform. Type-safe search params via `validateSearch` + zod.
- **Server state:** TanStack Query (client cache / mutations / background
  refetch / optimistic updates). SSR loaders call the API server-to-server for
  initial render and prepopulate the Query cache; Query hydrates on the client.
- **Forms:** hand-rolled with zod validation (matching existing patterns — no
  TanStack Form library needed; `validateSearch` + zod on route search params).
- **Tables:** plain React `AdminTable<T>` component with `Column<T>` type config
  (TanStack Table v9 was installed but its API was incompatible — dep removed;
  see `src/components/admin/ContentTable.tsx`).
- **Styling:** Tailwind CSS v3.4 + PostCSS + autoprefixer. **NOT v4** (v4 is a
  CSS-first config rewrite; deferred). Design tokens (TDD §6 / §12: 青花瓷
  porcelain-cobalt palette over porcelain white, ink text, antique-gold seal
  accent — Stripe-clean layout) as CSS variables in `src/styles/tokens.css`,
  mirrored as hex literals in `tailwind.config.ts` (keep in sync — literals
  are required for Tailwind alpha modifiers).
- **UI primitives:** headless accessible primitives (Radix UI or equivalent),
  styled with our own design tokens via Tailwind. **No full pre-styled UI kit**
  (no Mantine/shadcn-as-kit) — the look is our brand (celadon/ink), not a kit's.
- **Icons:** Phosphor (React) — `@phosphor-icons/react`.
- **HTTP:** a thin typed API client in `src/lib/api.ts` over a `Transport`
  interface. Default (`VITE_API_MODE=mock`) uses the in-process mock transport
  (`src/mocks/`) — routes mirror `api/router.go`, DTOs mirror `models/*.go`,
  errors use the `{error:{code,...}}` envelope with codes from `errors.go`,
  and ALL money math (FX + PRD rounding + shipping tiers) happens inside the
  mock "server", never in UI code. `VITE_API_MODE=live` swaps in the fetch
  transport against the sibling Fiber API.
- **Path alias:** `~/*` → `src/*` (see `tsconfig.json`).
- **E2E:** Playwright (`@playwright/test`) — 6 tests in `e2e/` covering the
  critical user journeys in mock mode. `playwright.config.ts` starts the Vite
  dev server with `VITE_API_MODE=mock`. Run with `pnpm e2e`.
- **Deploy:** nitro `node-server` preset; `Caddyfile` routes `/`→SSR (Node :3000),
  `/api/*`→Fiber (prefix stripped), `/ws`→WebSocket upgrade, `/webhooks/*`→Fiber,
  `/media/*`→Fiber static. See `docs/SMOKE-CHECKLIST.md` for post-deploy
  verification.

## Getting Started

```bash
# from this frontend/ directory
pnpm install          # install deps (generates pnpm-lock.yaml)
pnpm dev              # TanStack Start dev server (SSR, default port 3000)
pnpm build            # production build
pnpm start            # serve the built output
```

**pnpm v11 build-script approval:** pnpm v10+ blocks dependency install scripts
(postinstall) by default. If a native dep is added and pnpm reports
`[ERR_PNPM_IGNORED_BUILDS]`, run `pnpm approve-builds --all` and flip its
`allowBuilds` entry in `pnpm-workspace.yaml`, then `pnpm install`.

The backend API is a **sibling process**. To develop against it:

```bash
# from the repo root — starts PG + Redis + api + worker + chromedp
cd ../backend && make up
# the API listens on :1323 (service `jdz-api`); see ../backend/docker-compose.dev.yml
```

There is **no global `/api/v1` prefix** on the backend. Routes live at the
root: `/auth`, `/profile`, `/cart`, `/catalog/products`, `/artists`,
`/ceramicstory`, `/engage`, `/notifications`, `/admin/...`, `/webhooks/*`,
`/ws`, `/fx/rates`, `/shipping/quote`, `/certificates/:code`,
`/sitemap.xml`, `/robots.txt`. In dev, point the frontend's API base URL at
`http://localhost:1323` (env var / vite proxy — to be established). In
production the reverse proxy maps `/api/*` → backend `/*` (strip prefix).

## Commands

- Dev: `pnpm dev`
- Build: `pnpm build`
- Start (prod): `pnpm start`
- Typecheck: `pnpm typecheck`
- Lint: `pnpm lint`
- Format: `pnpm format`
- Unit tests: `pnpm test` (Vitest, 57 tests)
- E2E tests: `pnpm e2e` (Playwright, 6 tests — starts the dev server automatically)

## Layout (as-built)

```
vite.config.ts        Vite + TanStack Router plugin + nitro (node-server preset) + dev proxy
playwright.config.ts  Playwright E2E config (webServer: pnpm dev + VITE_API_MODE=mock)
Caddyfile             Production reverse proxy (/ → SSR, /api/* → Fiber, /ws → WebSocket)
tsconfig.json
package.json
tailwind.config.ts
postcss.config.js
src/
  styles/tokens.css    design tokens (celadon/ink, spacing, type scale) — source of visual truth
  router.tsx           createRouter(routeTree) + RouterProvider wiring
  routeTree.gen.ts     CODE-GENERATED — do not edit (regenerated by the Vite plugin)
  routes/
    __root.tsx         root shell: <html lang> derived from the URL, 404 fallback
    $locale.tsx        locale layout: validates the $locale segment, mounts providers
                       (i18n, auth, cart, wishlist, toast, chat, realtime) + Header/Footer
                       or AdminShell (isAdmin regex check); skip-to-content link
    $locale/           public routes, locale-segment (en-US | zh-CN). NOTE:
                       TanStack Router uses `$param` directories — not Next.js
                       -style `[locale]`
      index.tsx        landing (horizontal magazine)
      catalog/         products list + $slug detail (gallery, SKU, wishlist, cart)
      artists/         artist profiles
      ceramicstory/    History content (index + $slug)
      engage/          Destinations/Lifestyle (index + $slug)
      cart/
      checkout/        address form + payment gateway (Airwallex/PayPal sandbox)
      orders/
      wishlist/
      itinerary/       custom-travel wizard (5-step + consent)
      account/         orders, wishlist, addresses, profile, privacy, 2FA, notifications
      certificates/    $code public cert page + QR
      notifications/   notification list + unread badge
    $locale/auth/      login (+ 2FA verify step), signup, activate, forgot, reset, 2fa-enroll
    $locale/admin/     client-rendered CMS, RBAC-gated (5 staff roles + 16 perm keys)
      dashboard, content/{stories,activities,artists,products}, media, orders,
      itineraries, chat, settings, users, audit, certificates
  components/
    layout/            Header, Footer, Spine (magazine), AdminShell (sidebar + topbar)
    common/            Button, ButtonLink, Spinner, Badge, FieldError, QuantityStepper,
                       HeartButton, EmptyState, ConsentBanner, Toaster, Breadcrumbs, ui.tsx
    admin/             AdminTable<T>, ContentTable, ContentBlocks, MediaPicker
    chat/              ChatWidget (floating bubble + panel)
    cards.tsx          ProductCard
    artwork/           PorcelainFigure (seeded SVG vessel/landscape generator)
    ornaments/         SealMark, WaveBand, PetalScatter (decorative SVGs)
    seo/               JsonLd, buildSeoHead helper
  lib/
    api.ts             API client (Transport interface, LiveTransport/MockTransport, API_MODE)
    auth.tsx           auth context (token store, login, 2FA, signup, logout, RBAC helpers)
    types.ts           TS interfaces for the domain (Product, SKU, Cart, Order, Address,
                       Itinerary, Artist, Activity, CeramicStory, ChatSession, Certificate —
                       money as bigint/integer, never JS number)
    i18n.tsx           i18n context (useI18n / t) + catalog loader
    cart.tsx            cart context (localStorage, merge on login)
    wishlist.tsx        wishlist context (localStorage)
    chat.tsx            chat context (mockChat registry, cross-tab sync, ChatProvider)
    realtime.tsx        RealtimeProvider (WebSocket /ws, backoff, push → toast)
    consent.tsx         consent context (localStorage, per-kind)
    analytics.ts        trackPageview, trackItineraryFormView (fire-and-forget)
    money.ts            display formatter (Intl.NumberFormat; no math)
    seo.ts              buildSeoHead (canonical, hreflang, OG, Twitter)
    utils.ts            cn, SUPPORTED_CURRENCIES, formatDate, seededRandom, isLocale
  i18n/
    en-US.ts            UI string catalog (~700 keys)
    zh-CN.ts            UI string catalog (~700 keys)
  mocks/
    data.ts             mock dataset (PRODUCTS, ARTISTS, STORIES, DEMO_USERS, CERTIFICATES)
    transport.ts        MockTransport (routes mirror api/router.go, money math server-side)
    engine.ts           mock chat engine helpers
e2e/                    Playwright E2E tests (6 specs, waitForHydration helper)
docs/                   SMOKE-CHECKLIST.md (10-section post-deploy verification)
public/
  favicon.svg
```

## Conventions

- **Routing:** TanStack Router file-based routes. Each route file:
  ```tsx
  export const Route = createFileRoute('/path')({
    beforeLoad, // auth guard (throw redirect to /auth/login)
    validateSearch, // zod schema for type-safe search params (filters, wizard step)
    loader, // server-to-server API call, prepopulates Query cache
    component,
  })
  ```
  Consume loader data with `Route.useLoaderData()`. Use `<Link to params search>`.
  **Do not edit `routeTree.gen.ts`** — it regenerates on dev/build.
- **Data fetching:** SSR loaders (`Promise.all` parallel API calls — TDD
  §11.1 #1) for initial render; `useQuery`/`useMutation` (TanStack Query) for
  client refetch + mutations. No inline mock data in production routes.
- **Auth (current backend reality — READ THIS):** the backend issues a
  **single HS256 access JWT, 30-day expiry**, validated from the
  `Authorization: Bearer <token>` header. There is **no refresh-token rotation
  yet** (deferred to a post-frontend milestone per TDD §5.1). So the frontend
  must store the token client-side (`localStorage`) and attach it as a Bearer
  header on every API call. **Do NOT** build assuming an httpOnly refresh
  cookie / rotate-on-401 flow exists — that's the _target_, not the current
  state. Build the auth context + API interceptor so refresh-rotation slots in
  without redesign later. Login may require a **2FA verify step** (TOTP) for
  users with 2FA enabled (mandatory for `super_admin`); the flow is
  `POST /auth/login` → pending-2FA token → `POST /auth/2fa/verify`. Google
  OAuth (`POST /auth/google`) also gates on 2FA when enabled. Backup-code
  entry is a separate rate-limited endpoint.
- **i18n (TDD §6):** two layers, do not conflate.
  - **Content i18n (backend-owned):** the backend stores per-entity
    translation rows (one per locale, status on the translation row). The
    frontend **never translates content** — it passes the locale to the API
    and renders what comes back. Locales are **`en-US` and `zh-CN`** (BCP 47),
    default `en-US` (the backend's `models.SupportedLocales`).
  - **UI string catalogs (frontend-owned):** static UI labels (buttons, nav,
    form fields, error messages, aria-labels) live in per-locale TS catalogs
    (`src/i18n/en-US.ts`, `zh-CN.ts`) — **NOT in the database** (TDD §3.2). A
    `useI18n()` context provides `t()`. Locale switching is URL-driven via
    the `[locale]` route segment; a toggle navigates to the sibling locale
    path and sets a `locale` cookie for SSR.
  - Public routes live under `src/routes/[locale]/...` with the locale param
    validated against `["en-US","zh-CN"]` at the route level; out-of-range
    locale → redirect to `en-US` with the same sub-path.
- **Money (TDD §7):** all money is **minor units** (`BIGINT` fen/cents/pence)
  as **integers** in the API. Never use JS `number` for money (float) — use
  `bigint` or a money helper. FX + rounding (`<100 → ceil 0.50; ≥100 → ceil
1.00`, PRD §3.2.3) is done **server-side**; the frontend only **formats for
  display** via `Intl.NumberFormat(locale, { style: 'currency', currency })`
  on the presentment values the API returns. Do not re-derive prices, cart
  totals, or order totals client-side. Presentment in USD/EUR/GBP, base CNY.
- **Cart (TDD §6):** guests keep cart in `localStorage`; on login, merge via
  `POST /cart/merge`. Locale + currency in cookie + context.
- **SEO (TDD §6, PRD §4.4):** meta per entity from the API (`meta_title`,
  `meta_description` on translations); `hreflang` + canonical in the locale
  root layout; `sitemap.xml` is served by the backend (`GET /sitemap.xml`);
  JSON-LD components (Product, Article, BreadcrumbList) built frontend-side.
- **Errors (TDD §4.3):** the backend serializes domain errors via a central
  error-mapper into `{ error: { code: string, message: string, details? } }`
  with the right HTTP status. Domain error codes are centralized in
  `../backend/internal/models/errors.go` (e.g. `ErrNotFound`,
  `ErrUnauthorized`, `ErrInsufficientStock`, `ErrConflict`). The frontend API
  client parses this envelope into a typed `ApiError` keyed by `code`
  (NOT by HTTP status — codes are stable and domain-specific). Surface via
  toast + inline + form-field error layers; `ErrUnauthorized`/token-expired →
  kick to login (later: refresh).
- **Components:** React function components + hooks; `Suspense`/`ErrorBoundary`
  for async. Headless accessible primitives (Radix) for interactive widgets
  (Dialog, Select, Combobox, Tooltip, Tabs, Toast, Menu) — add as needed, not
  up front.
- **Commit style:** Conventional Commits (`feat(scope): …`, `fix(scope): …`),
  matching the backend repo.

## Constraints

- **Do not edit `src/routeTree.gen.ts`** — it is code-generated.
- **Do not call live payment gateways / OSS / email from the frontend.**
  The backend fronts all external services behind adapters; the frontend only
  talks to the Fiber API. In sandbox/dev the backend uses mocks.
- **Do not re-implement money / FX / shipping math client-side.** The API
  returns presentment totals and shipping quotes; render them.
- **Do not commit `.env` / secrets.** API base URL + any keys are env-only.
  The root `.gitignore` covers `node_modules/`, `dist/`, `.tanstack/`,
  `*.env`, `.env.*` (with `!.env.example`) — these apply to `frontend/` too.
- **Backend is the contract source.** When a route shape is unclear, read the
  Go handler in `../backend/internal/modules/<module>/` and the router in
  `../backend/internal/api/router.go`, or the Swagger UI at
  `GET /admin/swagger/*` (needs a JWT). Do not guess.
- **One repo.** `git` operations target the root `refactor` repo (origin
  `https://github.com/shujiejune/jingdezhen-ceramics-refactor.git`).
- **Do not bump to Tailwind v4** (CSS-first config rewrite — deferred) or
  **zod v4** (breaking API rewrite — stay v3.x). The stack is locked as above.
