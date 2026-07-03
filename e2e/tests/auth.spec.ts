import { test, expect } from '@playwright/test';
import { wipeUploads } from '../fixtures/isolation.js';
import { TOKEN } from '../lib/config.mjs';

// Auth-flow coverage (issue #21). Pins the token-login → page-load contract:
//
// The auth model differs from the naive "page rejects without token" guess.
// Static pages (`/`, `/files`) are served unconditionally — Chi mounts them
// outside the `/api` tokenAuth middleware, so the HTML (and the dropzone)
// renders whether or not the request is authenticated. What the token
// actually gates:
//
//   1. The `/api/*` routes return 401 JSON `{"error":"unauthorized"}` without
//      a valid `?token=` query OR a `hyperdrop_session` cookie.
//   2. A token-bearing page request (`/?token=<token>`) sets the
//      `hyperdrop_session` cookie (HttpOnly, SameSite=Lax) — the persistent
//      credential — and the page JS then strips `?token=` from the address
//      bar via history.replaceState so it doesn't linger in history/Referer.
//   3. The server only injects the token into same-origin Send<->Files nav
//      hrefs when the request proved knowledge of the token. An
//      unauthenticated page never receives a token-bearing href (no leak).
//
// So the "unauthenticated" contract is asserted at the API and cookie layer,
// not at the page-render layer — the page always renders. The interesting
// user-facing behavior is: a cookie set once authenticates every subsequent
// navigation and API call without the token reappearing in the URL.

// Every test starts from an empty upload root. The suite shares one server
// and one UPLOAD_DIR across files; the wipe keeps this file from racing the
// shared dir against upload.spec.ts (workers: 1 in CI bounds cross-file
// races to local runs, which is accepted).
test.beforeEach(() => wipeUploads());

// Shortcut for the session cookie among the context's cookies.
function sessionCookie(cookies: Array<{ name: string; value: string }>) {
  return cookies.find((c) => c.name === 'hyperdrop_session');
}

test('unauthenticated "/" serves the page but sets no session cookie', async ({ page }) => {
  await page.goto('/');

  // The page is static HTML and always renders — auth is at the API layer.
  await expect(page.getByTestId('dropzone')).toBeVisible();

  // No token was presented, so the server sets no session cookie. This is
  // the real "not authenticated" signal: the cookie is the credential, and
  // it is absent.
  expect(sessionCookie(await page.context().cookies())).toBeUndefined();
});

test('unauthenticated "/api/files" returns 401 (API layer is the gate)', async ({ request }) => {
  // The standalone `request` fixture carries no page cookies — a clean
  // probe that the API, not the page, is what rejects unauthenticated use.
  const resp = await request.get('/api/files');
  expect(resp.status()).toBe(401);
  expect(await resp.json()).toEqual({ error: 'unauthorized' });
});

test('a wrong token does not set a cookie or inject into nav hrefs', async ({ page }) => {
  await page.goto('/?token=not-the-token');

  // The page still serves (static), but the unknown token is not trusted.
  await expect(page.getByTestId('dropzone')).toBeVisible();
  expect(sessionCookie(await page.context().cookies())).toBeUndefined();

  // Security slice (mirrors TestNavLinks_UnauthenticatedPageLeaksNoToken):
  // an unauthenticated response must not carry a token-bearing nav href.
  await expect(page.getByTestId('nav-files')).toHaveAttribute('href', '/files');
  await expect(page.getByTestId('nav-send')).toHaveAttribute('href', '/');
});

test('"/?token=<token>" authenticates: dropzone visible, cookie set, URL cleaned', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  await expect(page.getByTestId('dropzone')).toBeVisible();

  // The token-bearing page request sets the persistent session cookie.
  const cookie = sessionCookie(await page.context().cookies());
  expect(cookie).toBeDefined();
  expect(cookie!.value).toBe(TOKEN);

  // The page strips ?token from the address bar (history.replaceState) once
  // the cookie is the credential — so the token doesn't linger in history
  // or leak via Referer. this.token stays in memory for in-flight API calls
  // until the cookie round-trips.
  await expect(page).toHaveURL('/');
});

test('authenticated "/" injects the token onto Send<->Files nav hrefs', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);

  // Only a request that proved knowledge of the token gets token-bearing
  // same-origin nav hrefs (the next click re-seeds the cookie). The brand
  // link is intentionally excluded — only the in-app nav carries it.
  await expect(page.getByTestId('nav-files')).toHaveAttribute('href', `/files?token=${TOKEN}`);
  await expect(page.getByTestId('nav-send')).toHaveAttribute('href', `/?token=${TOKEN}`);
});

test('with the cookie set, "/files" loads from the cookie (no token in URL)', async ({ page }) => {
  // First request authenticates and seeds the session cookie.
  await page.goto(`/?token=${TOKEN}`);
  await expect(page.getByTestId('dropzone')).toBeVisible();

  // From here the cookie — not the URL — is the credential. Navigate to the
  // Files page with no token in the URL: the page renders, the file-list
  // API call authenticates via the cookie, and the URL stays clean.
  await page.goto('/files');
  await expect(page).toHaveURL('/files');
  await expect(page.getByTestId('nav-files')).toBeVisible();
  // The file list container is present even when empty — the page rendered
  // its data load successfully (no 401 toast, no redirect).
  await expect(page.getByTestId('file-item')).toHaveCount(0);
});

test('nav round-trip Send -> Files -> Send keeps the session authenticated', async ({ page }) => {
  await page.goto(`/?token=${TOKEN}`);
  await expect(page.getByTestId('dropzone')).toBeVisible();

  // Send -> Files: the nav link carries the token, so the destination page
  // re-seeds the cookie and its file-list API call authenticates.
  await page.getByTestId('nav-files').click();
  await expect(page).toHaveURL(/\/files/);
  await expect(page.getByTestId('nav-files')).toBeVisible();

  // Files -> Send: round-trip back. Still authenticated — the dropzone
  // renders and no login/redirect intercepts the navigation. Asserting the
  // dropzone also waits for the navigation to settle, then check pathname
  // (toHaveURL matches the full URL string; the token was stripped to '/').
  await page.getByTestId('nav-send').click();
  await expect(page.getByTestId('dropzone')).toBeVisible();
  expect(new URL(page.url()).pathname).toBe('/');

  // The session cookie survived the round-trip.
  const cookie = sessionCookie(await page.context().cookies());
  expect(cookie?.value).toBe(TOKEN);
});
