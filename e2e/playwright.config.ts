import { defineConfig, devices } from '@playwright/test';

// Playwright is the driver, Bun is the package manager, Node is the runtime.
// Install deps with `bun install`; run Playwright with `npx playwright …`.
// Never route the Playwright CLI through Bun — `scripts/check-toolchain.mjs`
// guards against that (it is unsupported/flaky).

const PORT = 8090;
const TOKEN = 'e2e-token';

export default defineConfig({
  testDir: './tests',
  // One shared server + one shared upload root: tests in a file must run in
  // order so per-test wipes/uploads don't race across parallel workers.
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  use: {
    baseURL: `http://localhost:${PORT}`,
    actionTimeout: 10_000,
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  // Launches the real Go binary via scripts/launch.mjs (which honors E2E_BIN).
  // Polling the token-bearing URL confirms the server actually answers HTTP.
  webServer: {
    command: 'node scripts/launch.mjs',
    url: `http://localhost:${PORT}/?token=${TOKEN}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
