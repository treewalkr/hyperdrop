# HyperDrop — E2E tests

Browser end-to-end tests driving the **real** HyperDrop binary with Playwright.

**Toolchain:** Playwright is the driver, **Bun** is the package manager, **Node** is the runtime. The governing rule is **`bun` installs, `npx` runs** — the Playwright CLI must never be routed through Bun (it is unsupported/flaky). `scripts/check-toolchain.mjs` guards this and is wired into the `check` script.

## Install

```sh
cd e2e
bun install
npx playwright install chromium
```

## Run

```sh
npx playwright test
```

Playwright's `webServer` builds the Go binary (if `E2E_BIN` is unset), creates a fresh temp upload dir, and launches `hyperdrop --token e2e-token --port 8090 <tmpdir>` for the duration of the run. The base URL is `http://localhost:8090`.

### Skip the in-script build (CI / fast local loops)

Build the binary once and point the harness at it via `E2E_BIN` — the launch script then skips `go build`:

```sh
# from the repo root
go build -o ./bin/hyperdrop ./cmd/hyperdrop
cd e2e && E2E_BIN="$(pwd)/../bin/hyperdrop" npx playwright test
```

## Layout

- `tests/` — Playwright specs (one round-trip slice plus its building blocks).
- `fixtures/isolation.ts` — `wipeUploads()` helper; the spec resets the upload root in `beforeEach`.
- `scripts/launch.mjs` — the `webServer` command (binary resolution + spawn + teardown).
- `scripts/check-toolchain.mjs` — enforces the bun-installs / npx-runs rule.
- `playwright.config.ts` — project, `webServer`, and run options.
