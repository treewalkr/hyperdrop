// Single source for the E2E server's port, token, and upload root. Imported by
// playwright.config.ts, scripts/launch.mjs, and tests/*.spec.ts so the values
// the server starts with, the harness polls, and the specs navigate to cannot
// drift. Plain ESM (no TS) so both Node (.mjs) and Playwright's TS bundler
// load it natively.
import { tmpdir } from 'node:os';
import path from 'node:path';

export const PORT = 8090;
export const TOKEN = 'e2e-token';
// Fixed upload root under the OS temp dir. Shared between scripts/launch.mjs
// (which creates/empties it) and fixtures/isolation.ts (which wipes it between
// tests) — exporting it here keeps the two in lockstep.
export const UPLOAD_DIR = path.join(tmpdir(), 'hyperdrop-e2e-uploads');
