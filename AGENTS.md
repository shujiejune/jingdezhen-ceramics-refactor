# AGENTS.md

Guidance for any coding agent (or human contributor) working on this repository. Read this before touching code.

## Project

**Jingdezhen Ceramics Platform** — an internationalized culture / e-commerce / custom-travel platform for Jingdezhen ceramic art. Backend in Go + Fiber; frontend planned in SolidStart (TS) — but the frontend does not exist in this repo yet.

- **Current phase:** refactor of an inherited backend (formerly a "Learning & Communication Platform") toward the PRD. MVP target go-live **Mon, Aug 31, 2026**.
- **Track work via:** `docs/PRD.md` (requirements, v0.17), `docs/TDD.md` (technical design, v0.1), `docs/REFACTOR-TODO.md` (checkbox task list). The TDD's section numbers (e.g. §3.4) are the source of truth for design decisions; the PRD's sections (e.g. §3.4.1) are the source of truth for *what* and *why*.

## Repository layout

```
backend/      Go + Fiber API (module: jingdezhen-ceramics-backend)
  cmd/api/        main.go (two run modes: serve, worker)
  internal/
    api/          router + middleware (auth, RBAC)
    config/       env-based config
    migrations/   golang-migrate SQL files (baseline = 000001_baseline)
    models/       domain structs + request/response DTOs
    modules/      feature modules (handler → service → repository)
    platform/     cross-cutting (fx, pdf, i18n, jobs)   [to be created]
    ws/           WebSocket hub (chat)
  pkg/
    adapters/     external-service interfaces + impls   [to be created]
    email/        (SES — to be replaced by Brevo adapter)
    utils/
docs/         PRD.md, TDD.md, REFACTOR-TODO.md, this file
frontend/     REMOVED. To be created fresh with SolidStart per TDD §6.
```

## Tech stack & key decisions (do not contradict without discussion)

- **Backend:** Go 1.24 + Fiber v2; PostgreSQL (pgx v5 pool); Redis (sessions, cache, pub/sub, Asynq queues); golang-migrate; WebSocket via `gofiber/contrib/websocket`.
- **Money:** `BIGINT` minor units (fen/cents/pence) + 3-char currency code — never float, never NUMERIC-for-arithmetic. See TDD §7.
- **i18n:** per-locale translation tables per entity (status lives on the translation), BCP 47 locale keys. UI strings are frontend catalogs, not DB. See TDD §3.2.
- **External services:** every external dependency (payments, email, LLM, storage, WhatsApp, blockchain cert) goes behind an interface in `pkg/adapters/`. Sandbox/live/mock is an env-var flip. Never call live gateways in tests. See TDD §4.1, §10.
- **Content workflow:** `draft → in_review → published | rejected`; only the **Super Administrator** may approve/publish (PRD §3.1.1).
- **Checkout:** signed-in customers only; full refunds, no partial refunds; no logistics/carrier integration (manual tracking-number entry). PRD §3.2.3.
- **Deployment:** single Hong Kong VPS, Docker Compose; Alibaba Cloud OSS (HK) + CDN (non-mainland edges, ICP-free); CFS to be chosen. PRD §2.1.
- **Currencies:** USD / EUR / GBP presentment, **CNY** base + settlement (China bank). FX = daily ECB + 2% default markup + rounding rule. PRD §3.2.3.
- **Payments (MVP):** Airwallex (cards) + PayPal, both in **sandbox** until merchant onboarding completes post-MVP. PRD §7.

## Working rules

1. **Read first, then act.** Before changing a module, read its handler/service/repository and the relevant PRD + TDD sections. The inherited code has latent bugs and schema/code drift — verify against code, not assumptions.
2. **Preserve existing code where possible.** The kept modules (`user`, `gallery`, `ceramicstory`, `engage`, `notification`, `ws`) are the foundation; extend, don't rewrite, unless the TDD says to evolve them (e.g. `artworks`→Product/SKU, `user_favorite_artworks`→wishlist).
3. **`go build ./...` and `go vet ./...` must pass** after every change. Don't leave the tree broken.
4. **Migrations:** add new numbered files in `backend/internal/migrations/` (next after `000001_baseline`). Always provide matching `.up.sql` and `.down.sql`. Never edit the baseline. Schema changes should be additive; drop columns only in a later cleanup migration.
5. **Don't touch the frontend.** There is no `frontend/` in this repo. When the SolidStart app is created, follow TDD §6.
6. **No new external SDK calls in business logic.** Route them through `pkg/adapters/` interfaces.
7. **Follow the existing Go style:** handler→service→repository layers; repository takes a `pgx` executor (already supports tx); services return typed `models.Err*` errors (a central error-mapper middleware is planned, TDD §4.3).
8. **Tests:** unit tests with testify (adapters mocked); integration with testcontainers-go (real PG+Redis). Priority test targets are money, shipping, order/stock, webhook idempotency, RBAC. PRD §2.4.
9. **Update `docs/REFACTOR-TODO.md`** as items are completed; update TDD §12 when technical decisions are settled.
10. **When unsure, ask.** Especially before: changing auth (current single-JWT → refresh-rotation is proposed, TDD §5.1), deleting code with uncommitted work, or contradicting a confirmed/decided PRD item.

## Common commands (run from `backend/`)

```
go build ./...            # must pass
go vet ./...              # must pass
go test ./...             # unit tests
make up                   # start dev services (docker compose, dev file)
make migrate-up           # apply migrations (needs DATABASE_URL in .env)
make migrate-down         # roll back last migration
make logs                 # tail compose logs
```

Note: `make migrate-up` requires the `migrate` CLI installed and `DATABASE_URL` set in `backend/.env`. The dev compose file provisions PostgreSQL + Redis.

## Known issues / gotchas

- **Backend repo has uncommitted state** from the refactor deletions — review `git status` and commit before starting new work.
- **`cs_repository.go`** has `ORDER BY ... start_year DSC` — typo for `DESC` (pre-existing bug, not yet fixed).
- **`router.go` `/admin` group is a placeholder** — admin CMS routes will be rebuilt around RBAC (PRD §3.4.1).
- **`pkg/email` uses AWS SES** — to be replaced by a Brevo adapter (TDD §10). The `email.ServiceInterface` stays.
- **Migration baseline (`000001_baseline`) is validated by inspection only** — not yet run against live PostgreSQL (no Docker available in the session that created it). First `make migrate-up` should be watched.
- **`config.go` still has AWS fields** — to be replaced with Brevo / Airwallex / PayPal / Qwen / OSS / Redis keys during M0.

## Definition of done (for REFACTOR-TODO items)

- Code implemented per TDD section; builds and vets clean.
- New schema in a numbered migration (up + down).
- Adapters behind interfaces; sandbox/mock impls provided where the live service isn't onboarded.
- Unit tests for the change; integration test where it touches PG/Redis.
- Relevant `docs/REFACTOR-TODO.md` box checked; TDD updated if a §12 decision was settled.
