import { defineConfig, devices } from '@playwright/test'

/**
 * E2E config — runs against the Vite dev server in mock mode.
 * The webServer starts `pnpm dev` on port 3000 with VITE_API_MODE=mock.
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'retain-on-failure',
    locale: 'en-US',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:3000',
    timeout: 60_000,
    reuseExistingServer: !process.env.CI,
    env: { ...process.env, VITE_API_MODE: 'mock' },
  },
})
