Status: done

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

The `hyperdrop` command that bootstraps the entire application. Running `hyperdrop [dir]` parses CLI flags (`--token`, `--host`, `--port`, `--dev`), resolves the root directory (positional arg or `.` by default), generates an auto-token (random 8 chars unless `--token` is specified), starts a Chi HTTP server, and prints the network URL to stdout (e.g. `http://192.168.1.42:8080/?token=a7x9k2m4`).

The server serves placeholder `index.html` and `files.html` via `embed.FS`. When `--dev` is passed, static assets are served from disk instead (enabling live frontend iteration without recompiling). The project compiles with a standard `go build ./cmd/hyperdrop`.

If the specified directory doesn't exist or isn't readable, print a clear error and exit with code 1.

Default values: host `0.0.0.0`, port `8080`, root directory `.`, token auto-generated.

Project structure follows the layout from the PRD: `cmd/hyperdrop/` for main, `internal/static/` for embedded assets.

## Acceptance criteria

- [ ] `hyperdrop` starts a server that responds to `GET /` with an HTML page
- [ ] `hyperdrop /some/path` serves files from that directory as root
- [ ] `--token mysecret` uses that token instead of auto-generating one
- [ ] `--host 127.0.0.1` binds to localhost only
- [ ] `--port 3000` listens on that port
- [ ] `--dev` serves HTML from disk instead of embedded FS
- [ ] Startup output includes the full network URL with token
- [ ] Non-existent directory argument prints an error and exits with code 1
- [ ] `go build ./cmd/hyperdrop` produces a single binary with no errors
- [ ] Tests cover CLI flag parsing and server startup via httptest

## Blocked by

None — can start immediately
