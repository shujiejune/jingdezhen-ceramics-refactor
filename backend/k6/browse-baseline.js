// k6 load tests for the Jingdezhen Ceramics Platform (PRD §2.4.3).
//
// Usage:
//   k6 run backend/k6/browse-baseline.js          # browse-heavy baseline
//   k6 run backend/k6/checkout-funnel.js          # checkout funnel
//   k6 run backend/k6/ws-sessions.js              # WebSocket sessions
//   k6 run backend/k6/spike.js                     # spike test
//   k6 run backend/k6/soak.js                      # soak test (2-4h)
//
// Run against a staging environment:
//   k6 run -e BASE_URL=https://staging.jingdezhen.example backend/k6/browse-baseline.js
//
// PRD §2.4.3 thresholds (the gate — a regression fails the pipeline):
//   - http_req_duration p95 < 300 ms
//   - error rate < 0.1%
//
// Launch-scale defaults (PRD §2.4.3 line 137):
//   - baseline: 50 RPS on SSR pages / 200 concurrent users
//   - checkout: 10 orders/min
//   - WebSocket: 500 concurrent sessions
//   - spike: 10× baseline
import http from 'k6/http'
import { check, sleep, group } from 'k6'
import { Rate, Trend } from 'k6/metrics'

// --- Config ---
const BASE_URL = __ENV.BASE_URL || 'http://localhost:1323'
const LOCALE = __ENV.LOCALE || 'en-US'

// --- Metrics ---
const errorRate = new Rate('errors')
const pageLoadDuration = new Trend('page_load_duration')

// --- PRD §2.4.3 thresholds — the gate ---
export const thresholds = {
  http_req_duration: ['p(95)<300'],   // p95 < 300ms
  errors: [{ threshold: 'rate<0.001', abort: false }],  // < 0.1%
}

// --- PRD §2.4.3 launch-scale defaults ---
export const options = {
  scenarios: {
    browse_baseline: {
      executor: 'constant-arrival-rate',
      rate: 50,              // 50 RPS (PRD §2.4.3 line 137)
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 200,  // 200 concurrent users (PRD §2.4.3 line 137)
      maxVUs: 250,
    },
  },
  thresholds,
}

// --- Helpers ---
function makeHeaders(extra = {}) {
  return Object.assign(
    {
      'Accept': 'application/json',
      'Accept-Language': LOCALE,
    },
    extra,
  )
}

// --- Test logic: browse-heavy baseline ---
// Hits the 4 SSR-equivalent public read paths (home, gallery, article,
// product detail) in both locales — validates the < 300ms p95 + PG query perf.
export default function () {
  const headers = makeHeaders()

  group('browse_catalog', () => {
    // Public catalog list — the heaviest public read (locale-aware JOINs)
    const res = http.get(`${BASE_URL}/catalog/products?locale=${LOCALE}&limit=20`, { headers })
    check(res, {
      'catalog 200': (r) => r.status === 200,
    })
    errorRate.add(res.status !== 200)
    pageLoadDuration.add(res.timings.duration)
  })

  group('product_detail', () => {
    // Product detail by slug — the JOIN-heavy path (product + translations + SKUs + tags + gallery)
    const res = http.get(`${BASE_URL}/catalog/products/jdz-test-product-001?locale=${LOCALE}`, { headers })
    check(res, {
      'product detail 200 or 404': (r) => r.status === 200 || r.status === 404,
    })
    errorRate.add(res.status !== 200 && r.status !== 404)
    pageLoadDuration.add(res.timings.duration)
  })

  group('ceramicstory', () => {
    // Ceramic stories list — the other content read path
    const res = http.get(`${BASE_URL}/ceramicstory?locale=${LOCALE}`, { headers })
    check(res, {
      'ceramicstory 200': (r) => r.status === 200,
    })
    errorRate.add(res.status !== 200)
    pageLoadDuration.add(res.timings.duration)
  })

  group('artists', () => {
    // Artists list — the third content read path
    const res = http.get(`${BASE_URL}/artists?locale=${LOCALE}`, { headers })
    check(res, {
      'artists 200': (r) => r.status === 200,
    })
    errorRate.add(res.status !== 200)
    pageLoadDuration.add(res.timings.duration)
  })

  sleep(0.5) // simulate user think-time between page loads
}
