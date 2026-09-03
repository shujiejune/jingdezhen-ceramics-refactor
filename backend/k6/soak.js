// k6 soak test (PRD §2.4.3 scenario 5).
//
// Moderate load for 2–4 hours to catch memory leaks (Go goroutine leaks,
// Redis connection leaks, SSR memory growth). Runs at 60% of baseline RPS
// to be sustainable over a long duration.
//
// Usage:
//   k6 run -e BASE_URL=https://staging.jingdezhen.example backend/k6/soak.js
//
// Set duration via env: SOAK_DURATION=2h (default), SOAK_DURATION=4h (max).
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Rate } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:1323'
const LOCALE = __ENV.LOCALE || 'en-US'
const DURATION = __ENV.SOAK_DURATION || '2h'

const errorRate = new Rate('errors')

export const thresholds = {
  http_req_duration: ['p(95)<300'],     // must hold for the entire soak
  errors: [{ threshold: 'rate<0.001', abort: false }],
  // Leak detection: if VU iterations slow down over time, p95 degrades.
  // The p(95)<300 threshold catches this — a steady leak will eventually
  // push p95 over 300ms and fail the run.
}

export const options = {
  scenarios: {
    soak: {
      executor: 'constant-arrival-rate',
      rate: 30,              // 60% of baseline 50 RPS
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 120,
      maxVUs: 150,
    },
  },
  thresholds,
}

export default function () {
  const res = http.get(`${BASE_URL}/catalog/products?locale=${LOCALE}&limit=20`, {
    headers: { 'Accept': 'application/json', 'Accept-Language': LOCALE },
  })
  check(res, {
    'status 200': (r) => r.status === 200,
  })
  errorRate.add(res.status !== 200)
  sleep(1) // longer think-time for sustainability
}
