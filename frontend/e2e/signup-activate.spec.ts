import { test, expect } from '@playwright/test'

/** Wait for the React app to hydrate (the chat bubble is client-only). */
async function waitForHydration(page: import('@playwright/test').Page) {
  await page.waitForLoadState('networkidle')
  // the chat bubble button (aria-label="Chat with us") only renders after hydration
  await expect(page.locator('button[aria-label="Chat with us"]')).toBeVisible({ timeout: 15_000 })
}

/**
 * Signup → activation flow.
 * Mock backend returns an activation token; the signup page shows a dev
 * activation link. We follow it, land on /auth/activate?token=…, and the
 * mock backend exchanges the token for a session.
 */
test('signup + activate (mock)', async ({ page }) => {
  const email = `e2e_${Date.now()}@test.dev`

  await page.goto('/en-US/auth/signup')
  await waitForHydration(page)

  await page.locator('#su-nickname').fill('E2E Tester')
  await page.locator('#su-email').fill(email)
  await page.locator('#su-password').fill('test12345')
  await page.locator('#su-confirm').fill('test12345')

  // agree to ToS + privacy
  await page
    .locator('label')
    .filter({ hasText: /Terms of Service/i })
    .click()
  await page
    .locator('label')
    .filter({ hasText: /Privacy Policy/i })
    .click()

  await page.getByRole('button', { name: /Create account/i }).click()

  // the mock backend returns needsActivation=true and a dev activation link
  await expect(page.locator('a[href*="/auth/activate"]').first()).toBeVisible({ timeout: 10_000 })

  // follow the activation link
  await page.locator('a[href*="/auth/activate"]').first().click()
  // the activate page auto-activates and redirects to home
  await page.waitForURL(/\/en-US(?!\/auth)/, { timeout: 15_000 })
})
