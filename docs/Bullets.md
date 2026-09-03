# Resume Bullets — Jingdezhen Ceramics Platform

Full-Stack Software Engineer role. Google XYZ formula (Accomplished X as measured by Y by doing Z).

---

## Architecture & Infrastructure

- **Architected** a full-stack international ceramics marketplace in Go and React, serving multi-currency checkout, bilingual content, and custom-travel booking across 20+ feature modules.

- **Built** 200+ REST API endpoints in Go with Fiber using a layered handler→service→repository pattern with pgx pooling, cleanly separating business logic from HTTP concerns.

- **Modeled** a 30-table PostgreSQL schema using golang-migrate, storing money as BIGINT minor units, product attributes as JSONB, and translations in per-locale tables for bilingual content.

## Internationalization & Content

- **Implemented** bilingual content (English/Chinese) using per-locale translation tables with BCP 47 keys, enabling independent editorial workflow states per locale without cross-locale fallback.

- **Enforced** a draft→review→publish approval workflow at three layers — routing RBAC, a Go state machine, and database grants — so only Super Administrators can publish content.

## Security & Authentication

- **Designed** role-based access control with 5 staff roles and 16 permission keys, backed by a Redis JWT blocklist and a unit-tested permission matrix preventing privilege escalation.

- **Implemented** JWT authentication with 2FA (TOTP), Google OAuth, per-IP rate limiting, and a Redis-backed per-email throttle stopping mailbox flooding across rotating IPs.

- **Built** GDPR consent recording and data erasure preserving order history by anonymizing user records in-place with NO ACTION foreign keys, meeting the 24-hour RPO requirement.

## E-commerce & Payments

- **Delivered** signed-in-only checkout with atomic stock decrement inside PostgreSQL transactions, handling oversell races by rejecting with 409 and restoring stock on cancellation.

- **Integrated** Airwallex and PayPal behind adapter interfaces with sandbox/mock env flipping, webhook idempotency via unique keys, and HMAC signature verification — never calling live gateways in tests.

- **Built** an FX pipeline converting CNY to USD/EUR/GBP using daily ECB rates with a 2% markup, snapshotting the rate at checkout to prevent total drift between sessions.

## Real-time & Notifications

- **Rewrote** the WebSocket hub to use Redis pub/sub for cross-instance fan-out, enabling worker-side notifications to reach users connected on any API instance as measured by 500 concurrent sessions.

## Frontend

- **Rebuilt** the frontend on React 19 + TanStack Start with server-side rendering via Vite 8, Tailwind CSS, and zod validation, featuring a blue-and-white porcelain-inspired visual language.

- **Implemented** per-locale SEO with canonical URLs, hreflang alternates, Open Graph cards, JSON-LD, a generated sitemap.xml, and noindex on private routes — audited in both locales.

## Error Handling

- **Built** a central error-mapper converting typed Go errors into a stable JSON envelope with machine-readable codes, replacing per-handler error chains and aligning with the frontend's classification layer.

## Testing & CI/CD

- **Wrote** unit tests with testify and integration tests with testcontainers-go (real PostgreSQL + Redis), covering money math, stock decrement, webhook idempotency, RBAC, and FX rounding.

- **Set up** GitHub Actions with 5 jobs — lint, unit, integration, Docker build, and govulncheck security scan — achieving zero reachable vulnerabilities as measured by the hard gate.

## Performance & Disaster Recovery

- **Created** k6 load test scenarios with codified thresholds (p95 < 300ms, error rate < 0.1%), targeting 50 RPS baseline, 500 concurrent WebSocket sessions, and 10× spike bursts.

- **Wrote** nightly pg_dump scripts uploading compressed database backups to Alibaba Cloud OSS with 14-day retention, plus a parallel restore procedure targeting a 4-hour recovery objective.

## Custom Travel & Analytics

- **Built** a custom-travel itinerary wizard with a planner CRM, SLA monitoring, and a cron-based breach notifier pushing dashboard notifications and emails to travel planners.

- **Built** an in-house analytics system with GeoIP, consent-gated event tracking, nightly rollup aggregation, and a dashboard with traffic, sales, and funnel views plus CSV export.

- **Developed** a media library with local and Alibaba Cloud OSS storage adapters, supporting direct browser uploads, ordered entity galleries, and WebP image processing.
