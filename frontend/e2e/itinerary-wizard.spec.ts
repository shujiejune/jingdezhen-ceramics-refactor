import { test, expect } from '@playwright/test'

/** Wait for the React app to hydrate (the chat bubble is client-only). */
async function waitForHydration(page: import('@playwright/test').Page) {
  await page.waitForLoadState('networkidle')
  await expect(page.locator('button[aria-label="Chat with us"]')).toBeVisible({ timeout: 15_000 })
}

/**
 * Itinerary wizard — fill out the multi-step form and submit.
 * Requires sign-in for submission. We use the demo customer.
 * The wizard has 5 steps: 1 (trip basics) → 2 (interests/budget) →
 * 3 (services) → 4 (contact + consent) → 5 (review + submit).
 */
test('itinerary wizard submit (mock)', async ({ page }) => {
  // sign in first
  await page.goto('/en-US/auth/login')
  await waitForHydration(page)
  await page.locator('#login-email').fill('emily@demo.dev')
  await page.locator('#login-password').fill('porcelain123')
  await page.getByRole('button', { name: /^Sign in$|^登录$/ }).click()
  await page.waitForURL(/\/en-US(?!\/auth)/, { timeout: 15_000 })

  // go to the itinerary wizard
  await page.goto('/en-US/itinerary')
  await waitForHydration(page)
  await expect(page).toHaveURL(/\/itinerary/)

  // step 1: fill arrival date (required)
  const arrivalInput = page.locator('input[type="date"]').first()
  await arrivalInput.fill('2027-05-15')

  // advance through steps 1→3 (continue button)
  for (let step = 1; step <= 3; step++) {
    const continueBtn = page.getByRole('button', { name: /^Continue$|^继续$/ })
    await expect(continueBtn).toBeVisible({ timeout: 10_000 })
    await continueBtn.click()
    await page.waitForTimeout(800)
  }

  // step 4: check the consent checkbox before advancing
  const consentCheckbox = page.locator('label input[type="checkbox"]').first()
  await expect(consentCheckbox).toBeVisible({ timeout: 10_000 })
  await consentCheckbox.check()

  // continue to step 5 (review)
  const continueBtn = page.getByRole('button', { name: /^Continue$|^继续$/ })
  await continueBtn.click()
  await page.waitForTimeout(800)

  // step 5 (review): submit
  await expect(page.locator('h2').filter({ hasText: /review/i })).toBeVisible({ timeout: 10_000 })
  const submitBtn = page.getByRole('button', { name: /^Send my request$|^发送需求$/ })
  await submitBtn.click()

  // success — the submitted screen renders with a title
  await expect(page.locator('h1').first()).toBeVisible({ timeout: 15_000 })
  await expect(page.locator('body')).toContainText(/submitted|已提交|request received/i)
})
