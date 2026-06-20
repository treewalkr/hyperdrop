#!/usr/bin/env node
// Enforces the bun-installs / npx-runs rule for Playwright. The Playwright CLI
// must never be routed through Bun (`bunx`, `bun run playwright`, `bun playwright`)
// because that path is unsupported/flaky. Install deps with `bun install`; run
// Playwright with `npx playwright …`.
//
// Scans code/script/config files (not prose .md, which legitimately quotes the
// forbidden forms to warn against them). Skips its own source, which has to name
// the tokens to match them. Also scans the repo's CI workflows — the most likely
// place someone reaches for `bunx` — not just the e2e/ tree.
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = path.resolve(root, '..');
const workflowsDir = path.join(repoRoot, '.github', 'workflows');
const self = fileURLToPath(import.meta.url);
const SKIP_DIRS = new Set(['node_modules', '.cache', 'test-results', 'playwright-report', '.git']);
const SCAN_EXT = /\.(mjs|js|ts|cjs|json|sh|yml|yaml)$/;

const FORBIDDEN = [
  { re: /\bbunx\b/, label: '`bunx`' },
  { re: /\bbun\s+run\s+playwright\b/, label: '`bun run playwright`' },
  { re: /\bbun\s+playwright\b/, label: '`bun playwright`' },
];

const offenders = [];

function walk(dir) {
  for (const entry of readdirSync(dir)) {
    if (SKIP_DIRS.has(entry)) continue;
    const p = path.join(dir, entry);
    if (p === self) continue;
    const st = statSync(p);
    if (st.isDirectory()) {
      walk(p);
    } else if (SCAN_EXT.test(entry)) {
      const text = readFileSync(p, 'utf8');
      for (const { re, label } of FORBIDDEN) {
        const m = text.match(re);
        if (m) offenders.push(`${path.relative(root, p)}: ${label}`);
      }
    }
  }
}

walk(root);
if (existsSync(workflowsDir)) walk(workflowsDir);

if (offenders.length) {
  console.error('Playwright toolchain rule violated (bun installs, npx runs):');
  for (const o of offenders) console.error('  - ' + o);
  console.error('Use `npx playwright …`; never bunx / `bun run playwright` / `bun playwright`.');
  process.exit(1);
}

console.log('Toolchain check passed: no bun-routed Playwright usage found.');
