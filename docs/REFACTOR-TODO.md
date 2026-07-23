# Backend Refactor TODO

Tracking the transition of the existing `backend/` codebase (former "Learning & Communication Platform") to the PRD targets (`docs/PRD.md`, v0.17). Milestone references (M0–M4) per PRD §7.

## ✅ Done — Deleted redundant modules (not in PRD)

- [x] `internal/modules/course` + `models/course.go`, `models/quiz.go`, `models/assignment.go` (online courses, quizzes, assignments)
- [x] `internal/modules/forum` + `models/forum.go` (community forum)
- [x] `internal/modules/note` + `models/note.go` (private study notes)
- [x] `internal/modules/portfolio` + `models/portfolio.go` (student portfolio works)
- [x] `models/badge.go` + badge fields in `ProfileData` (gamification)
- [x] Migrations: 000001, 000003, 000012–000033, 000035–000036, 000038
- [x] Cleaned dangling references: `main.go`, `router.go`, gallery (note deps), notification types, `Artwork.NoteCount`
- [x] `go build ./...` and `go vet ./...` pass

## M0 — Foundations

### Migration baseline
- [x] Squash remaining migrations (000002, 000004–000011, 000034, 000037) into a fresh `000001_baseline` (pre-production, gaps in numbering)
  - Fixes applied during squash: `events` table renamed to `activities` (code queries `activities`); `ceramic_stories.slug` column added (model/repo use it but no migration created it); `articles.id` changed to `BIGSERIAL` (was plain `BIGINT` with no generator); tables reordered so FK targets exist (`artworks` before `artwork_images`)
  - ⚠️ Existing dev databases must be reset (`migrate drop` or recreate the DB) — the old `schema_migrations` version numbers no longer exist
  - ⚠️ Known bug spotted (not fixed, pre-existing): `cs_repository.go` `ORDER BY … start_year DSC` — typo, should be `DESC`; will fail at query time
- [x] Validate baseline against real PostgreSQL (no Docker available in this session) — first `make migrate-up` or testcontainers run
- [ ] Switch migration workflow into CI (run against testcontainers, PRD §2.4.1)

### User / Auth (PRD §3.4.1, §3.5)
- [x] Replace role model `admin/normal_user/guest` with PRD RBAC: customer + `super_admin`, `content_editor`, `travel_planner`, `ecommerce_operator`, `customer_service` (roles/permissions tables) — migration `000002_rbac`, models/rbac.go, middleware `RequirePermission`/`RequireRole`/`HasRole`, repo `GetUserRoles`/`AssignRole`, service `AdminAssignRole`, admin user routes wired under `/admin/users`
- [ ] Add shipping address book (multiple addresses per user; country drives shipping calculator)
  - [x] Shipping address book implemented: migration `000003_user_addresses`, `models/address.go`, `address` module (handler/service/repository), routes under `/profile/addresses`; one-default-per-user enforced via partial unique index + tx
- [x] Add preferred locale (`en-US`/`zh-CN`) and preferred currency (USD/EUR/GBP) to profile — migration `000004_user_preferences` adds columns + CHECK constraint; `User`/`UserUpdateData` updated; all 4 scan paths fixed (latent double-ProfileData scan bug removed); validator `oneof=USD EUR GBP` for clean 400s
- [x] Add consent records (Privacy Policy / ToS acceptance with timestamp & version — GDPR) — migration `000005_consent_records` (append-only, nullable user_id for anonymous, 4 kinds, IP hashed via HMAC); `models/consent.go`; `consent` module (repo/service/handler); `POST /consent` public, `GET /profile/consent` + `GET /profile/consent/:kind` protected; `CONSENT_HMAC_KEY` in config
- [x] Add TOTP 2FA (mandatory for Super Admin, optional for staff) 
  — migration `000006_user_2fa` (encrypted secret at rest via AES-GCM, two-phase enroll→confirm); `pkg/utils/crypto.go` AES-GCM; `twofa` module (repo/service/handler); login gates on enabled 2FA via `Err2FARequired` + 5-min pending JWT, `/auth/2fa/verify` completes login; `POST /profile/2fa/{enroll,confirm}`, `DELETE /profile/2fa`; `TWO_FA_ENCRYPTION_KEY` in config. 
  - **Super-admin must-enroll enforced**: a super_admin with no 2FA is blocked at login (`Err2FAEnrollmentRequired`), walked through `/auth/2fa/pending-enroll` → `/auth/2fa/pending-confirm` (15-min pending token; confirm enables 2FA AND mints the real JWT). 
  - **Google OAuth login is 2FA-gated too** (shared `challenge2FAOrMint` helper). 
  - **Backup codes**: migration `000009_user_2fa_backup_codes` (one-time, stored SHA-256 hashed with app-key pepper); generated at confirm (shown ONCE), accepted at `/auth/2fa/verify` (TOTP tried first, then a backup code is consumed atomically); `POST /profile/2fa/backup-codes/regenerate` + `GET /profile/2fa/backup-codes` (remaining count); `Disable` clears backup codes; unit tests for hashing/normalization
- [ ] Add WhatsApp OAuth provider (schema already generic via `auth_provider`) — *may trail post-MVP*
- [x] Replace AWS SES sender with **Brevo** (keep `email.ServiceInterface`; remove AWS config/deps) 
  — `pkg/email/brevo_sender.go` REST adapter; `ses_sender.go` removed; AWS SDK dropped from go.mod via `go mod tidy`
- [x] Extend `config.go`: Brevo, Airwallex, PayPal, Qwen, OSS, Redis; remove AWS fields — `.env.example` documents all keys; Redis added to `docker-compose.dev.yml`
- [x] GDPR self-service: user data export + account deletion endpoints 
  — migration `000010_user_gdpr` (adds `users.deleted_at`; anonymize-in-place so order-record retention keeps referential integrity); `models/privacy.go` (`UserDataExport` bundle + `DeleteAccountRequest`); `privacy` module (repository spans users/addresses/consent/2fa/favorites/notifications tables; service captures email pre-erasure then anonymizes + enqueues Brevo confirmation via `email:send` queue); `GET /profile/export` (synchronous JSON, `Content-Disposition` download; async job + download link deferred to M2/M3 when order history makes payload large); `POST /privacy/delete-account` (`{"confirm":"DELETE"}` body guard; irreversibly nulls email/nickname/password_hash/avatar/tokens/auth_provider_id, sets `is_active=false`+`deleted_at`; CASCADE-purges addresses/2FA/favorites/notifications; SET NULL on consent_records + content authorship for audit). Login already rejects erased stubs via `is_active=false`. JWT blocklist (deleted-user token invalidation) deferred to M4 security pass

### Infrastructure
- [x] Add Redis (sessions, cache, pub/sub, Asynq job queue) — `redis:7-alpine` in `docker-compose.dev.yml` (port 6379, healthcheck); `internal/platform/redis` go-redis client; `internal/platform/jobs` Asynq enqueue client + worker server + cron scheduler; `main.go` split into `serve`/`worker` run modes; compose runs both
- [ ] Add OSS media upload pipeline (WebP conversion via OSS image processing)
- [x] i18n content infrastructure: per-locale translation tables pattern (BCP 47 keys) — `platform/i18ncontent` package (locale constants en-US/zh-CN, `ContentStatus` workflow enum, rich-content `ContentBlock` types, `NormalizeLocale`, `Transition`/`CanEdit` state machine, `PublishedFilter`); `models/i18n.go` shared types; unit test for the workflow transition matrix. Per-module migrations (ceramicstory/engage/artist) follow
- [ ] Rewrite `ws` hub with Redis pub/sub fan-out + chat session/message persistence (PRD §3.3.1)

## M1 — CMS & Content

### Restructure kept content modules (i18n + workflow)
- [x] `ceramicstory` → History & Heritage: add translations, rich multimedia blocks, workflow status (draft → review → published), `created_at`/`updated_at` — migration `000007_ceramic_story_translations`; admin CMS wired (create/update/delete/submit/approve/reject/unpublish) under `/admin/ceramicstory` with `RequirePermission(PermContentWrite)` + `PermContentPublish` split; service uses `i18ncontent.Transition`/`CanEdit`/`NormalizeLocale(validate=true)`. Rich-content blocks + media gallery deferred
- [x] `engage` (`Activity`/`Article`) → Destinations & Local Lifestyle: add translations, per-locale slugs/meta, media galleries, OSM coordinates, workflow status — migration `000008_activity_translations`; admin CMS wired (create/update/delete/submit/approve/reject/unpublish) under `/admin/engage` with `RequirePermission(PermContentWrite)` + `PermContentPublish` split. Media galleries + opening_info modeling deferred
- [x] Artist model → full artist profiles (bio/media, i18n, linked to products) — migration `000011_artist_translations` (parent gains `avatar_url` + `display_order`; translation table with name/slug/bio/meta + workflow status, UNIQUE(locale,slug), hot-path indexes); backfilled en-US from existing `artists.name`/`bio` (slug derived from name); `models/artist.go` merged view + `Create`/`Update` DTOs; dedicated `artist` module (handler/service/repository following ceramicstory pattern); public reads `GET /artists` + `GET /artists/:slug` (locale-aware, published-only, paginated); admin CMS under `/admin/artists` with `RequirePermission(PermContentWrite)` + `PermContentPublish` split; old `name`/`bio` parent columns retained (additive, future cleanup with gallery evolution)
- [ ] Category tree + tags taxonomy (replace bare `Period`/`Category` strings)
- [ ] Approval workflow: only Super Admin can approve/publish (PRD §3.1.1)
- [ ] Media library (OSS upload, WebP, FFmpeg→HLS video)
- [ ] SEO: per-locale slugs, meta tags, sitemap.xml generation, hreflang
- [ ] Admin CMS routes rebuilt around RBAC (router `/admin` group is currently a placeholder)

## M2 — E-commerce (all new)

- [x] Product/SKU model on top of `artworks`: price (CNY), stock, packed weight, JSONB attributes (size, technique, glaze, edition type, year, kiln — PRD §3.2.1), publish status, product videos — migration `000012_products_skus` (parent `products` + `product_translations` i18n table with editorial workflow; `skus` table with price_cny BIGINT minor units, stock, weight_grams, low_stock_threshold default 2, attributes JSONB, GIN index; backfilled from `artworks` + en-US translations + default SKU per product; added `product.publish` permission for super_admin-only publish gate); `models/product.go` (`Product` merged view + `SKU` + DTOs); `product` module (handler/service/repository following the ceramicstory/artist pattern); public reads `GET /catalog/products` + `GET /catalog/products/:slug` (includes SKUs); admin CMS under `/admin/products` with `RequirePermission(PermProductWrite)` + `PermProductPublish` split; SKU CRUD under `/admin/products/:id/skus` (create) + `/admin/skus/:id` (update/delete). Gallery module (`artworks`) left intact (additive; catalog is parallel). Product videos + media_assets FK deferred to media-library infra
- [x] Evolve `user_favorite_artworks` into **wishlist** — migration `000013_evolve_artworks_drop` creates `wishlists(user_id, sku_id, created_at)` (keyed on SKU, the purchasable unit — a customer favorites a variant, not a product); migrates existing favorites via the default-SKU mapping from 000012; drops the legacy `user_favorite_artworks` + `artworks` + `artwork_images` + `artwork_tags` tables; dedicated `wishlist` module (handler/service/repository) with `GET /wishlist` (locale-aware, enriched with product display info via JOIN skus→products→product_translations), `POST /wishlist` (`{"sku_id":N}`), `DELETE /wishlist/:sku_id`. Gallery module + `models/artwork.go` deleted; products is now the sole catalog read path
- [x] Cart (server-side, merge-on-login; quantity ops, bulk remove) — migration `000015_carts` (`carts.user_id UNIQUE` → one cart per signed-in user; `cart_items(cart_id, sku_id, qty)` with `UNIQUE(cart_id,sku_id)`; `ON DELETE CASCADE` mirrors wishlists); `models/cart.go`; `cart` module (handler/service/repository). Endpoints: `GET /cart` (locale-aware, `?currency=` presentment via FX); `POST /cart/items` (additive — existing qty += new, qty defaults to 1); `PATCH /cart/items/:sku_id` (absolute set); `DELETE /cart/items/:sku_id`; `DELETE /cart/items` (bulk `{"sku_ids":[...]}`); `POST /cart/merge` (guest localStorage cart → server cart on login; additive, capped at stock, unknown SKUs skipped). Stock is NOT decremented at cart stage (authoritative atomic decrement at checkout, TDD §4.3); service enforces an advisory `qty>stock` guard (`ErrConflict`). CNY totals always present; presentment totals + per-line presentment when `?currency=` supplied (graceful degradation → CNY-only on FX failure). Seed `60_cart.sql` (customer, 2 items)
- [x] FX pipeline: daily ECB rates + 2% markup + rounding rule — migration `000014_fx_rates` (`fx_rates(currency CHAR(3) PK, rate_to_cny NUMERIC(18,8), fetched_at)`); `internal/platform/fx` package (`RateSource` interface, `ECBClient` XML parser, `FixtureRateSource` for dev/tests, pure `Convert`/`RoundPrice` functions, `Repository` for fx_rates, `Service` with `Refresh` + `Convert`); worker `fx:refresh` job wired (`jobServer.FXRefresh = fxService.Refresh`); `POST /admin/fx/refresh` (settings.manage) enqueues the job; `GET /fx/rates` debug endpoint; `GET /catalog/products/:slug?currency=USD|EUR|GBP` consumer adds `price`+`price_currency` to each SKU (graceful degradation — CNY-only if rates missing). Rounding rule (PRD §3.2.3): `<100` → ceil to 0.50; `≥100` → ceil to 1.00. Markup direction: stored rate = raw ÷ (1+markup), so customer pays more presentment per CNY. Unit tests for RoundPrice matrix + Convert + Refresh markup direction (priority test target, TDD §11). `shopspring/decimal` dep for money arithmetic (never float, TDD §7). Redis cache TTL + stale-rate alert (>48h) deferred
- [x] `ShippingFeeTable`: per-country weight tiers; fee calculator; overweight block + contact message — migration `000016_shipping_fee_tiers` (`shipping_fee_tiers(country CHAR(2), max_weight_grams, fee_cny, UNIQUE(country, max_weight_grams))`; seeded US/GB/DE/CN tiers. `internal/platform/shipping/calc.go` pure `CalcFee(tiers, weight)` (no tier → `ErrUnshippable`; weight > heaviest tier → `ErrOverweight`; else cheapest sufficient tier) + unit tests (boundary/overweight/unshippable). `shipping` module: `GET /shipping/quote?country=&weight=` public preview (TDD §5.2) + `/admin/shipping/tiers` CRUD (settings.manage).
- [x] Checkout (signed-in only) + order lifecycle (created → paid → shipped → completed / cancelled / refunded, full refunds only) — migration `000017_orders` (`orders` with presentment + CNY totals, `fx_rate_used` NUMERIC snapshot, immutable `address` JSONB snapshot, status state machine with CHECK; `order_items` with `unit_price_minor` + `unit_price_cny` + `title_snapshot`/`attributes_snapshot` JSONB; `user_id` FK NO ACTION — orders survive GDPR erasure). `models/order.go`; `order` module (handler/service/repository). Authoritative stock decrement inside the order-creation tx (TDD §4.3: `UPDATE skus SET stock=stock-$1 WHERE id=$2 AND stock>=$1`; zero rows → rollback → `ErrConflict`, no partial order). Order snapshot reconciles exactly (line=unit×qty, subtotal=Σline, total=subtotal+shipping) — matches cart convention + checkout snapshot model (TDD §7). Routes: `POST /checkout`, `GET /orders[/:id]`, `POST /orders/:id/cancel` (created→cancelled, stock restored); admin `GET /admin/orders[/:id]` (order.read), `POST /admin/orders/:id/ship` (order.write, paid→shipped with carrier+tracking), `/complete` (shipped→completed), `/refund` (order.refund, paid|shipped→refunded, full refunds only). Mock payment seam (`PAYMENTS_MODE=mock` dev default): checkout enqueues `payment:finalize{success}` → worker `MarkPaid` drives created→paid; live mode (Airwallex/PayPal adapters) is #6. `fx.Service.Rate` added for the `fx_rate_used` snapshot. `jobs.PaymentFinalizePayload` extended with `OrderID`+`Success`. User service `PreferredCurrency` added (checkout default currency). Config `PAYMENTS_MODE`
- [x] Airwallex integration (Payment Intents + webhooks) — **sandbox for MVP** — `pkg/adapters/payments` Gateway interface (TDD §10: CreateIntent/Refund/VerifyWebhook) + `AirwallexGateway` (HTTP client: `/pa/payment_intents/create`, `/pa/refunds/create`, HMAC-SHA256 webhook verify over the raw body, `/authentication/login` bearer token). `payment` module (repo/service/handler + `Registry`). `payments` table (migration 000018: gateway, gateway_ref, status pending|succeeded|failed|refunded, amount_minor, currency, raw_webhook JSONB, idempotency_key UNIQUE). Checkout in sandbox/live creates an intent + returns `hosted_url` (hosted checkout); the webhook verifies the signature → idempotent upsert (ON CONFLICT idempotency_key, xmax=0 probe) → enqueue `payment:finalize` → worker `MarkPaid` (idempotent — replayed webhook is a 200 no-op). Unit-tested with httptest stubs (CreateIntent/Refund/verify valid+tampered); live sandbox round-trip pending merchant onboarding (post-MVP, PRD §2.1)
- [x] PayPal integration (checkout + webhooks) — **sandbox for MVP** — `PayPalGateway` (Orders v2 `CreateIntent` → approve link; `/v2/payments/captures/{id}/refund`; `/v1/notifications/verify-webhook-signature` API; client-credentials bearer cached). Same `payments.Gateway` interface + `payments` table + webhook path. Unit-tested with httptest stubs; live sandbox pending onboarding. **Note:** the Airwallex/PayPal HTTP clients are written from the public sandbox API specs and unit-tested with httptest + fixture signatures but NOT live-sandbox-tested (no merchant creds until post-MVP onboarding); `mock` mode (`PAYMENTS_MODE=mock`, dev default) is the fully-tested path
- [ ] Certificates: auto-generate at product creation, QR public page, provenance records, PDF
- [ ] Bulk product import (CSV/Excel) + low-stock alerts (threshold default 2, dashboard + Brevo email)
- [x] Order emails via Brevo (order-confirmation enqueued at checkout; transition emails for paid/shipped/refunded are next); tracking-number entry (manual, no carrier APIs) — `POST /admin/orders/:id/ship` records carrier_name + tracking_number (PRD §3.2.3: no carrier API integration). Order-confirmation email enqueued at checkout via the `email:send` job; paid/shipped/refunded transition emails deferred to the payments+email-template pass (#6)

## M3 — Custom Travel & Support (all new)

- [ ] Itinerary 4-step wizard API (fields per PRD §3.3.2), save-draft, sign-in required
- [ ] 24h-SLA acknowledgment email (Brevo); SLA flagging in planner inbox
- [ ] Planner CRM: inbox, statuses (pending → processing → quoted → deposit paid → confirmed / cancelled / closed), assignment, notes, CSV export
- [ ] Quote builder with **mocked option rates** (CMS rate table, CNY); 30% deposit payment via payment stack
- [ ] Itinerary PDF generation (shared pipeline with certificates)
- [ ] Chatbot: WebSocket chat + Qwen3.5 + live handoff to agent console (minimal viable)
- [ ] WhatsApp Business messaging (Meta Cloud API) — *may trail post-MVP*

## M4 — Compliance, Dashboard & Hardening

- [ ] Cookie consent banner backend (consent records); analytics gated on consent
- [ ] In-house analytics: event endpoint + GeoLite2 geolocation + PostgreSQL
- [ ] Dashboard APIs: traffic, sales, itinerary funnel; CSV export
- [ ] Audit log for sensitive admin actions
- [ ] Rate limiting, webhook signature verification, security pass
- [ ] k6 load tests (thresholds per PRD §2.4.3); backup/restore drill

## Testing & CI (PRD §2.4 — build up alongside milestones)

- [ ] GitHub Actions pipeline (lint → unit → integration → build → deploy → smoke)
- [ ] testify + mockery for unit tests; testcontainers-go for integration
- [ ] Playwright E2E + smoke suite; k6 scenarios

## Notes

- Existing repo had uncommitted local modifications; deletions were done on the filesystem — **review `git status` and commit the deletions**.
- `notification` module kept; PRD-aligned notification types (order status, low-stock, itinerary status, chat handoff, content approval) to be added as modules land.
- User schema review (PRD §8, item 7) partially done during this cleanup; the remaining alignment happens in M0 tasks above.
