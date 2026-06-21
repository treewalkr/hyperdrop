import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { TOKEN } from '../lib/config.mjs';

// The token comes from lib/config.mjs — the single source shared with the
// launch script and playwright.config.ts. The first page request sets the
// session cookie; the token is only needed for that initial navigation.

// Writes a fixture file into a unique temp subdir and returns its path. The
// basename is preserved so the page shows the intended filename; the unique
// parent keeps parallel workers from clobbering each other.
let fixtureCounter = 0;
const fixtureDirs: string[] = [];
function fixture(name: string, contents: string): string {
  const dir = path.join(tmpdir(), `hd-e2e-${process.pid}-${fixtureCounter++}`);
  fixtureDirs.push(dir);
  mkdirSync(dir, { recursive: true });
  const p = path.join(dir, name);
  writeFileSync(p, contents);
  return p;
}

// Every test starts from an empty upload root.
test.beforeEach(() => wipeUploads());
// Drop the fixture temp dirs so the suite doesn't litter $TMPDIR across runs.
test.afterAll(() => {
  for (const dir of fixtureDirs) rmSync(dir, { recursive: true, force: true });
});

test('Send page renders the dropzone', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);
  await expect(page.getByTestId('dropzone')).toBeVisible();
});

test('uploading a file via the input shows its name and a success toast', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);
  await page.getByTestId('file-input').setInputFiles(fixture('hello.txt', 'hello e2e'));

  await expect(page.getByTestId('file-name')).toHaveText('hello.txt');
  await expect(page.getByTestId('toasts')).toContainText('hello.txt uploaded');
});

test('uploaded file appears on the Files page and can be downloaded', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);
  const body = 'round trip body';
  await page.getByTestId('file-input').setInputFiles(fixture('round-trip.txt', body));

  // Wait for the upload to land on disk before leaving the Send page.
  await expect(page.getByTestId('toasts')).toContainText('round-trip.txt uploaded');

  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);
  // `file-name` is also used on the Send page's file cards; here it resolves to
  // the single uploaded row. Once multi-file specs land, scope this locator
  // (e.g. within a `file-item`) to avoid ambiguity.
  await expect(page.getByTestId('file-name')).toHaveText('round-trip.txt');

  // The download action is hover-revealed; force-click skips that visibility gate.
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByTestId('download').click({ force: true }),
  ]);
  expect(download.suggestedFilename()).toBe('round-trip.txt');
  // Close the loop on the actual bytes, not just the filename/event.
  expect(readFileSync(await download.path(), 'utf8')).toBe(body);
});

// Regression coverage for issue #15 (server-reported upload progress). The
// server-authoritative progress path had instability bugs on fast links / small
// files that these specs pin down:
//
//   1. The Send page opened the progress WebSocket twice. Alpine v3 auto-calls
//      a component's init(); the body also had x-init="init()", so init() ran
//      twice → connectWS() twice → every event processed twice.
//   2. xhr.onload finalized the card off the *raw* entry object (not Alpine's
//      reactive proxy in this.files), so status flipped to 'done' in the data
//      but the DOM never re-rendered — the card froze on "uploading" while the
//      success toast still fired. It also bypassed the "hold the bar at 100%"
//      delay and could double-toast (once per socket).
//
// Both are fixed: init() runs once (single socket), and both completion signals
// route through one finalizeUpload() that mutates the reactive proxy, holds the
// bar at 100% briefly, and finalizes exactly once.
//
// These live in this file (not their own) because the suite shares one server
// and one upload root across spec files; keeping them here runs them in order
// in a single worker instead of racing the shared dir with another file.

// Patches the page before any app script runs to count WebSocket opens to
// /api/ws. The Send page must open exactly one progress socket.
async function expectSingleProgressSocket(page: import('@playwright/test').Page): Promise<void> {
  await page.addInitScript(() => {
    (window as any).__wsOpens = 0;
    const OrigWS = (window as any).WebSocket;
    (window as any).WebSocket = class extends OrigWS {
      constructor(url: string, protocols?: any) {
        super(url, protocols);
        if (typeof url === 'string' && url.includes('/api/ws')) {
          (window as any).__wsOpens++;
        }
      }
    };
  });
}

const progressCases: Array<[string, number]> = [['small', 512], ['chunk-boundary', 65537], ['large', 1 << 20]];
for (const [label, size] of progressCases) {
  test(`uploading a ${label} file opens one socket, fills the bar, and toasts once`, async ({ page }) => {
    const name = `${label}.bin`;
    await expectSingleProgressSocket(page);
    await page.goto(`/?token=${TOKEN}`);
    expect(await page.evaluate(() => (window as any).__wsOpens)).toBe(1);

    await page.getByTestId('file-input').setInputFiles(fixture(name, 'a'.repeat(size)));

    // The card reaches the done state and the bar filled to 100%.
    await expect(page.locator('[data-testid="file-item"] .done-label')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('[data-testid="file-item"] .progress-bar .fill')).toHaveClass(/done/);

    // Exactly one success toast for this file — the double-socket /
    // double-completion races would render a second toast node. The toast
    // fires in the same block as the done-label, so it is present the moment
    // done-label appears (auto-dismisses after 2.5s).
    await expect(page.locator('[data-testid="toasts"] .toast.success', { hasText: name })).toHaveCount(1);
  });
}

