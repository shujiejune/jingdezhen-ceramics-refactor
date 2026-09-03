# k6 Load Tests (PRD §2.4.3)

## Install

```bash
# macOS
brew install k6

# Linux (Arch)
pacman -S k6

# Or via go install
go install go.k6.io/k6@latest
```

## Run

All scripts accept `BASE_URL` env var. Defaults to `http://localhost:1323` (dev).

```bash
# Smoke (post-deploy gate, < 2 min, PRD §2.4.2)
k6 run backend/k6/smoke.js

# Browse-heavy baseline (50 RPS, 200 VUs, 2 min)
k6 run backend/k6/browse-baseline.js

# Checkout funnel (10 orders/min, 2 min)
k6 run -e TEST_EMAIL=customer@jingdezhen.test -e TEST_PASSWORD=password123 backend/k6/checkout-funnel.js

# WebSocket sessions (ramp to 500, 2.5 min)
k6 run -e BASE_URL=ws://localhost:1323 -e TEST_TOKEN=<jwt> backend/k6/ws-sessions.js

# Spike test (10× baseline burst, 2.5 min)
k6 run backend/k6/spike.js

# Soak test (30 RPS, 2h — change with SOAK_DURATION=4h)
k6 run -e SOAK_DURATION=2h backend/k6/soak.js
```

## Thresholds (PRD §2.4.3 line 135)

| Metric | Threshold | Gate |
|--------|-----------|------|
| `http_req_duration` | p95 < 300 ms | pipeline-failing |
| `errors` (error rate) | < 0.1% | pipeline-failing |
| `ws_connecting` | p95 < 1000 ms | WS only, looser |

A threshold breach fails the k6 run (exit code != 0). Wire into CI as a
nightly/pre-release job (PRD §2.4.1 line 105: "nightly / pre-release: load
tests against staging").

## Launch-Scale Targets (PRD §2.4.3 line 137)

| Scenario | Target |
|----------|--------|
| Browse baseline | 50 RPS / 200 concurrent users |
| Checkout funnel | 10 orders/min |
| WebSocket | 500 concurrent sessions |
| Spike | 10× baseline |

## Notes

- Run against **staging on the same VPS spec as production** (PRD §2.4.3 line 136).
- The checkout funnel may return 409 (insufficient stock) under load —
  this is the correct atomic-decrement behavior (TDD §4.3), not an error.
- The WS test requires a valid JWT (use `?token=<jwt>` from a login).
- The smoke test checks both locales + sitemap + FX (proves DB + Redis +
  storage are alive).
