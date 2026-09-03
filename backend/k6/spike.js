// k6 spike test (PRD §2.4.3 scenario 4).
//
// Sudden 10× traffic burst (e.g. marketing campaign) to verify graceful
// degradation. Baseline = 50 RPS; spike = 500 RPS.
// Thresholds: p95 < 300ms (hard); error rate < 0.1% during the burst.
//
// Usage:
//   k6 run -e BASE_URL=https://staging.jingdezhen.example backend/k6/spike.js
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Rate } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:1323'
const LOCALE = __ENV.LOCALE || 'en-US'

const errorRate = new Rate('errors')

export const thresholds = {
  http_req_duration: ['p(95)<300'],
  errors: [{ threshold: 'rate<0.001', abort: false }],
}

export const options = {
  scenarios: {
    spike: {
      executor: 'ramping-arrival-rate',
      startRate: 50,          // baseline: 50 RPS
      timeUnit: '1s',
      preAllocatedVUs: 600,   // 10× baseline needs headroom
      maxVUs: 800,
      stages: [
        { duration: '30s', target: 50 },   // baseline
        { duration: '5s', target: 500 },   // 10× spike (sudden)
        { duration: '1m', target: 500 },   // hold at spike
        { duration: '10s', target: 50 },   // recover
        { duration: '30s', target: 50 },   // back to baseline
      ],
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
  sleep(0.1)
}
