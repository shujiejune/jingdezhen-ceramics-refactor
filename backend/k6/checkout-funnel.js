// k6 checkout funnel load test (PRD §2.4.3 scenario 2).
//
// Simulates: login → cart operations → shipping-fee calc → order creation
// (payment stubbed in mock mode). Catches locking/transaction issues around
// inventory decrement.
//
// Target: 10 orders/min (PRD §2.4.3 line 137).
// Thresholds: p95 < 300ms, error rate < 0.1%.
import http from 'k6/http'
import { check, group, sleep } from 'k6'
import { Rate, Counter } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:1323'
const TEST_EMAIL = __ENV.TEST_EMAIL || 'customer@jingdezhen.test'
const TEST_PASSWORD = __ENV.TEST_PASSWORD || 'password123'
const TEST_SKU_ID = __ENV.TEST_SKU_ID || '1' // a seeded SKU with stock

const orderCreated = new Counter('orders_created')
const errorRate = new Rate('errors')

export const thresholds = {
  http_req_duration: ['p(95)<300'],
  errors: [{ threshold: 'rate<0.001', abort: false }],
}

export const options = {
  scenarios: {
    checkout_funnel: {
      executor: 'constant-arrival-rate',
      rate: 10,              // 10 orders/min → 1 per 6s
      timeUnit: '6s',
      duration: '2m',
      preAllocatedVUs: 20,
      maxVUs: 30,
    },
  },
  thresholds,
}

function login() {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: TEST_EMAIL, password: TEST_PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  if (res.status !== 200) return null
  try {
    const body = JSON.parse(res.body)
    return body.access_token || body.accessToken || null
  } catch {
    return null
  }
}

function authHeaders(token) {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
    'Accept-Language': 'en-US',
  }
}

export default function () {
  const token = login()
  if (!token) {
    errorRate.add(true)
    return
  }
  const headers = authHeaders(token)

  group('add_to_cart', () => {
    const res = http.post(
      `${BASE_URL}/cart/items`,
      JSON.stringify({ sku_id: parseInt(TEST_SKU_ID), qty: 1 }),
      { headers },
    )
    check(res, { 'cart add 200': (r) => r.status === 200 })
    errorRate.add(res.status !== 200)
  })

  group('get_cart', () => {
    const res = http.get(`${BASE_URL}/cart`, { headers })
    check(res, { 'cart get 200': (r) => r.status === 200 })
    errorRate.add(res.status !== 200)
  })

  group('shipping_quote', () => {
    const res = http.get(`${BASE_URL}/shipping/quote?country=US`, { headers })
    check(res, { 'shipping 200': (r) => r.status === 200 })
    errorRate.add(res.status !== 200)
  })

  group('checkout', () => {
    // Checkout requires an address ID. In mock mode, payment:finalize
    // drives created→paid via the worker. The checkout endpoint creates
    // the order with the atomic stock decrement (TDD §4.3).
    const res = http.post(
      `${BASE_URL}/checkout`,
      JSON.stringify({
        address_id: 1, // seeded address
        currency: 'USD',
      }),
      { headers },
    )
    // 201 = order created; 409 = insufficient stock (a race under load —
    // not an error, the system correctly rejected the oversell).
    const ok = res.status === 201 || res.status === 409
    check(res, { 'checkout 201 or 409': () => ok })
    errorRate.add(!ok)
    if (res.status === 201) orderCreated.add(1)
  })

  sleep(1)
}
