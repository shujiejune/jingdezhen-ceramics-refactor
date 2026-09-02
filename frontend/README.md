# Jingdezhen Ceramics — Frontend

React 19 + TanStack Start (SSR) storefront + admin CMS for the Jingdezhen
Ceramics Platform, backed by the Go + Fiber API in the sibling `../backend`
directory. Read `AGENTS.md` first — it is the authoritative guide.

## Quickstart

```bash
pnpm install     # Node ≥ 22, pnpm 11.19.0 (pinned via packageManager)
pnpm dev         # SSR dev server on http://localhost:3000
```

The prototype runs fully offline against typed in-process mocks
(`src/mocks/`) that mirror the backend contract — no API needed. To run
against the real backend: `cd ../backend && make up` (API on `:1323`), then
set `VITE_API_MODE=live` (see `.env.example`).

**Demo accounts** (mock mode): `emily@demo.dev` (customer) and
`admin@demo.dev` (2FA code `123456`) — password `porcelain123`.

## Scripts

| Command          | What it does                                |
| ---------------- | ------------------------------------------- |
| `pnpm dev`       | Dev server (SSR, port 3000)                 |
| `pnpm build`     | Production build (nitro, `.output/`)        |
| `pnpm start`     | Serve the built output                      |
| `pnpm lint`      | ESLint (flat config)                        |
| `pnpm format`    | Prettier write (`format:check` for CI)      |
| `pnpm typecheck` | `tsc --noEmit`                              |
| `pnpm test`      | Vitest unit + RTL tests (`test:watch` mode) |

All of the above run in CI (`.github/workflows/ci.yml` → `frontend` job).

## Where things live

- `src/routes/$locale/…` — public pages, locale-segmented (`en-US`, `zh-CN`).
  The landing and heritage index are horizontal "magazine" pages driven by
  `src/lib/loop-scroller.ts`; the catalog is a record-crate flip deck.
- `src/lib/` — API client (`api.ts`), auth/cart/wishlist/i18n contexts,
  money formatter (display only — all math is server-side), loop-scroller.
- `src/mocks/` — the offline backend (transport mirrors `api/router.go`;
  FX/rounding/shipping math per TDD §7 happens here, never in the UI).
- `src/styles/tokens.css` ↔ `tailwind.config.ts` — design tokens are
  intentional twins (CSS vars for SVG, hex literals for Tailwind alpha).
  A vitest guard (`src/styles/__tests__/tokens-sync.test.ts`) keeps them
  in sync — edit both or neither.
- `screenshots/` — local design-reference captures (gitignored).

The milestone plan lives in `TODO.md`. Screenshots: `pnpm dev` then visit
`/en-US`, `/en-US/catalog`, `/en-US/ceramicstory`, …
