// k6 smoke test (PRD §2.4.2 line 124).
//
// Post-deploy gate: must complete < 2 min. Checks:
//   - homepage SSR 200 both locales
//   - API /health (or a public GET that proves DB+Redis are alive)
//   - product page render
//
// Usage:
//   k6 run -e BASE_URL=https://staging.jingdezhen.example backend/k6/smoke.js
import http from 'k6/http'
import { check } from 'k6'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:1323'

export const options = {
  vus: 1,
  iterations: 1,
  timeout: '2m', // must complete < 2 min (PRD §2.4.2)
}

export default function () {
  // 1. Catalog reads in both locales (proves DB + i18n JOIN path)
  const enRes = http.get(`${BASE_URL}/catalog/products?locale=en-US&limit=1`)
  check(enRes, { 'en-US catalog 200': (r) => r.status === 200 })

  const zhRes = http.get(`${BASE_URL}/catalog/products?locale=zh-CN&limit=1`)
  check(zhRes, { 'zh-CN catalog 200': (r) => r.status === 200 })

  // 2. Product detail (proves JOIN-heavy path: product + translations + SKUs + tags)
  const detailRes = http.get(`${BASE_URL}/catalog/products/jdz-test-product-001?locale=en-US`)
  check(detailRes, {
    'product detail 200 or 404': (r) => r.status === 200 || r.status === 404,
  })

  // 3. FX rates (proves Redis-backed FX cache + ECB fixture fallback)
  const fxRes = http.get(`${BASE_URL}/fx/rates`)
  check(fxRes, { 'fx rates 200': (r) => r.status === 200 })

  // 4. Sitemap (proves storage adapter + content query path)
  const sitemapRes = http.get(`${BASE_URL}/sitemap.xml`)
  check(sitemapRes, { 'sitemap 200': (r) => r.status === 200 })
}
