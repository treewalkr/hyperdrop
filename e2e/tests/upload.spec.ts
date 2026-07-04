import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { fixture, cleanupFixtureDirs } from '../fixtures/files.js';
import { TOKEN, UPLOAD_DIR } from '../lib/config.mjs';

// The token comes from lib/config.mjs — the single source shared with the
// launch script and playwright.config.ts. The first page request sets the
// session cookie; the token is only needed for that initial navigation.

// Every test starts from an empty upload root.
test.beforeEach(() => wipeUploads());
// Drop the fixture temp dirs so the suite doesn't litter $TMPDIR across runs.
test.afterAll(() => cleanupFixtureDirs());

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

// =============================================================================
// Directory upload (issue #30). A folder is uploaded whole and its nested
// structure is preserved on the server.
//
// The native folder picker (webkitdirectory) and the drag-drop entry traversal
// can't be driven faithfully from a headless browser, so these specs exercise
// the real intake code path by handing the Alpine component synthetic File
// objects that carry webkitRelativePath — exactly the shape the picker/drop
// handlers produce. This still runs the real FormData/XHR upload, the real
// backend (MkdirAll + nesting + sandbox), and the real Files-page render.
// =============================================================================

// Builds [{file, rel}] in-page and hands it to the Send page's folder intake —
// the same `intakeFolderEntries` that drag-and-drop resolves a dropped directory
// into. A headless browser can't synthesize a real folder drag (no
// webkitGetAsEntry entries), so we drive the production intake directly. The rel
// carries the full nested path; assertions happen on the rendered DOM and disk.
async function uploadFolderViaComponent(
  page: import('@playwright/test').Page,
  topFolder: string,
  files: Array<{ rel: string; body: string }>,
): Promise<void> {
  await page.evaluate(
    ({ topFolder, files }) => {
      const comp = (window as any).Alpine.$data(document.body);
      const items = files.map((f) => ({
        rel: `${topFolder}/${f.rel}`,
        file: new File([f.body], f.rel.split('/').pop()!, { type: 'text/plain' }),
      }));
      comp.intakeFolderEntries(items, topFolder);
    },
    { topFolder, files },
  );
}

test('a folder upload shows one aggregate card and nests files on the Files page', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  await uploadFolderViaComponent(page, 'vacation', [
    { rel: 'postcard.txt', body: 'wish you were here' },
    { rel: 'photos/sunset.jpg', body: 'jpeg-bytes' },
    { rel: 'photos/raw/keep.dng', body: 'raw-bytes' },
  ]);

  // Exactly one aggregate folder card (not three flat file rows).
  await expect(page.getByTestId('folder-item')).toHaveCount(1);
  await expect(page.getByTestId('folder-name')).toHaveText('vacation');

  // The aggregate card reaches done; its child rows render underneath.
  await expect(page.locator('[data-testid="folder-item"].state-done')).toBeVisible({ timeout: 5000 });
  await expect(page.getByTestId('folder-children').getByTestId('file-item')).toHaveCount(3);

  // Files landed nested on disk.
  expect(readFileSync(path.join(UPLOAD_DIR, 'vacation', 'postcard.txt'), 'utf8')).toBe('wish you were here');
  expect(readFileSync(path.join(UPLOAD_DIR, 'vacation', 'photos', 'sunset.jpg'), 'utf8')).toBe('jpeg-bytes');
  expect(readFileSync(path.join(UPLOAD_DIR, 'vacation', 'photos', 'raw', 'keep.dng'), 'utf8')).toBe('raw-bytes');

  // The Files page shows the top folder as a directory and nests underneath.
  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);

  // vacation appears as a directory row; click into it and descend the tree.
  const folderRow = (name: string) =>
    page.locator('[data-testid="file-name"].is-folder', { hasText: name }).first();

  await folderRow('vacation').click();
  await expect(folderRow('photos')).toBeVisible();
  await folderRow('photos').click();
  await expect(page.locator('[data-testid="file-name"]', { hasText: 'sunset.jpg' })).toBeVisible();
});

test('a folder upload summarises into one success toast, not one per file', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  await uploadFolderViaComponent(page, 'album', [
    { rel: 'a.txt', body: 'a' },
    { rel: 'b.txt', body: 'b' },
    { rel: 'sub/c.txt', body: 'c' },
  ]);

  // Wait for the folder card to finish, then assert exactly ONE success toast
  // — the summary "album · 3 files uploaded" — rather than three per-file ones.
  await expect(page.locator('[data-testid="folder-item"].state-done')).toBeVisible({ timeout: 5000 });
  await expect(page.getByTestId('toasts')).toContainText('album · 3 files uploaded');
  await expect(page.locator('[data-testid="toasts"] .toast.success')).toHaveCount(1);
});

test('dropping two folders yields two separate aggregate cards', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  // A headless browser can't synthesise a real multi-folder drag, so drive the
  // production intake twice — the per-folder grouping the refactored drop
  // handler now performs resolves to exactly this sequence of calls. Each call
  // must produce its own card with its own name and id.
  await uploadFolderViaComponent(page, 'FolderA', [{ rel: 'a1.txt', body: 'a1' }]);
  await uploadFolderViaComponent(page, 'FolderB', [{ rel: 'b1.txt', body: 'b1' }]);

  await expect(page.getByTestId('folder-item')).toHaveCount(2);
  await expect(page.getByTestId('folder-name')).toHaveText(['FolderA', 'FolderB']);
});

test('a folder upload whose relative path escapes the root is rejected by the server', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  // '../../escape.txt' under the top folder cleans to a path above the upload
  // root, which SanitizePath rejects (400). The UI surfaces it as an error.
  await uploadFolderViaComponent(page, 'evil', [
    { rel: '../../escape.txt', body: 'pwned' },
  ]);

  await expect(page.locator('[data-testid="folder-item"].state-error')).toBeVisible({ timeout: 5000 });
  // Nothing was written: the rejected part never reaches os.Create.
  expect(readdirSync(UPLOAD_DIR).length).toBe(0);
});

// =============================================================================
// Name collision (issue #16). Re-uploading an existing name must rename to
// "name (1).ext" rather than overwrite, and the Send card must reconcile to
// the actually-written name carried in the upload response — otherwise the
// displayed name and the delete/download/share actions would target the
// pre-existing file (silent data loss on delete).
// =============================================================================

test('re-uploading an existing name renames it and reconciles the card', async ({ page }) => {
  // Pre-existing file in the upload root — the collision target.
  writeFileSync(path.join(UPLOAD_DIR, 'dupe.txt'), 'original');

  await page.goto(`/?token=${TOKEN}`);
  await page.getByTestId('file-input').setInputFiles(fixture('dupe.txt', 'new'));

  // The card reconciles to the actually-written name (not the requested one).
  await expect(page.getByTestId('file-name')).toHaveText('dupe (1).txt');
  await expect(page.getByTestId('toasts')).toContainText('dupe (1).txt uploaded');

  // Both files survive on disk; the original is untouched (no overwrite).
  expect(readFileSync(path.join(UPLOAD_DIR, 'dupe.txt'), 'utf8')).toBe('original');
  expect(readFileSync(path.join(UPLOAD_DIR, 'dupe (1).txt'), 'utf8')).toBe('new');
});


