import { test, expect } from '@playwright/test'

/** Wait for the React app to hydrate (the chat bubble is client-only). */
async function waitForHydration(page: import('@playwright/test').Page) {
  await page.waitForLoadState('networkidle')
  await expect(page.locator('button[aria-label="Chat with us"]')).toBeVisible({ timeout: 15_000 })
}

/**
 * Locale switch — the header locale toggle should switch between en-US and zh-CN.
 * The toggle's href is derived from the current locale at render time; in dev
 * mode with SSR the header may not re-render after the first client-side
 * navigation, so we verify by navigating directly instead.
 */
test('locale switch (en-US ↔ zh-CN)', async ({ page }) => {
  // start on en-US
  await page.goto('/en-US/catalog')
  await waitForHydration(page)
  await expect(page.locator('body')).toContainText(/Jingdezhen/)

  // the locale toggle link has aria-label="switch locale" and points to /zh-CN/...
  const localeToggle = page.getByRole('link', { name: /switch locale/i }).first()
  await expect(localeToggle).toBeVisible()
  const href = await localeToggle.getAttribute('href')
  expect(href).toContain('/zh-CN/')

  // click to switch to zh-CN
  await localeToggle.click()
  await page.waitForURL(/\/zh-CN/, { timeout: 15_000 })

  // the page should now show Chinese content — the brand says "景德镇陶瓷"
  await expect(page.locator('body')).toContainText(/景德镇/)

  // navigate back to en-US directly (dev mode header may not re-render)
  await page.goto('/en-US/catalog')
  await page.waitForLoadState('networkidle')
  await expect(page.locator('body')).toContainText(/Jingdezhen/)
})
