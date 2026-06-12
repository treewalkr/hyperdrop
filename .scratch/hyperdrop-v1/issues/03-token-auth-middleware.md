Status: done

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Chi middleware for token-based authentication. Every API request (`/api/*`) is checked for a valid session cookie or a `?token=` query parameter.

Auth flow:
1. Check for a session cookie. If present and valid, proceed.
2. If no valid cookie, check `?token=` query parameter. If it matches the server's token, set a session cookie (`HttpOnly`, `SameSite=Lax`) and proceed.
3. If neither is valid, return 401 with JSON `{"error": "unauthorized"}`.

Static assets (`/`, `/files`, CSS/JS) are served without auth so the login page loads. The middleware wraps only the `/api/*` route group.

The token is stored in the server struct and shared between the CLI flag parser and the middleware.

## Acceptance criteria

- [ ] `GET /api/files?token=valid` returns 200 and sets a session cookie
- [ ] `GET /api/files` with valid session cookie returns 200
- [ ] `GET /api/files` with no cookie and no token returns 401 JSON
- [ ] `GET /api/files?token=wrong` returns 401 JSON
- [ ] Static assets (`/`, `/files`) are served without authentication
- [ ] Session cookie is `HttpOnly` and `SameSite=Lax`
- [ ] Tests use httptest to verify the full auth flow

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
