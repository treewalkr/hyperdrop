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

