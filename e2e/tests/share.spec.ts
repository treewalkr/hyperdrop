import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { cleanupFixtureDirs } from '../fixtures/files.js';
import { TOKEN, UPLOAD_DIR, PORT } from '../lib/config.mjs';

// Share-link flow (PR #50) — owner creates a scoped per-file link, recipient
// opens /s/<token> on a cookie-free context, scope/revoke boundary enforced.
// Mirrors the established suite shape: one shared server + upload root,
// workers:1 in the config, beforeEach wipeUploads() plus a clearShares(page)
// (shares live in process-memory and outlive wipeUploads).
//
// Out of scope:
//   - Real-time TTL expiry: no clock-injection API and the store is
//     process-lifetime, so revoke (test 7) is the deterministic revocation
//     path under test.
//   - Owner ?token= byte-route regression: already covered by
//     TestShare_OwnerTokenStillStreams in share_test.go; not duplicated here.
//
// Recipient isolation: every landing navigation uses a fresh
// browser.newContext() page (no owner cookie); every cookie-free HTTP probe
// uses the bare `request` fixture (no cookie jar). page.request is used only
// for owner-authenticated calls (it shares the page context's session cookie).

// Every test starts from an empty upload root AND an empty share set.
test.beforeEach(async ({ page }) => {
  wipeUploads();
  await clearShares(page);
});
test.afterAll(() => cleanupFixtureDirs());

// ---- helpers ----

// Resolves the row (file-item) for a given filename. Mirrors player.spec.ts.
const rowFor = (page: import('@playwright/test').Page, name: string) =>
  page.locator('[data-testid="file-item"]', { hasText: name });

// getByTestId scoped to *visible* matches only. Required because share-title
// and share-download each render in two branches (playable + non-playable:
// share.html ~L114/137 and ~L116/139); only one is visible at a time, and an
// unscoped getByTestId resolves 2 elements → strict-mode violation.
const visible = (page: import('@playwright/test').Page, testid: string) =>
  page.getByTestId(testid).filter({ visible: true });

// Create a share as the owner via the API. Requires a prior
// page.goto(`/?token=${TOKEN}`) so the session cookie is set (page.request
// shares that cookie). POST /api/shares returns 200 (not 201) with
// { token, url, path, created_at }; expires_at is omitted entirely for
// `never` (*time.Time nil + omitempty, not null).
async function createShareViaApi(
  page: import('@playwright/test').Page,
  rel: string,
  ttl: string,
): Promise<{ token: string; url: string; path: string; created_at: string; expires_at?: string }> {
  const resp = await page.request.post('/api/shares', { data: { path: rel, ttl } });
  expect(resp.status()).toBe(200);
  return (await resp.json()) as any;
}

// Delete every live share so the next test starts clean. Shares are
// process-lifetime in-memory and outlive wipeUploads(). GET /api/shares
// returns a bare JSON array (not { shares: [...] }); DELETE /api/shares/<token>
// is idempotent.
async function clearShares(page: import('@playwright/test').Page): Promise<void> {
  // The owner cookie is set on the first token-bearing navigation; without it
  // GET /api/shares would 401 and leftover shares from a prior test would
  // leak. Seed the cookie before listing.
  await page.goto(`/?token=${TOKEN}`);
  const resp = await page.request.get('/api/shares');
  if (!resp.ok()) return;
  const shares = (await resp.json()) as Array<{ token: string }>;
  for (const s of shares) {
    await page.request.delete('/api/shares/' + encodeURIComponent(s.token));
  }
}

// Pull `<token>` out of a /s/<token> URL.
function tokenFromUrl(url: string): string {
  const m = url.match(/\/s\/([^/?#]+)/);
  return m ? m[1] : '';
}

// ---- tests ----

// 1. Owner creates a share via the Files modal — exercises the real UI flow:
// modal open, TTL pill select, Create button, auto-copy toast, rendered link
// row.
test('owner creates a share via the Files modal', async ({ page }) => {
  // Seed a playable mp4 directly on disk (the modal only needs the row).
  writeFileSync(path.join(UPLOAD_DIR, 'movie.mp4'), 'fake mp4 body');

  await page.goto(`/files?token=${TOKEN}`);
  await expect(rowFor(page, 'movie.mp4')).toBeVisible();

  // The Share action is hover-revealed; force-click skips that gate.
  await rowFor(page, 'movie.mp4').getByTestId('share').click({ force: true });
  await expect(page.getByTestId('share-modal')).toBeVisible();

  // Pick never-expire, then create. createShare() also fires an earlier
  // "link copied" toast via copyShareUrl; assert the terminal success toast
  // by visible text, NOT by toast count (two toasts briefly coexist).
  await page.getByTestId('ttl-never').click();
  await page.getByTestId('share-create').click();

  await expect(page.getByTestId('toasts')).toContainText('link created and copied');

  // A link row rendered with a non-empty URL. The token is recoverable from
  // the /s/<token> suffix on that URL. Wait on Alpine's x-text (which is
  // reactive, not present on first paint) before reading textContent.
  const linkRow = page.getByTestId('share-link-row');
  const urlEl = linkRow.getByTestId('share-link-url');
  await expect(linkRow).toBeVisible();
  await expect(urlEl).not.toHaveText('');
  const url = (await urlEl.textContent()) || '';
  expect(tokenFromUrl(url)).not.toBe('');
});

// 2. Recipient landing renders (playable branch). Seed via API, open /s/<token>
// in a fresh cookie-free context, wait on Alpine's :src/:href bindings.
test('recipient landing renders the player for a playable file', async ({ page, browser }) => {
  writeFileSync(path.join(UPLOAD_DIR, 'movie.mp4'), 'fake mp4 body');
  await page.goto(`/?token=${TOKEN}`);
  const { url } = await createShareViaApi(page, 'movie.mp4', 'never');
  const token = tokenFromUrl(url);

  // Fresh context: no owner cookie, no leaked session.
  const ctx = await browser.newContext();
  const recipient = await ctx.newPage();
  await recipient.goto(`/s/${token}`);

  // Alpine binds :src/:href after init; wait on the bound attribute, not
  // DOMContentLoaded.
  const downloadEl = visible(recipient, 'share-download');
  await expect(downloadEl).toHaveAttribute('href', /\/api\/files\/.+\?s=/);

  await expect(visible(recipient, 'share-title')).toHaveText('movie.mp4');
  // share-video lives only in the playable branch — single element, safe to
  // scope directly.
  await expect(recipient.getByTestId('share-video')).toHaveAttribute('src', /\/api\/stream\/movie\.mp4\?s=/);
  await expect(downloadEl).toHaveAttribute('href', /\/api\/files\/movie\.mp4\?s=/);

  await ctx.close();
});

// 3. Recipient landing renders (non-playable branch). share-video is hidden
// via x-show (display:none) but still in the DOM — assert toBeHidden, NOT
// toHaveCount(0).
test('recipient landing hides the player for a non-playable file', async ({ page, browser }) => {
  // .zip is not a natively-decodable video container → non-playable branch.
  writeFileSync(path.join(UPLOAD_DIR, 'archive.zip'), 'zip body');
  await page.goto(`/?token=${TOKEN}`);
  const { url } = await createShareViaApi(page, 'archive.zip', 'never');
  const token = tokenFromUrl(url);

  const ctx = await browser.newContext();
  const recipient = await ctx.newPage();
  await recipient.goto(`/s/${token}`);

  const downloadEl = visible(recipient, 'share-download');
  await expect(downloadEl).toHaveAttribute('href', /\/api\/files\/.+\?s=/);

  await expect(recipient.getByTestId('share-video')).toBeHidden();
  await expect(visible(recipient, 'share-title')).toHaveText('archive.zip');
  await expect(downloadEl).toHaveAttribute('href', /\?s=/);

  await ctx.close();
});

// 4. Recipient streams/downloads bytes against the real launched binary. New
// pattern for this suite: assert the byte body equals the seeded bytes. The
// owner cookie is set on the page fixture; recipient byte probes go through
// the bare `request` fixture (no cookie jar), exactly like player.spec.ts's
// inline/attachment test.
test('recipient stream and download serve the seeded bytes', async ({ page, request }) => {
  const bytes = Buffer.from('deterministic-mp4-bytes-0123456789');
  writeFileSync(path.join(UPLOAD_DIR, 'movie.mp4'), bytes);

  await page.goto(`/?token=${TOKEN}`);
  const { token } = await createShareViaApi(page, 'movie.mp4', 'never');

  const base = `http://localhost:${PORT}`;
  // Stream is inline and serves the exact bytes (no Range here — full body).
  const stream = await request.get(`${base}/api/stream/movie.mp4?s=${token}`);
  expect(stream.status()).toBe(200);
  expect(Buffer.from(await stream.body())).toEqual(bytes);

  // Download forces attachment (Content-Disposition: attachment).
  const dl = await request.get(`${base}/api/files/movie.mp4?s=${token}`);
  expect(dl.status()).toBe(200);
  expect(dl.headers()['content-disposition']).toContain('attachment');
});

// 5. Scope enforcement: a token bound to a.mp4 must not unlock b.mp4 or a
// path-traversal attempt. Both go through fileAccessAuth and must 401.
test('a share token is scoped to its bound file', async ({ page, request }) => {
  writeFileSync(path.join(UPLOAD_DIR, 'a.mp4'), 'a body');
  writeFileSync(path.join(UPLOAD_DIR, 'b.mp4'), 'b body');
  await page.goto(`/?token=${TOKEN}`);
  const { token } = await createShareViaApi(page, 'a.mp4', 'never');

  const base = `http://localhost:${PORT}`;
  const b = await request.get(`${base}/api/stream/b.mp4?s=${token}`);
  expect(b.status()).toBe(401);

  // Encoded traversal: the recipient side cannot rewrite the binding.
  const traversal = encodeURIComponent('../../etc/passwd');
  const trav = await request.get(`${base}/api/stream/${traversal}?s=${token}`);
  expect(trav.status()).toBe(401);
});

// 6. No-escalation: a share token must not unlock owner endpoints. The Go
// suite covers GET /api/files and DELETE /api/files/{path}; the other three
// (POST /api/upload, GET /api/shares, DELETE /api/shares/{token}) are net-
// new regression value here.
test('a share token does not grant owner endpoints', async ({ page, request }) => {
  writeFileSync(path.join(UPLOAD_DIR, 'a.mp4'), 'a body');
  await page.goto(`/?token=${TOKEN}`);
  const { token } = await createShareViaApi(page, 'a.mp4', 'never');

  const base = `http://localhost:${PORT}`;
  const s = `?s=${token}`;

  expect((await request.get(`${base}/api/files${s}`)).status()).toBe(401);
  expect(
    (
      await request.post(`${base}/api/upload${s}`, {
        multipart: { file: { name: 'a.mp4', mimeType: 'application/octet-stream', buffer: Buffer.from('x') } },
      })
    ).status(),
  ).toBe(401);
  expect((await request.delete(`${base}/api/files/a.mp4${s}`)).status()).toBe(401);
  expect((await request.get(`${base}/api/shares${s}`)).status()).toBe(401);
  expect((await request.delete(`${base}/api/shares/${token}${s}`)).status()).toBe(401);
});

// 7. Revocation from the /shares management page kills the link end-to-end:
// the row goes away, the recipient landing 404s, the byte routes 401.
test('revoking from /shares kills the link for recipients', async ({ page, request, browser }) => {
  writeFileSync(path.join(UPLOAD_DIR, 'a.mp4'), 'a body');
  await page.goto(`/?token=${TOKEN}`);
  const { token } = await createShareViaApi(page, 'a.mp4', 'never');

  await page.goto(`/shares?token=${TOKEN}`);

  // Scope to the row matching this file so revoke is unambiguous.
  const row = page.getByTestId('share-row').filter({ hasText: 'a.mp4' });
  await expect(row).toBeVisible();
  await row.getByTestId('revoke').click();
  await expect(row).toHaveCount(0);

  // Recipient (fresh context) sees the dead-link page on /s/<token>.
  const ctx = await browser.newContext();
  const recipient = await ctx.newPage();
  const resp = await recipient.goto(`/s/${token}`);
  expect(resp?.status()).toBe(404);
  await expect(recipient.getByText('no longer available')).toBeVisible();
  await ctx.close();

  // The byte route also rejects the now-revoked token.
  const base = `http://localhost:${PORT}`;
  const stream = await request.get(`${base}/api/stream/a.mp4?s=${token}`);
  expect(stream.status()).toBe(401);
});

// 8. Dead-link page: an unknown token renders the "no longer available" 404
// page directly (no share lookup can resolve).
test('a bogus token renders the dead-link page', async ({ browser }) => {
  const ctx = await browser.newContext();
  const recipient = await ctx.newPage();
  const resp = await recipient.goto('/s/definitely-bogus');
  expect(resp?.status()).toBe(404);
  await expect(recipient.getByText('no longer available')).toBeVisible();
  await ctx.close();
});
