import { defineConfig, devices } from '@playwright/test';

// Playwright is the driver, Bun is the package manager, Node is the runtime.
// Install deps with `bun install`; run Playwright with `npx playwright …`.
// Never route the Playwright CLI through Bun — `scripts/check-toolchain.mjs`
// guards against that (it is unsupported/flaky).

// Shared with scripts/launch.mjs and tests/*.spec.ts via lib/config.mjs so the
// token/port the server starts with and the harness polls cannot drift.
import { PORT, TOKEN } from './lib/config.mjs';

export default defineConfig({
  testDir: './tests',
  // One shared server + one shared upload root. More than one worker would run
  // dir-touching spec files (upload, player) concurrently against that single
  // root and race on per-test wipes/uploads, so the whole suite is serial.
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  // 'list' for live terminal output; 'html' so CI can upload playwright-report/
  // on failure (HTML is the report path the artifact step captures).
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never' }]]
    : 'list',
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
    // Locally, reuse a server already on :PORT if present — convenient for
    // re-runs, but note it will attach to *any* process answering the URL,
    // not necessarily one this harness started. CI always starts fresh.
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
