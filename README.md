# Jingdezhen Ceramics Platform

An internationalized culture, e-commerce, and custom-travel platform for Jingdezhen ceramic art. Built as a modular monolith — Go + Fiber backend, React + TanStack Start frontend, PostgreSQL, Redis, and Alibaba Cloud OSS — deployed on a single Hong Kong VPS via Docker Compose.

The platform showcases ceramic history and heritage, sells original artworks with digital certificates of authenticity, and offers custom travel itinerary planning with a CRM-backed quote workflow — all managed through a role-based admin CMS.

---

## Architecture

```
                         ┌──────────────────────────────────────────────────┐
                         │              Cloud VPS / AWS EC2 (Docker)        │
                         │                                                  │
    ┌──────────┐         │  ┌──────────┐    ┌───────────┐     ┌───────────┐ │
    │  CDN     │─────────│─▶│  Caddy   │───▶│ TanStack  │───▶│  Fiber    │ │
    │ (non-CN  │  HTTPS  │  │ (rev.    │    │ Start SSR │     │  API (Go) │ │
    │  edges)  │         │  │  proxy)  │    │ (Node)    │     │  :1323    │ │
    └──────────┘         │  └──────────┘    └───────────┘     └─────┬─────┘ │
                         │                                      │        │
    ┌──────────┐         │                                      ├───────┐│
    │ Alibaba  │◀────────│──────────────────────────────────────│───────││
    │ Cloud OSS│  S3 API │                                      │       ││
    │ (HK)     │         │                              ┌───────▼───┐   ││
    └──────────┘         │                              │ PostgreSQL│   ││
                         │                              │  (pgx v5) │   ││
                         │                              └───────────┘   ││
                         │                                      │        ││
                         │                              ┌───────▼───┐   ││
                         │                              │  Redis    │   ││
                         │                              │ (cache,   │   ││
                         │                              │  pub/sub, │   ││
                         │                              │  Asynq)   │   ││
                         │                              └───────────┘   ││
                         │                                      │        ││
                         │                              ┌───────▼───┐   ││
                         │                              │  Worker   │   ││
                         │                              │ (Asynq +  │   ││
                         │                              │  cron)    │   ││
                         │                              └───────────┘   ││
                         │                                      │        ││
                         │  ┌──────────────┐  ┌──────────────┐  │        ││
                         │  │ Prometheus   │──│  Grafana     │  │        ││
                         │  │ (scrape      │  │  (dashboards │  │        ││
                         │  │  /metrics)   │  │   + alerting)│  │        ││
                         │  └──────────────┘  └──────────────┘  │        ││
                         └──────────────────────────────────────────────┘┘

    External Services (behind adapter interfaces; sandbox/mock in dev):
    ┌────────────┐  ┌──────────┐  ┌────────┐  ┌─────────┐  ┌───────────┐
    │ Airwallex  │  │  PayPal  │  │ Brevo  │  │  Qwen   │  │ chromedp  │
    │ (cards)    │  │ (alt)    │  │ (email)│  │  (LLM)  │  │ (PDF)     │
    └────────────┘  └──────────┘  └────────┘  └─────────┘  └───────────┘
```

**Request flow:** Browser → CDN → Caddy (TLS, reverse proxy) → TanStack Start SSR (public pages, server-side fetched from Fiber) or Fiber API directly. The Fiber API reads from PostgreSQL (pgx pool) and Redis (sessions, cache, pub/sub). Background jobs (Asynq) run in a separate worker process on the same binary. Prometheus scrapes the API's `/metrics` endpoint every 15 seconds; Grafana visualizes the data in dashboards. Media uploads go browser → presigned OSS upload (never through the VPS). PDF generation uses a chromedp headless-shell sidecar.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.26, Fiber v2, pgx v5, go-redis v9, golang-migrate, Asynq |
| **Frontend** | React 19, TanStack Start 1.168, TanStack Router, TanStack Query, Vite 8, Tailwind CSS 3.4, zod 3 |
| **Database** | PostgreSQL 16 (PostGIS), 28 migrations |
| **Cache/Queue** | Redis 7 (sessions, cache, pub/sub, Asynq job queue) |
| **Object Storage** | Alibaba Cloud OSS (HK region), S3-compatible, on-the-fly WebP |
| **CDN** | Alibaba Cloud CDN (non-mainland edges, ICP-free) |
| **Payments** | Airwallex (card intents) + PayPal, both sandbox until merchant onboarding |
| **Email** | Brevo (transactional), template-driven |
| **AI Chat** | Qwen (DashScope) — deferred post-MVP |
| **PDF** | chromedp headless-shell sidecar (HTML → PDF) |
| **Testing** | testify, testcontainers-go, k6, Vitest, Playwright |
| **Monitoring** | Prometheus (metrics scraping), Grafana (dashboards + alerting) |
| **CI/CD** | GitHub Actions (lint → unit → integration → build → security) |

---

## Repository Layout

```
├── AGENTS.md              Contributing guide (read before touching code)
├── README.md              This file
├── .github/workflows/     CI: lint, unit, integration, build, security, frontend
├── backend/               Go + Fiber API (module: jingdezhen-ceramics-backend)
│   ├── cmd/api/           Single binary, two modes: serve, worker
│   ├── internal/
│   │   ├── api/           Router + middleware (auth, RBAC, error mapper, metrics)
│   │   ├── config/        Env-based config (Viper)
│   │   ├── migrations/    golang-migrate SQL (28 up/down pairs)
│   │   ├── models/        Domain structs + DTOs + RBAC + error sentinels
│   │   ├── modules/       21 feature modules (handler → service → repository)
│   │   ├── platform/     Cross-cutting: fx, i18n, jobs, pdf, redis, shipping
│   │   ├── ws/           WebSocket hub (Redis pub/sub fan-out)
│   │   └── docs/         Generated Swagger 2.0 spec
│   ├── pkg/
│   │   ├── adapters/     payments, storage, pdf, geoip, ratelimit, certchain
│   │   ├── email/        Brevo sender + templates
│   │   └── utils/        Crypto, token, context helpers
│   ├── k6/               Load test scripts (smoke, browse, checkout, spike, soak)
│   ├── scripts/          Seed SQL + backup/restore (pg_dump → OSS)
│   ├── monitoring/       Prometheus config + Grafana provisioning (datasource + dashboards)
│   ├── docker-compose.dev.yml    Dev stack (api, worker, db, redis, chromedp)
│   ├── docker-compose.prod.yml   Production stack (api, worker, db, redis, frontend, caddy, prometheus, grafana)
│   └── Makefile          build, test, migrate, k6, backup, prod-* targets
├── frontend/             React + TanStack Start (storefront + admin CMS)
│   ├── src/
│   │   ├── routes/       File-based routes (locale-segmented, admin CMS)
│   │   ├── components/   UI components (layout, admin, ornaments, common)
│   │   ├── lib/          Auth, API client, i18n, cart, wishlist, realtime
│   │   ├── i18n/         UI string catalogs (en-US, zh-CN)
│   │   ├── mocks/        Offline mock backend (MockTransport)
│   │   └── styles/       Design tokens (CSS variables)
│   ├── e2e/              Playwright E2E (6 specs)
│   ├── Caddyfile         Production reverse proxy config (TLS, routing)
│   ├── Dockerfile        Production SSR build (node:22-alpine, pnpm)
│   └── package.json      pnpm 11.19, scripts: dev, build, test, e2e
└── docs/
    ├── PRD.md            Product requirements (v0.17)
    ├── TDD.md            Technical design (v0.1)
    ├── REFACTOR-TODO.md  Milestone task tracker (M0–M4)
    └── deploy-aws-ec2.md Step-by-step AWS EC2 deployment guide
```

### Backend modules (`internal/modules/`)

`address` · `analytics` · `artist` · `audit` · `cart` · `ceramicstory` · `certificate` · `consent` · `engage` · `itinerary` · `media` · `notification` · `order` · `payment` · `privacy` · `product` · `shipping` · `sitemap` · `twofa` · `user` · `wishlist`

---

## Getting Started

### Prerequisites

- **Go** 1.26+
- **Node.js** 22+ and **pnpm** 11.19+
- **Docker** + Docker Compose
- **k6** (for load tests; optional)
- **migrate** CLI (for DB migrations; `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

### Backend

```bash
cd backend

# 1. Copy and fill the env file
cp .env.example .env          # fill in DB_USER, DB_PASSWORD, JWT_SECRET, etc.

# 2. Start dev services (PostgreSQL + Redis + API + Worker)
make up

# 3. Apply migrations
make migrate-up

# 4. Seed dev data (optional — resets commerce tables, preserves users)
make db-seed

# 5. Verify
curl http://localhost:1323/          # → {"message":"Welcome to..."}
curl http://localhost:1323/catalog/products?locale=en-US
```

### Frontend

```bash
cd frontend

# 1. Install dependencies
pnpm install

# 2. Copy env
cp .env.example .env          # VITE_API_MODE=mock (offline) or live

# 3. Dev server (http://localhost:3000)
pnpm dev
```

### Common commands (from `backend/`)

| Command | Description |
|---------|-------------|
| `make up` | Start all dev services (api, worker, db, redis, chromedp) |
| `make down` | Stop containers (keeps data volumes) |
| `make down-v` | Stop + wipe data volumes |
| `make logs` | Tail all service logs |
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Roll back last migration |
| `make db-seed` | Reset + seed dev data |
| `make test` | All tests (unit + integration, -race) |
| `make test-unit` | Unit tests only (-short, no Docker) |
| `make test-integration` | Integration tests (testcontainers: real PG + Redis) |
| `make k6-smoke` | k6 smoke test (post-deploy gate) |
| `make k6-browse` | k6 browse baseline (50 RPS, 200 VUs, 2 min) |
| `make k6-spike` | k6 spike test (500 RPS burst) |
| `make k6-soak` | k6 soak test (30 RPS, 2h) |
| `make backup` | pg_dump → Alibaba Cloud OSS (nightly) |
| `make restore` | pg_restore from OSS |
| `make swag` | Regenerate Swagger spec from handler annotations |
| `make prod-build` | Build all production Docker images |
| `make prod-up` | Start full production stack (api, worker, db, redis, frontend, caddy, prometheus, grafana) |
| `make prod-down` | Stop production stack (keeps data volumes) |
| `make prod-logs` | Tail all production service logs |
| `make prod-restart` | Restart a production service (`NAME=api`) |

---

## Key Features

### Storefront (public)
- **Bilingual catalog** (EN/zh-CN) with per-locale translation tables, product detail pages with SKU variants, tags, and artist cross-links
- **Search & browse** — PostgreSQL full-text search, category/tag filtering, locale-aware slugs
- **Wishlist + cart** — localStorage for guests, merged on login; atomic stock decrement at checkout
- **Checkout** — signed-in customers only; reactive shipping-quote UX; Airwallex/PayPal redirect flow; order tracking
- **Digital certificates** — auto-generated per product with QR code + provenance chain; public verification page
- **Custom travel** — 5-step itinerary wizard (dates, interests, budget, consent, review); planner CRM with quote builder + 30% deposit
- **SEO** — per-locale canonical URLs, hreflang alternates, Open Graph/Twitter cards, JSON-LD structured data, generated sitemap.xml
- **Real-time notifications** — WebSocket push via Redis pub/sub (cross-instance fan-out)

### Admin CMS (RBAC-gated, 2FA-enforced)
- **Content management** — ceramic stories, destinations, artist profiles, products/SKUs with approval workflow (`draft → in_review → published | rejected`); only Super Administrator may approve/publish
- **Media library** — presigned OSS uploads, ordered entity galleries, WebP image processing
- **Orders** — list, ship, complete, full refund (no partial refunds); tracking-number entry
- **Itinerary CRM** — inbox, assignment, notes, quote builder, confirm, refund, CSV export
- **Dashboard** — traffic, sales, funnel analytics with CSV export
- **Settings** — shipping tiers, itinerary option rates, FX rate refresh
- **User management** — role assignment, audit log

### Security & Compliance
- **Auth** — JWT (HS256, 30-day), TOTP 2FA, Google OAuth, JWT blocklist (Redis)
- **RBAC** — 5 staff roles, 16 permission keys, per-route middleware enforcement
- **Rate limiting** — per-IP (100/min global, 5/min auth), per-userID 2FA lockout, per-email throttle (3/hour)
- **GDPR** — consent recording, data export, self-service erasure (anonymizes in-place, preserves order history)
- **Webhook security** — HMAC-SHA256 signature verification (Airwallex), API-based verification (PayPal), idempotent processing

---

## API Overview

The Fiber API exposes ~200+ endpoints across these route groups:

| Group | Auth | Routes |
|-------|------|--------|
| Public catalog | — | `/catalog/*`, `/artists/*`, `/ceramicstory/*`, `/engage/*`, `/certificates/*` |
| Auth | rate-limited | `/auth/signup`, `/auth/login`, `/auth/2fa/*`, `/auth/google/*`, `/auth/reset-password` |
| Profile | JWT | `/profile/*` (addresses, consent, 2FA, GDPR export) |
| Commerce | JWT | `/cart`, `/checkout`, `/orders/*`, `/wishlist`, `/itineraries/*` |
| Notifications | JWT + WS | `/ws` (WebSocket), `/notifications/*` |
| Admin | JWT + RBAC | `/admin/*` (CMS, orders, media, CRM, dashboard, settings, users, audit) |
| Webhooks | signature | `/webhooks/airwallex`, `/webhooks/paypal` |
| Analytics | — | `/analytics/events` (consent-gated) |
| Metrics | — | `/metrics` (Prometheus exposition, internal-only) |
| SEO | — | `/sitemap.xml`, `/robots.txt` |

Swagger UI available at `/admin/swagger/*` (JWT-gated).

---

## Money & FX

All monetary amounts are stored as **BIGINT minor units** (fen/cents/pence) with a 3-character currency code — never float, never NUMERIC for arithmetic. The FX pipeline converts CNY (base + settlement) to USD/EUR/GBP using daily ECB reference rates with a 2% default markup, snapshotting the rate at checkout to prevent total drift between sessions.

---

## Testing

### Backend

| Type | Tool | Scope |
|------|------|-------|
| Unit | testify | Money math, FX rounding, RBAC matrix, error mapper, email throttle |
| Integration | testcontainers-go | Real PostgreSQL + Redis: order state machine, webhook idempotency, RBAC, i18n slug resolution, certificate uniqueness, GDPR erasure |
| Load | k6 | Smoke, browse (50 RPS), checkout funnel, WebSocket (500 sessions), spike (10×), soak (2h) |

**Load test results (local dev, 16-core/32 GB):**

| Scenario | Throughput | p95 latency | Error rate |
|----------|-----------|-------------|------------|
| Browse baseline (50 RPS, 2 min) | 50 RPS | 1.7 ms | 0% |
| Spike (500 RPS burst, 2.5 min) | 500 RPS | 1.8 ms | 0% |
| Ramp to ceiling | ~8000 RPS | 1.45 ms | 0% |

Production thresholds (PRD §2.4.3): p95 < 300 ms, error rate < 0.1%.

### Frontend

| Type | Tool | Count |
|------|------|-------|
| Unit | Vitest + Testing Library | 51 tests |
| E2E | Playwright | 6 specs (browse→checkout, signup+activate, locale switch, itinerary, certificate, cart) |

---

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on every PR + push to main:

| Job | Tool | Gate |
|-----|------|------|
| **Lint** | golangci-lint v2 + swag-check | Hard (spec drift fails) |
| **Unit** | `go test -race -short` | Hard |
| **Integration** | `go test -race` (testcontainers) | Hard (20-min timeout) |
| **Build** | Docker Buildx | Hard (needs unit + integration) |
| **Security** | govulncheck (reachable vulns) + osv-scanner (advisory) | govulncheck hard, osv-scanner advisory |
| **Frontend** | pnpm: lint, format:check, typecheck, test, build | Hard |

Dependabot enabled for dependency updates.

---

## Deployment

**Target:** Single VPS (AWS EC2, Alibaba Cloud, or Tencent Cloud), Docker Compose.

| Component | Config |
|-----------|--------|
| VPS / EC2 | `t3.large` recommended (2 vCPU, 8 GB RAM, ~$63/mo); `t3.medium` minimum |
| Object storage | Alibaba Cloud OSS (HK region), S3-compatible |
| CDN | Alibaba Cloud CDN (non-mainland edges, ICP-free) |
| Reverse proxy | Caddy (auto-TLS via ACME/Let's Encrypt) |
| Backups | Nightly pg_dump → OSS, 14-day retention, RPO 24h, RTO 4h |

**Production stack** (`docker-compose.prod.yml`): 8 services — api, worker, PostgreSQL, Redis, frontend SSR, Caddy, Prometheus, Grafana. See `docs/deploy-aws-ec2.md` for the complete step-by-step AWS EC2 deployment guide.

**MVP go-live:** May 2027. Payments in sandbox until merchant onboarding completes post-MVP.

---

## Monitoring

The Go API exposes a `/metrics` endpoint (Prometheus exposition format) with custom HTTP metrics:

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | counter | status, method, route |
| `http_request_duration_seconds` | histogram | route |
| `http_requests_in_flight` | gauge | — |
| `fiber_errors_total` | counter | type |

**Prometheus** scrapes `api:1323/metrics` every 15 seconds (config in `backend/monitoring/prometheus.yml`).

**Grafana** auto-provisions the Prometheus datasource and an "API Overview" dashboard (7 panels: request rate, p95 latency, error rate, in-flight requests, latency by route, rate by status, rate by method). Dashboards are in `backend/monitoring/grafana/`.

Both Prometheus (port 9090) and Grafana (port 3001) are bound to `127.0.0.1` — they are not exposed to the public internet. Access them via SSH tunnel:

```bash
ssh -L 3001:localhost:3001 -L 9090:localhost:9090 user@your-domain.com
```

Then open `http://localhost:3001` (Grafana) or `http://localhost:9090` (Prometheus) in your browser.

---

## Project Stats

- **276 commits** across the refactor
- **186 Go files** in the backend
- **105 TypeScript/TSX files** in the frontend
- **28 database migrations** (baseline + incremental)
- **21 feature modules** in the backend
- **~200+ API endpoints** (Fiber router)
- **2 locales** (en-US, zh-CN) with ~700 UI string keys each

---

## Documentation

| Document | Content |
|----------|---------|
| `docs/PRD.md` | Product requirements (v0.17) — what and why |
| `docs/TDD.md` | Technical design (v0.1) — architecture decisions by section |
| `docs/REFACTOR-TODO.md` | Milestone task tracker (M0–M4, all complete) |
| `docs/deploy-aws-ec2.md` | Step-by-step AWS EC2 deployment guide (instance setup, Docker, TLS, monitoring, backups) |
| `docs/Bullets.md` | Resume bullet points (project highlights) |
| `AGENTS.md` | Contributing guide (read before touching code) |
| `frontend/AGENTS.md` | Frontend-specific guide (stack, conventions, do-not-do rules) |
| `backend/k6/README.md` | Load test usage + thresholds |
| `backend/scripts/backup/README.md` | Backup/restore drill procedure |

---

## License

Private project. All rights reserved.
