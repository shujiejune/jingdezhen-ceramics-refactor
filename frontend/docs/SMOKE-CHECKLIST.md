# Post-Deploy Smoke Checklist

Run these after every staging/prod deploy. All items must pass before
declaring the deploy stable.

## 1. Frontend SSR

- [ ] `GET /` returns 200 with rendered HTML (not empty `<div>`)
- [ ] `GET /en-US/catalog` returns 200 with product cards
- [ ] `GET /zh-CN/catalog` returns 200 with Chinese content
- [ ] `GET /en-US/catalog/<slug>` returns 200 with product detail
- [ ] `GET /en-US/itinerary` returns 200 with wizard form
- [ ] `GET /en-US/certificates/<code>` returns 200 with cert details + QR

## 2. API reverse proxy

- [ ] `GET /api/profile` (with JWT) returns 200 (not 404 or 502)
- [ ] `GET /api/products` returns 200 JSON (prefix stripped → Fiber)
- [ ] `GET /webhooks/airwallex` returns expected status (not 404)

## 3. WebSocket

- [ ] `wss://<domain>/ws?token=<jwt>` upgrades successfully (101)
- [ ] Push notification arrives on client (signed-in user)

## 4. Auth flows

- [ ] Login (email + password) → redirect to home, session cookie set
- [ ] Signup → activation email sent → activation link works
- [ ] 2FA enrollment → QR renders, TOTP code accepted
- [ ] Logout → session cleared, redirect to home

## 5. Commerce

- [ ] Add to cart → cart count badge updates
- [ ] Cart → checkout → address form → payment gateway (sandbox)
- [ ] Wishlist add/remove → heart toggle + count updates
- [ ] Order history shows past orders (signed-in)

## 6. Itinerary

- [ ] Wizard 5-step form completes → submitted confirmation
- [ ] Admin console: itinerary inbox shows new request

## 7. Chat (when backend unblocked)

- [ ] Chat bubble opens, bot greets
- [ ] Escalate → agent console shows session
- [ ] Agent reply → customer sees message

## 8. i18n + SEO

- [ ] `<html lang>` matches locale path segment
- [ ] hreflang / canonical tags present in `<head>`
- [ ] `sitemap.xml` reachable
- [ ] `robots.txt` reachable

## 9. Static assets

- [ ] CSS file loads (no 404 on hashed asset URLs)
- [ ] JS chunks load (no 404, no MIME errors)
- [ ] Fonts load (Inter woff2, no CORS errors)
- [ ] favicon.svg loads

## 10. Env matrix verification

- [ ] `VITE_API_MODE=live` (not mock) in the running container
- [ ] `SITE_BASE_URL` matches the actual domain
- [ ] `CLIENT_ORIGIN` matches the frontend origin (CORS)
- [ ] Backend `CLIENT_ORIGIN` env allows the frontend origin
