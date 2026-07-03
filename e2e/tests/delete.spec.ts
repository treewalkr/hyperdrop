import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { TOKEN, UPLOAD_DIR } from '../lib/config.mjs';

// Delete flow via the Files page (issue #22). Mirrors upload.spec.ts: same
// fixture() helper, the same per-test wipeUploads() reset, and the same
// upload-then-navigate-to-Files pattern. The whole suite shares one server
// and one upload root, so this file owns its hooks in its own file-suite
// context — exactly like upload.spec.ts.

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

test('a file uploaded via Send can be deleted from the Files page', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);
  const body = 'delete me body';
  await page.getByTestId('file-input').setInputFiles(fixture('hello.txt', body));

  // Wait for the upload to land on disk before leaving the Send page.
  await expect(page.getByTestId('toasts')).toContainText('hello.txt uploaded');

  // Navigate to the Files page and confirm the row is present.
  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);
  await expect(page.getByTestId('file-name')).toHaveText('hello.txt');
  expect(existsSync(path.join(UPLOAD_DIR, 'hello.txt'))).toBe(true);

  // The delete action is hover-revealed; force-click skips that visibility gate.
  // Deletion goes through a CUSTOM confirm modal (not window.confirm), so after
  // clicking delete we must locate and click the modal's confirm button by its
  // "Delete" label (confirmOkLabel rendered into the .btn-danger button).
  await page.getByTestId('delete').click({ force: true });
  const confirmButton = page.locator('.modal-overlay .btn-danger', { hasText: 'Delete' });
  await expect(confirmButton).toBeVisible();
  await confirmButton.click();

  // Success toast surfaces and the row disappears from the UI.
  await expect(page.getByTestId('toasts')).toContainText('hello.txt deleted');
  await expect(page.getByTestId('file-item')).toHaveCount(0);
  await expect(page.getByTestId('file-name')).toHaveCount(0);

  // Close the loop on disk: the file is actually gone, not just hidden in the UI.
  expect(existsSync(path.join(UPLOAD_DIR, 'hello.txt'))).toBe(false);
});
