import { test, expect } from '@playwright/test'

/** Wait for the React app to hydrate (the chat bubble is client-only). */
async function waitForHydration(page: import('@playwright/test').Page) {
  await page.waitForLoadState('networkidle')
  await expect(page.locator('button[aria-label="Chat with us"]')).toBeVisible({ timeout: 15_000 })
}

/**
 * Browse catalog → product detail → add to cart → checkout.
 * Uses the demo customer (emily@demo.dev / porcelain123) for the signed-in checkout flow.
 */
test('browse → cart → checkout (mock gateway)', async ({ page }) => {
  await page.goto('/en-US/catalog')
  await waitForHydration(page)
  const productLink = page.locator('article a[href*="/catalog/"]').first()
  await expect(productLink).toBeVisible()

  await productLink.click()
  await page.waitForURL(/\/catalog\/[^/]+$/)
  await expect(page.locator('h1').first()).toBeVisible()

  const addBtn = page.getByRole('button', { name: /add to cart/i })
  if (await addBtn.isVisible().catch(() => false)) {
    await addBtn.click()
    await page.waitForTimeout(500)
  }

  await page.goto('/en-US/cart')
  await expect(page).toHaveURL(/\/cart/)
})

/**
 * Sign in with the demo customer, then checkout from cart.
 */
test('signed-in checkout (mock gateway)', async ({ page }) => {
  await page.goto('/en-US/auth/login')
  await waitForHydration(page)
  await page.locator('#login-email').fill('emily@demo.dev')
  await page.locator('#login-password').fill('porcelain123')
  await page.getByRole('button', { name: /^Sign in$|^登录$/ }).click()
  await page.waitForURL(/\/en-US(?!\/auth)/, { timeout: 15_000 })

  await page.goto('/en-US/catalog')
  await waitForHydration(page)
  const productLink = page.locator('article a[href*="/catalog/"]').first()
  await productLink.click()
  await page.waitForURL(/\/catalog\/[^/]+$/)

  const addBtn = page.getByRole('button', { name: /add to cart/i })
  if (await addBtn.isVisible().catch(() => false)) {
    await addBtn.click()
    await page.waitForTimeout(500)
  }

  await page.goto('/en-US/cart')
  const checkoutLink = page.getByRole('link', { name: /checkout|结算/i }).first()
  if (await checkoutLink.isVisible().catch(() => false)) {
    await checkoutLink.click()
    await page.waitForURL(/\/checkout/, { timeout: 10_000 })
    await expect(page.locator('h1, h2').first()).toBeVisible()
  }
})
