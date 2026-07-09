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
- [ ] Validate baseline against real PostgreSQL (no Docker available in this session) — first `make migrate-up` or testcontainers run
- [ ] Switch migration workflow into CI (run against testcontainers, PRD §2.4.1)

### User / Auth (PRD §3.4.1, §3.5)
- [x] Replace role model `admin/normal_user/guest` with PRD RBAC: customer + `super_admin`, `content_editor`, `travel_planner`, `ecommerce_operator`, `customer_service` (roles/permissions tables) — migration `000002_rbac`, models/rbac.go, middleware `RequirePermission`/`RequireRole`/`HasRole`, repo `GetUserRoles`/`AssignRole`, service `AdminAssignRole`, admin user routes wired under `/admin/users`
- [ ] Add shipping address book (multiple addresses per user; country drives shipping calculator)
  - [x] Shipping address book implemented: migration `000003_user_addresses`, `models/address.go`, `address` module (handler/service/repository), routes under `/profile/addresses`; one-default-per-user enforced via partial unique index + tx
- [x] Add preferred locale (`en-US`/`zh-CN`) and preferred currency (USD/EUR/GBP) to profile — migration `000004_user_preferences` adds columns + CHECK constraint; `User`/`UserUpdateData` updated; all 4 scan paths fixed (latent double-ProfileData scan bug removed); validator `oneof=USD EUR GBP` for clean 400s
- [x] Add consent records (Privacy Policy / ToS acceptance with timestamp & version — GDPR) — migration `000005_consent_records` (append-only, nullable user_id for anonymous, 4 kinds, IP hashed via HMAC); `models/consent.go`; `consent` module (repo/service/handler); `POST /consent` public, `GET /profile/consent` + `GET /profile/consent/:kind` protected; `CONSENT_HMAC_KEY` in config
- [x] Add TOTP 2FA (mandatory for Super Admin, optional for staff) — migration `000006_user_2fa` (encrypted secret at rest via AES-GCM, two-phase enroll→confirm); `pkg/utils/crypto.go` AES-GCM; `twofa` module (repo/service/handler); login gates on enabled 2FA via `Err2FARequired` + 5-min pending JWT, `/auth/2fa/verify` completes login; `POST /profile/2fa/{enroll,confirm}`, `DELETE /profile/2fa`; `TWO_FA_ENCRYPTION_KEY` in config. **Super-admin must-enroll enforced**: a super_admin with no 2FA is blocked at login (`Err2FAEnrollmentRequired`), walked through `/auth/2fa/pending-enroll` → `/auth/2fa/pending-confirm` (15-min pending token; confirm enables 2FA AND mints the real JWT). **Google OAuth login is 2FA-gated too** (shared `challenge2FAOrMint` helper). **Backup codes**: migration `000009_user_2fa_backup_codes` (one-time, stored SHA-256 hashed with app-key pepper); generated at confirm (shown ONCE), accepted at `/auth/2fa/verify` (TOTP tried first, then a backup code is consumed atomically); `POST /profile/2fa/backup-codes/regenerate` + `GET /profile/2fa/backup-codes` (remaining count); `Disable` clears backup codes; unit tests for hashing/normalization
- [ ] Add WhatsApp OAuth provider (schema already generic via `auth_provider`) — *may trail post-MVP*
- [x] Replace AWS SES sender with **Brevo** (keep `email.ServiceInterface`; remove AWS config/deps) — `pkg/email/brevo_sender.go` REST adapter; `ses_sender.go` removed; AWS SDK dropped from go.mod via `go mod tidy`
- [x] Extend `config.go`: Brevo, Airwallex, PayPal, Qwen, OSS, Redis; remove AWS fields — `.env.example` documents all keys; Redis added to `docker-compose.dev.yml`
- [x] GDPR self-service: user data export + account deletion endpoints — migration `000010_user_gdpr` (adds `users.deleted_at`; anonymize-in-place so order-record retention keeps referential integrity); `models/privacy.go` (`UserDataExport` bundle + `DeleteAccountRequest`); `privacy` module (repository spans users/addresses/consent/2fa/favorites/notifications tables; service captures email pre-erasure then anonymizes + enqueues Brevo confirmation via `email:send` queue); `GET /profile/export` (synchronous JSON, `Content-Disposition` download; async job + download link deferred to M2/M3 when order history makes payload large); `POST /privacy/delete-account` (`{"confirm":"DELETE"}` body guard; irreversibly nulls email/nickname/password_hash/avatar/tokens/auth_provider_id, sets `is_active=false`+`deleted_at`; CASCADE-purges addresses/2FA/favorites/notifications; SET NULL on consent_records + content authorship for audit). Login already rejects erased stubs via `is_active=false`. JWT blocklist (deleted-user token invalidation) deferred to M4 security pass

### Infrastructure
- [x] Add Redis (sessions, cache, pub/sub, Asynq job queue) — `redis:7-alpine` in `docker-compose.dev.yml` (port 6379, healthcheck); `internal/platform/redis` go-redis client; `internal/platform/jobs` Asynq enqueue client + worker server + cron scheduler; `main.go` split into `serve`/`worker` run modes; compose runs both
- [ ] Add OSS media upload pipeline (WebP conversion via OSS image processing)
- [x] i18n content infrastructure: per-locale translation tables pattern (BCP 47 keys) — `platform/i18ncontent` package (locale constants en-US/zh-CN, `ContentStatus` workflow enum, rich-content `ContentBlock` types, `NormalizeLocale`, `Transition`/`CanEdit` state machine, `PublishedFilter`); `models/i18n.go` shared types; unit test for the workflow transition matrix. Per-module migrations (ceramicstory/engage/artist) follow
- [ ] Rewrite `ws` hub with Redis pub/sub fan-out + chat session/message persistence (PRD §3.3.1)

## M1 — CMS & Content

### Restructure kept content modules (i18n + workflow)
- [x] `ceramicstory` → History & Heritage: add translations, rich multimedia blocks, workflow status (draft → review → published), `created_at`/`updated_at` — migration `000007_ceramic_story_translations`; admin CMS wired (create/update/delete/submit/approve/reject/unpublish) under `/admin/ceramicstory` with `RequirePermission(PermContentWrite)` + `PermContentPublish` split; service uses `i18ncontent.Transition`/`CanEdit`/`NormalizeLocale(validate=true)`. Rich-content blocks + media gallery deferred
- [x] `engage` (`Activity`/`Article`) → Destinations & Local Lifestyle: add translations, per-locale slugs/meta, media galleries, OSM coordinates, workflow status — migration `000008_activity_translations`; admin CMS wired (create/update/delete/submit/approve/reject/unpublish) under `/admin/engage` with `RequirePermission(PermContentWrite)` + `PermContentPublish` split. Media galleries + opening_info modeling deferred
- [ ] Artist model → full artist profiles (bio/media, i18n, linked to products)
- [ ] Category tree + tags taxonomy (replace bare `Period`/`Category` strings)
- [ ] Approval workflow: only Super Admin can approve/publish (PRD §3.1.1)
- [ ] Media library (OSS upload, WebP, FFmpeg→HLS video)
- [ ] SEO: per-locale slugs, meta tags, sitemap.xml generation, hreflang
- [ ] Admin CMS routes rebuilt around RBAC (router `/admin` group is currently a placeholder)

## M2 — E-commerce (all new)

- [ ] Product/SKU model on top of `artworks`: price (CNY), stock, packed weight, JSONB attributes (size, technique, glaze, edition type, year, kiln — PRD §3.2.1), publish status, product videos
- [ ] Evolve `user_favorite_artworks` (migration 000034) into **wishlist**
- [ ] Cart (server-side, merge-on-login; quantity ops, bulk remove)
- [ ] FX pipeline: daily ECB rates + 2% markup + rounding rule; price cache
- [ ] `ShippingFeeTable`: per-country weight tiers; fee calculator; overweight block + contact message
- [ ] Checkout (signed-in only) + order lifecycle (created → paid → shipped → completed / cancelled / refunded, full refunds only)
- [ ] Airwallex integration (Payment Intents + webhooks) — **sandbox for MVP**
- [ ] PayPal integration (checkout + webhooks) — **sandbox for MVP**
- [ ] Certificates: auto-generate at product creation, QR public page, provenance records, PDF
- [ ] Bulk product import (CSV/Excel) + low-stock alerts (threshold default 2, dashboard + Brevo email)
- [ ] Order emails via Brevo; tracking-number entry (manual, no carrier APIs)

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
