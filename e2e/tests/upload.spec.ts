import { mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';

// Token the launch script starts the binary with. The first page request sets
// the session cookie; the token is only needed for that initial navigation.
const TOKEN = 'e2e-token';

// Writes a fixture file into a unique temp subdir and returns its path. The
// basename is preserved so the page shows the intended filename; the unique
// parent keeps parallel workers from clobbering each other.
let fixtureCounter = 0;
function fixture(name: string, contents: string): string {
  const dir = path.join(tmpdir(), `hd-e2e-${process.pid}-${fixtureCounter++}`);
  mkdirSync(dir, { recursive: true });
  const p = path.join(dir, name);
  writeFileSync(p, contents);
  return p;
}

// Every test starts from an empty upload root.
test.beforeEach(() => wipeUploads());

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
  await page.getByTestId('file-input').setInputFiles(fixture('round-trip.txt', 'round trip body'));

  // Wait for the upload to land on disk before leaving the Send page.
  await expect(page.getByTestId('toasts')).toContainText('round-trip.txt uploaded');

  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);
  await expect(page.getByTestId('file-name')).toHaveText('round-trip.txt');

  // The download action is hover-revealed; force-click skips that visibility gate.
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByTestId('download').click({ force: true }),
  ]);
  expect(download.suggestedFilename()).toBe('round-trip.txt');
});

