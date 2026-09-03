# Frontend Build Plan — TODO

> Takes the hi-fi prototype (commit `7ca45e3` + magazine UI pass) to production per `frontend/AGENTS.md`, TDD §6/§7/§5.3, PRD §3–§4/§7. Milestones mirror the PRD §7 tracks, prefixed `M-F*` for the frontend; `M-FD` is the user-driven design-revision track and runs in parallel whenever design feedback lands. Track completion here and mirror it in `docs/REFACTOR-TODO.md` → "Frontend".
>
> Status: ☐ todo · ◐ partial · ☑ done

## M-FD — Design revision track (user-driven, runs in parallel)

- [x] Heritage index → horizontal magazine chapters (spine chrome) — reuse `useLoopScroller` + `Spine`
- [x] Gallery browsing — record-crate flip experiment built, then **reverted per user review (2026-08-15): plain responsive grid** + sticky filter bar + pagination; the crate engine (center/deck modes) stays available in `lib/loop-scroller.ts` if revisited
- [x] Apply post-review UI tweaks — round 1: visible blue text on dark-panel CTAs (new `invert` button variant), content-fit panel widths (no right blank), panel CTAs moved to bottom rows, compact CONTACT tail (footer under the newsletter row)
- [x] Guard `src/styles/tokens.css` ↔ `tailwind.config.ts` hex/radius twins stay in sync — `src/styles/__tests__/tokens-sync.test.ts` (already caught + fixed real drift: family `50` shades, gold alias)

## M-F0 — Foundations & tooling (no backend dependency)

- [x] ESLint (typescript-eslint, react-hooks, react-refresh) + Prettier configs; `lint` / `format` scripts — flat config; react-hooks v6 `set-state-in-effect`/`refs` flow rules off with rationale (fetch-in-effect is deliberate until TanStack Query in M-F1) in `package.json`
- [x] Vitest + React Testing Library setup (`vitest.config.ts`, jsdom, explicit cleanup, `test` scripts)
- [x] Unit tests (51): money formatting, i18n catalog key-parity, loop-scroller wrap/nearest-panel math (extracted as pure helpers), ApiError + errorKey mapping + contract envelopes (flat/paginated/2FA/204), seededRandom determinism, tokens-sync drift guard, RTL Button smoke
- [x] Root `errorComponent` (branded 500 + dev error detail + reset) and pending spinner on root + router defaults across routes
- [x] A11y baseline: `prefers-reduced-motion` (CSS kill-switch + loop-scroller skips parallax/reveal and snaps instantly), keyboard arrows for loop, spine dots are focusable buttons with aria labels
- [x] Frontend CI job in `.github/workflows/ci.yml`: pnpm 11.19 + Node 22 (cached) → lint → format:check → typecheck → vitest → build
- [x] `frontend/README.md` quickstart (dev commands, mock vs live, demo accounts, scripts table, layout map)

**Acceptance: MET** — lint/format/typecheck/51 tests/build all green locally; CI job wired (first GitHub run pending push).

## M-F1 — Live API integration (storefront)

Contract realities from the backend inventory (absorb **all** of these):

- [x] LiveTransport: flat success bodies (no `{data:…}` wrapper); `PaginatedResponse {data,page,limit,total,total_pages}`; `{data:[…]}` wrappers (consent history, media assets); CSV exports & PDF 302s consumed as direct links
- [x] ApiError adapter for the **real** envelope: `{"message"}` only today (+ Fiber plain-text 404/405/panic bodies), 401 `{"error":{code,message,pending_token}}` for the 2FA login challenge; map status+message → typed keys; treat **204 as success** (analytics drop, mark-read)
- [x] Dev proxy: vite `server.proxy` `/api/*` → Fiber `:1323` (strip prefix) + `/media` passthrough; origin-prefix helper for relative `/media/...` URLs (local storage mode)
- [x] TanStack Query client + SSR-loader → cache hydration; migrate catalog / product / content / cart / wishlist reads off direct-context fetches
- [x] Real auth: signup returns 201 with `access_token:""` → activation page consuming email link (`POST /auth/activate {token}`); forgot/reset-password pages; Google OAuth receiver routes `/login/success | /login/2fa | /login/2fa/enroll | /login/error` (backend 307s to `{CLIENT_ORIGIN}`)
- [x] 2FA complete: verify, pending-enroll (QR + secret), pending-confirm (backup codes shown once)
- [x] `?locale=&currency=` on every read; cookie ↔ provider sync (existing `jdz-currency` invalidate path) — cart mutations fixed to carry currency in `params`; artists loader fixed to thread `loaderCurrency()`
- [x] Keep `mock` mode as dev/test fixture (`VITE_API_MODE`); contract tests asserting both envelopes parse (`src/lib/__tests__/api-contract.test.ts`, 12 tests)

**Acceptance:** MET in mock mode — all flows browser-verified (signup+activation, 2FA enrollment + verify, locale switch, currency switch with EUR cart totals). Live backend run (`make up`) pending: the dev proxy and `LiveTransport` are wired but the backend Docker stack needs to be running for a live round-trip. The mock path remains as a dev/test fixture.

## M-F2 — Commerce & account depth

- [x] Real media: per-entity gallery endpoints (`GET {entity}/:id/media`) wired into detail pages; `srcset`/lazy via OSS image-process params; hls.js video player (may-trail if no video assets) — product detail gallery wired with `mediaImageUrl`/`mediaSrcSet` + `loading="lazy"` (falls back to PorcelainFigure in mock mode); per-entity gallery endpoint + hls.js deferred to M-F4/M-F5
- [x] Address book CRUD + set-default (`/profile/addresses`) — full CRUD in account page with inline AddressForm; set-default; delete with confirm
- [x] Checkout live: reactive shipping-quote UX (unshippable / overweight / ok states), gateway `hosted_url` redirect, return handling + order polling to `paid` — hosted_url redirect for live mode; mock falls through to sandbox; polls `GET /orders/:id` every 2s for 30s after sandbox payment or gateway return (`?order_id=&paid=1`)
- [x] Order detail: status timeline, cancel-unpaid, carrier/tracking display, refund policy note — pre-existing from prototype; timeline + cancel + tracking all render
- [x] Certificates: verify page, QR PNG link, PDF download (302 + `?download=1`; graceful "not yet generated" 404 state) — QR generated client-side via `qrcode` lib; PDF download button with "not yet generated" disabled state in mock mode
- [x] Itinerary: quote view (line items, totals, 30% deposit), quote PDF link, `pay-deposit` (gateway redirect + poll to `deposit_paid`), my-journeys status pages — new `$id` detail route with quote breakdown + pay-deposit; "View quote" link on list; mock auto-generates quote on submit
- [x] Account: profile update, 2FA enroll/confirm/backup-codes, consent history, GDPR data export download + delete-account flow — consent history section; GDPR export via `GET /profile/export`; delete via `POST /privacy/delete-account` with DELETE confirm; 2FA enroll already shipped in M-F1
- [x] Notifications: list + unread badge + mark-read (poll `/notifications/unread-count`; WS push deferred to M-F5) — new `/$locale/notifications` route; header bell with 30s poll; mark-all-read + click-to-mark-read

**Acceptance:** MET in mock mode — account page (addresses, consent, GDPR), notifications, certificate QR/PDF, itinerary quote/deposit all browser-verified. Sandbox payment + EUR/GBP checkout already verified in M-F1. Live backend run pending (`make up`). WS notification push deferred to M-F5.

## M-F3 — Compliance, SEO & analytics

- [x] Cookie consent banner: granular kinds → public `POST /consent` (`doc_version`), persisted state, re-open from footer ("Cookie preferences")
- [x] Analytics events: `pageview` on route change + `itinerary_form_view` on wizard mount (dashboard funnel contract); 204 = silently dropped, never retried
- [x] SEO: `hreflang` from the API `alternates` maps, canonical + OG/Twitter meta per entity, JSON-LD completion (Product / Article / BreadcrumbList everywhere), sitemap cross-link in footer
- [x] Policy placeholder pages (privacy / terms / cookies) with consent-version framing
- [ ] PRD §4.4 audit checklist run in both locales

**Acceptance:** SEO audit passes per locale; consented events appear in the admin dashboard.

## M-F4 — Admin CMS (client-rendered, RBAC)

- [ ] Admin shell: route group under `$locale/admin` (client-rendered), staff guard + permission-keyed nav (`content.write`, `product.write`, `order.read`, `itinerary.read`, `dashboard.view`, `settings.manage`, …), 2FA-enforced login path
- [ ] Adopt TanStack Table (all lists) + TanStack Form + zod (CMS/wizard forms)
- [ ] Content CMS ×4 (ceramicstory / engage / artist / product translations): bilingual list/detail/edit, tag assignment (key-based), workflow submit; super-admin approve / reject / unpublish
- [ ] Media library: presign/upload (or local upload), asset registry list, per-entity galleries (attach / detach / reorder)
- [ ] Products & SKUs: CRUD, attributes JSONB editor, nested SKU create + flat SKU update, bulk CSV import UX with per-row `BulkImportSummary` report
- [ ] Orders: list/detail, ship (carrier_name + tracking_number), complete, full-only refund
- [ ] Itinerary CRM: inbox (status / assignee / SLA filters), assign, notes, quote builder send (`line_items`, `pay_full`), confirm, refund-deposit, CSV export
- [ ] Settings: shipping fee tiers CRUD, itinerary option rates CRUD, FX refresh trigger
- [ ] Dashboard: traffic / sales / funnel reports (+ range presets day…year + from/to), CSV downloads; users & roles; audit log viewer (+CSV); certificates admin (list/regenerate)

**Acceptance:** each staff role sees exactly its modules; editor → submit → super-admin publish round-trip works live.

## M-F5 — Chat, polish & launch readiness

- [ ] Chat widget + agent console — **backend-blocked** (see dependencies); build against mock frames, integrate when unblocked
- [ ] WS notification push (same blocker): reconnect/backoff, toast on push; poll fallback until then
- [ ] Performance: admin bundle code-split, image loading audit, font strategy, LCP < 3s target (PRD §4.2)
- [ ] Accessibility: WCAG 2.1 AA pass on public pages
- [ ] Playwright E2E: browse → wishlist → cart → checkout (mock gateway), signup + activate, locale switch, itinerary wizard submit, certificate verify
- [ ] Deploy: node-server preset; Caddy reverse proxy (`/api/*` strip → Fiber, `/media`, `/ws` upgrade); env matrix (`VITE_API_MODE=live`, `SITE_BASE_URL`/`CLIENT_ORIGIN` alignment); post-deploy smoke checklist
- [ ] Docs sync: `frontend/AGENTS.md` as-built updates, REFACTOR-TODO checkboxes, TDD §12 notes

**Acceptance:** staging deploy passes the smoke suite; E2E suite green.

## Backend dependencies (external — track in `docs/REFACTOR-TODO.md`, not here)

- `/ws` browser auth (query-token or subprotocol) — browser `WebSocket` cannot set the `Authorization` header the middleware requires; blocks WS push + chat
- Chat sessions / frames + agent console endpoints (TDD §5.3 aspiration — none exist today)
- Central error-mapper with stable `code`s (TDD §4.3) — until it lands, M-F1 ships the message-mapping adapter
- CORS `CLIENT_ORIGIN` entries for the frontend origin(s) in dev compose

## May-trail (PRD §7 MLS — do not block launch)

Google/WhatsApp OAuth providers (receiver routes ship in M-F1 regardless) · live payment processing (sandbox until merchant onboarding) · WhatsApp Business messaging · chatbot beyond the minimal version · legal-reviewed policy texts · bulk-import UX polish · hls.js video delivery
