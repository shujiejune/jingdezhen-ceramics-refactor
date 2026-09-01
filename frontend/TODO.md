# Frontend Build Plan — TODO

> Takes the hi-fi prototype (commit `7ca45e3` + magazine UI pass) to production per `frontend/AGENTS.md`, TDD §6/§7/§5.3, PRD §3–§4/§7. Milestones mirror the PRD §7 tracks, prefixed `M-F*` for the frontend; `M-FD` is the user-driven design-revision track and runs in parallel whenever design feedback lands. Track completion here and mirror it in `docs/REFACTOR-TODO.md` → "Frontend".
>
> Status: ☐ todo · ◐ partial · ☑ done

## M-FD — Design revision track (user-driven, runs in parallel)

- [ ] Heritage index → horizontal magazine chapters (spine chrome) — reuse `useLoopScroller` + `Spine`
- [ ] Gallery → museum-window / record-crate browsing (freakmag-style flip panels)
- [ ] Apply post-review UI tweaks (panel pacing, parallax intensity, spine density, family-color saturation)
- [ ] Guard `src/styles/tokens.css` ↔ `tailwind.config.ts` hex/radius twins stay in sync (single source of truth note in both files)

## M-F0 — Foundations & tooling (no backend dependency)

- [ ] ESLint (typescript-eslint, react-hooks, react-refresh) + Prettier configs; `lint` / `format` scripts in `package.json`
- [ ] Vitest + React Testing Library setup (`vitest.config.ts`, jsdom, `test` script)
- [ ] Unit tests: money formatting (minor units, locales, `.50`/whole display), i18n catalog key-parity (en-US ↔ zh-CN compile-time + runtime test), `useLoopScroller` wrap/shortest-path math, ApiError adapter, `seededRandom` determinism
- [ ] ErrorBoundary + 500 route; root 404 polish; loading/error component-state audit across routes
- [ ] Responsive + a11y baseline: focus-visible rings, keyboard nav for spine dots & loop scroller, aria labels, reduced-motion media query for parallax/reveal
- [ ] Frontend CI job in `.github/workflows/ci.yml` (pnpm install → lint → typecheck → vitest → build; Playwright added in M-F5)
- [ ] `frontend/README.md` quickstart (dev commands, mock vs live modes, demo accounts, screenshots dir)

**Acceptance:** CI green on the frontend job; `pnpm lint && pnpm typecheck && pnpm test && pnpm build` clean locally.

## M-F1 — Live API integration (storefront)

Contract realities from the backend inventory (absorb **all** of these):

- [ ] LiveTransport: flat success bodies (no `{data:…}` wrapper); `PaginatedResponse {data,page,limit,total,total_pages}`; `{data:[…]}` wrappers (consent history, media assets); CSV exports & PDF 302s consumed as direct links
- [ ] ApiError adapter for the **real** envelope: `{"message"}` only today (+ Fiber plain-text 404/405/panic bodies), 401 `{"error":{code,message,pending_token}}` for the 2FA login challenge; map status+message → typed keys; treat **204 as success** (analytics drop, mark-read)
- [ ] Dev proxy: vite `server.proxy` `/api/*` → Fiber `:1323` (strip prefix) + `/media` passthrough; origin-prefix helper for relative `/media/...` URLs (local storage mode)
- [ ] TanStack Query client + SSR-loader → cache hydration; migrate catalog / product / content / cart / wishlist reads off direct-context fetches
- [ ] Real auth: signup returns 201 with `access_token:""` → activation page consuming email link (`POST /auth/activate {token}`); forgot/reset-password pages; Google OAuth receiver routes `/login/success | /login/2fa | /login/2fa/enroll | /login/error` (backend 307s to `{CLIENT_ORIGIN}`)
- [ ] 2FA complete: verify, pending-enroll (QR + secret), pending-confirm (backup codes shown once)
- [ ] `?locale=&currency=` on every read; cookie ↔ provider sync (existing `jdz-currency` invalidate path)
- [ ] Keep `mock` mode as dev/test fixture (`VITE_API_MODE`); add contract tests asserting both envelopes parse

**Acceptance:** full browse → cart → checkout (mock gateway) → orders, plus auth/2FA, against the live backend (`make up`) with zero mock imports on the live path.

## M-F2 — Commerce & account depth

- [ ] Real media: per-entity gallery endpoints (`GET {entity}/:id/media`) wired into detail pages; `srcset`/lazy via OSS image-process params; hls.js video player (may-trail if no video assets)
- [ ] Address book CRUD + set-default (`/profile/addresses`)
- [ ] Checkout live: reactive shipping-quote UX (unshippable / overweight / ok states), gateway `hosted_url` redirect, return handling + order polling to `paid`
- [ ] Order detail: status timeline, cancel-unpaid, carrier/tracking display, refund policy note
- [ ] Certificates: verify page, QR PNG link, PDF download (302 + `?download=1`; graceful "not yet generated" 404 state)
- [ ] Itinerary: quote view (line items, totals, 30% deposit), quote PDF link, `pay-deposit` (gateway redirect + poll to `deposit_paid`), my-journeys status pages
- [ ] Account: profile update, 2FA enroll/confirm/backup-codes, consent history, GDPR data export download + delete-account flow
- [ ] Notifications: list + unread badge + mark-read (poll `/notifications/unread-count`; WS push deferred to M-F5)

**Acceptance:** sandbox payment completes in USD/EUR/GBP; certificate scannable; deposit flow reaches `deposit_paid`.

## M-F3 — Compliance, SEO & analytics

- [ ] Cookie consent banner: granular kinds → public `POST /consent` (`doc_version`), persisted state, re-open from footer ("Cookie preferences")
- [ ] Analytics events: `pageview` on route change + `itinerary_form_view` on wizard mount (dashboard funnel contract); 204 = silently dropped, never retried
- [ ] SEO: `hreflang` from the API `alternates` maps, canonical + OG/Twitter meta per entity, JSON-LD completion (Product / Article / BreadcrumbList everywhere), sitemap cross-link in footer
- [ ] Policy placeholder pages (privacy / terms / cookies) with consent-version framing
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
