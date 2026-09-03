import { test, expect } from '@playwright/test'

/**
 * Certificate verification — visit the public certificate page by code,
 * verify the cert details + QR render.
 */
test('certificate verify (mock)', async ({ page }) => {
  // JDZ-2026-A7F3 is the first mock certificate (product_id 4)
  await page.goto('/en-US/certificates/JDZ-2026-A7F3')
  await expect(page).toHaveURL(/\/certificates\/JDZ-2026-A7F3/)

  // the certificate code should appear somewhere on the page
  await expect(page.locator('body')).toContainText('JDZ-2026-A7F3')

  // the QR code image should be present (rendered as a data URL)
  const qrImg = page.locator('img[alt*="QR" i], img[alt*="二维码"]')
  await expect(qrImg.first()).toBeVisible()

  // certificate of authenticity heading or seal
  await expect(page.locator('body')).toContainText(/certificate|authenticity|证书/i)
})
