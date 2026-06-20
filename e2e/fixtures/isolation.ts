import { existsSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

// The upload root the launch script creates and the server writes into.
const uploadDir = path.join(tmpdir(), 'hyperdrop-e2e-uploads');

// Wipes the shared upload root so the caller (a test's beforeEach) can reset
// state between tests. The server keeps running across tests in a worker; this
// is what gives isolation. Kept as a plain helper rather than a test extension
// so the spec owns the Playwright hooks in its own file-suite context.
export function wipeUploads(): void {
  if (!existsSync(uploadDir)) return;
  for (const entry of readdirSync(uploadDir)) {
    rmSync(path.join(uploadDir, entry), { recursive: true, force: true });
  }
}
