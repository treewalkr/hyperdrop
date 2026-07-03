import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { TOKEN, UPLOAD_DIR, PORT } from '../lib/config.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// A real, tiny, H.264 mp4 committed to the repo (see fixtures/). Chromium can
// decode it, so the <video> element reaches HAVE_METADATA (readyState >= 1) —
// the round-trip assertion that proves playback actually works.
const SAMPLE_MP4 = path.resolve(__dirname, '..', 'fixtures', 'sample.mp4');

// Writes a fixture file into a unique temp subdir and returns its path; the
// basename is preserved so the page shows the intended filename.
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

test.beforeEach(() => wipeUploads());
test.afterAll(() => {
  for (const dir of fixtureDirs) rmSync(dir, { recursive: true, force: true });
});

// Upload via the real Send-page input, wait for it to land, then open Files.
async function uploadAndGoToFiles(page: import('@playwright/test').Page, filePath: string, name: string): Promise<void> {
  await page.goto(`/?token=${TOKEN}`);
  await page.getByTestId('file-input').setInputFiles(filePath);
  await expect(page.getByTestId('toasts')).toContainText(`${name} uploaded`);
  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);
}

// Resolves the row (file-item) for a given filename. Scoped so multi-file
// assertions don't go ambiguous.
const rowFor = (page: import('@playwright/test').Page, name: string) =>
  page.locator('[data-testid="file-item"]', { hasText: name });

test('Play button appears for a playable mp4 but not for an mkv', async ({ page }) => {
  await uploadAndGoToFiles(page, SAMPLE_MP4, 'sample.mp4');
  // A second upload of a .mkv (any bytes — categorize() tags it vid by ext, but
  // the browser can't decode it, so isPlayable() hides the Play action).
  await page.goto(`/?token=${TOKEN}`);
  await page.getByTestId('file-input').setInputFiles(fixture('clip.mkv', 'not real mkv'));
  await expect(page.getByTestId('toasts')).toContainText('clip.mkv uploaded');
  await page.getByTestId('nav-files').click();

  await expect(rowFor(page, 'sample.mp4').getByTestId('play')).toBeVisible();
  // x-show hides the action (display:none) but leaves it in the DOM, so assert
  // hidden rather than absent.
  await expect(rowFor(page, 'clip.mkv').getByTestId('play')).toBeHidden();
  // Download remains available for the unplayable container.
  await expect(rowFor(page, 'clip.mkv').getByTestId('download')).toBeVisible();
});

test('clicking Play opens the player and the video reaches HAVE_METADATA', async ({ page, context }) => {
  await uploadAndGoToFiles(page, SAMPLE_MP4, 'sample.mp4');

  // Play opens /player in a new tab (target="_player").
  const popupPromise = context.waitForEvent('page');
  await rowFor(page, 'sample.mp4').getByTestId('play').click();
  const player = await popupPromise;
  await player.waitForLoadState('domcontentloaded');

  await expect(player).toHaveURL(/\/player/);
  await expect(player.getByTestId('video')).toBeVisible();
  await expect(player.getByTestId('player-title')).toHaveText('sample.mp4');

  // The <video> fetched bytes from /api/stream/* and decoded enough to expose
  // metadata — the end-to-end proof that streaming + Range actually works.
  await player.waitForFunction(
    () => {
      const v = document.querySelector('video') as HTMLVideoElement | null;
      return !!v && v.readyState >= 1;
    },
    { timeout: 15000 },
  );

  // Download on the player still targets the attachment endpoint.
  await expect(player.locator('main').getByTestId('download')).toHaveAttribute('href', /\/api\/files\/sample\.mp4/);
});

test('opening /player for an unplayable file shows the download fallback', async ({ page }) => {
  // Seed an mkv directly on disk (no upload needed; the page only reads ?file=).
  writeFileSync(path.join(UPLOAD_DIR, 'clip.mkv'), 'not real mkv');

  await page.goto(`/player?file=clip.mkv&token=${TOKEN}`);

  await expect(page.getByTestId('player-fallback')).toBeVisible();
  await expect(page.getByTestId('video')).toBeHidden();
  await expect(page.getByTestId('player-fallback').getByTestId('download')).toHaveAttribute('href', /\/api\/files\/clip\.mkv/);
});

test('opening /player with no file shows the missing-file fallback', async ({ page }) => {
  await page.goto(`/player?token=${TOKEN}`);
  await expect(page.getByTestId('player-fallback')).toBeVisible();
  await expect(page.getByTestId('video')).toBeHidden();
});

// Guards the backend refactor: the stream route serves inline (no attachment
// header) while the download route still forces attachment. Uses the request
// API (not page.goto) because the attachment response triggers a download.
test('stream response is inline, download response is an attachment', async ({ request }) => {
  writeFileSync(path.join(UPLOAD_DIR, 'sample.mp4'), readFileSync(SAMPLE_MP4));
  const base = `http://localhost:${PORT}`;

  const stream = await request.get(`${base}/api/stream/sample.mp4?token=${TOKEN}`);
  expect(stream.status()).toBe(200);
  expect(stream.headers()['content-disposition']).toBeFalsy();

  const dl = await request.get(`${base}/api/files/sample.mp4?token=${TOKEN}`);
  expect(dl.status()).toBe(200);
  expect(dl.headers()['content-disposition']).toContain('attachment');
});
