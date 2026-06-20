// Single source for the E2E server's port + token. Imported by
// playwright.config.ts, scripts/launch.mjs, and tests/*.spec.ts so the values
// the server starts with, the harness polls, and the specs navigate to cannot
// drift. Plain ESM (no TS) so both Node (.mjs) and Playwright's TS bundler
// load it natively.
export const PORT = 8090;
export const TOKEN = 'e2e-token';
