#!/usr/bin/env node
// webServer command: resolves the binary, prepares a fresh upload root, and
// runs `hyperdrop --token e2e-token --port 8090 <uploadDir>` as a foreground
// child so Playwright sees a live process. Tears the child down on SIGTERM/SIGINT.
//
// Binary resolution honors the E2E_BIN override (CI passes a pre-built binary,
// local devs can skip the in-script rebuild); building from source is a fallback
// that only runs when E2E_BIN is unset.
import { spawn, execFileSync } from 'node:child_process';
import { mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
// e2e/scripts/launch.mjs -> e2e/ then repo root.
const e2eDir = path.resolve(here, '..');
const repoRoot = path.resolve(e2eDir, '..');

const TOKEN = 'e2e-token';
const PORT = 8090;
// Fixed upload root under the OS temp dir, shared with the isolation fixture.
const uploadDir = path.join(tmpdir(), 'hyperdrop-e2e-uploads');

function resolveBinary() {
  if (process.env.E2E_BIN) return process.env.E2E_BIN;
  const out = path.join(e2eDir, '.cache', 'hyperdrop');
  mkdirSync(path.dirname(out), { recursive: true });
  execFileSync('go', ['build', '-o', out, './cmd/hyperdrop'], {
    cwd: repoRoot,
    stdio: 'inherit',
  });
  return out;
}

// Begin each webServer start from an empty upload root.
rmSync(uploadDir, { recursive: true, force: true });
mkdirSync(uploadDir, { recursive: true });

const bin = resolveBinary();
const child = spawn(bin, ['--token', TOKEN, '--port', String(PORT), uploadDir], {
  stdio: ['ignore', 'inherit', 'inherit'],
});

const shutdown = () => {
  try {
    child.kill('SIGTERM');
  } catch {
    /* already gone */
  }
};
process.on('SIGTERM', () => {
  shutdown();
  process.exit(0);
});
process.on('SIGINT', () => {
  shutdown();
  process.exit(0);
});
child.on('exit', (code) => process.exit(code ?? 0));
