import { existsSync, readdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { UPLOAD_DIR } from '../lib/config.mjs';

// Wipes the shared upload root so the caller (a test's beforeEach) can reset
// state between tests. The server keeps running across tests in a worker; this
// is what gives isolation. Kept as a plain helper rather than a test extension
// so the spec owns the Playwright hooks in its own file-suite context.
export function wipeUploads(): void {
  if (!existsSync(UPLOAD_DIR)) return;
  for (const entry of readdirSync(UPLOAD_DIR)) {
    rmSync(path.join(UPLOAD_DIR, entry), { recursive: true, force: true });
  }
}
