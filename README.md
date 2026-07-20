# HyperDrop

[![CI](https://github.com/treewalkr/hyperdrop/actions/workflows/ci.yml/badge.svg)](https://github.com/treewalkr/hyperdrop/actions/workflows/ci.yml)
[![Release](https://github.com/treewalkr/hyperdrop/actions/workflows/release.yml/badge.svg)](https://github.com/treewalkr/hyperdrop/actions/workflows/release.yml)
[![GitHub release](https://img.shields.io/github/v/release/treewalkr/hyperdrop)](https://github.com/treewalkr/hyperdrop/releases)

Move files between devices on your local network from a browser. Run `hyperdrop`
in a directory, open the printed URL on your phone or laptop, and drag files
into a polished web UI — no cloud, no USB drive, no accounts.

HyperDrop is a single static binary written in Go. It starts an HTTP server on
your LAN, prints a URL with an access token, and serves a glassmorphism web UI
(the Aurora design system). Any device on the same network authenticates with
the token and can upload, download, delete, and browse files. A WebSocket keeps
every connected browser in sync in real time.

## Features

- **Upload** via drag-and-drop or file picker — streaming straight to disk, no temp files
- **Download** any file from the shared directory
- **Delete** files with a confirmation prompt
- **Browse** subdirectories with breadcrumb navigation
- **Real-time** updates: every browser sees new and deleted files instantly over WebSocket
- **Secure by default**: random access token, session cookie, all paths sandboxed to the root
- **Single binary**, ~6 MB, no runtime dependencies

## Install

**From source** (any platform with Go 1.24+):

```bash
go install github.com/treewalkr/hyperdrop/cmd/hyperdrop@latest
```

`go install` places the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`).
If `hyperdrop: command not found`, add that directory to your `PATH`:

```bash
# add to ~/.bashrc (Linux) or ~/.zshrc / ~/.bash_profile (macOS)
export PATH="$PATH:$(go env GOPATH)/bin"
```

Then reload your shell (`source ~/.bashrc`) or open a new terminal, and verify
with `which hyperdrop`.

**Pre-built binary** from the [releases page](https://github.com/treewalkr/hyperdrop/releases):

```bash
# macOS (Apple Silicon)
curl -sLO https://github.com/treewalkr/hyperdrop/releases/latest/download/hyperdrop_0.2.0_macos_arm64.tar.gz
tar xzf hyperdrop_0.2.0_macos_arm64.tar.gz
```

Assets are published for macOS, Linux (amd64 + arm64), and Windows, with a
`checksums.txt` for verification.

## Quick start

```bash
$ hyperdrop
http://192.168.1.42:8080/?token=a7x9k2m4p8q1w5r3
```

Open that URL on any device on the same Wi-Fi. The token is auto-generated on
each run; the server sets a session cookie so you stay signed in.

## CLI flags

| Flag | Default | Purpose |
|------|---------|---------|
| `[directory]` | `.` | Root directory to serve |
| `--token` | auto-generated 16 chars | Access token |
| `--host` | `0.0.0.0` | Bind address (use `127.0.0.1` for localhost-only) |
| `--port` | `8080` | Listen port |
| `--max-size` | unlimited | Max upload size, e.g. `500MB`, `2GB` |
| `--dev` | off | Serve static assets from disk (frontend iteration) |
| `--version` | — | Print version and exit |

## How it works

- **Auth flow:** requests are checked for a valid session cookie, then a `?token=`
  query param (which sets the cookie). Anything else returns `401`. The login page
  loads without auth.
- **Path sandboxing:** every requested path is resolved and checked against the root
  directory — `..` traversal, absolute escapes, and symlink breaks are all rejected.
- **Streaming uploads:** multipart parts are streamed to disk with `io.Copy`, never
  buffered in memory. `--max-size` is enforced before streaming starts.
- **Real-time:** file add/delete events are broadcast to all WebSocket clients as JSON.

### API surface

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | Send page (upload) |
| `GET` | `/files` | Files page (browse & download) |
| `GET` | `/api/files` | JSON directory listing (`?path=` for subdirectories) |
| `POST` | `/api/upload` | Streaming multipart upload |
| `GET` | `/api/files/{path}` | Download a file |
| `DELETE` | `/api/files/{path}` | Delete a file or folder |
| `GET` | `/api/ws` | WebSocket for real-time events |

## Development

Requires Go 1.24+ and [`just`](https://github.com/casey/just).

```bash
just build       # build ./bin/hyperdrop (with version ldflags)
just test        # go test ./...
just test-race   # with the race detector
just check       # fmt-check + vet + test
just run         # go run ./cmd/hyperdrop
```

Frontend lives in `internal/static/` and is embedded into the binary with
`embed.FS`. Use `--dev` to serve it from disk so HTML/CSS/JS changes show on
browser refresh without recompiling:

```bash
just run -- --dev
```

## Project structure

```
cmd/hyperdrop/   main() — CLI parsing, server bootstrap
internal/
  cli/            flag parsing, config, human-readable sizes
  server/         Chi routes: upload, download, delete, list, WebSocket, auth
  sandbox/        path sanitization (the security boundary)
  static/         embedded HTML/CSS/JS (Aurora UI)
  version/        build-time + build-info version metadata
```

## Releasing

Releases are automated with [goreleaser](https://goreleaser.com). Push a `v*` tag
and the `release` workflow builds all targets and publishes a GitHub Release:

```bash
git tag v0.2.0
git push origin v0.2.0
```

## Tech stack

- **Backend:** Go + [Chi](https://github.com/go-chi/chi) router, stdlib-compatible handlers
- **Frontend:** [Alpine.js](https://alpinejs.dev) for reactivity, embedded via `embed.FS` — no build step
- **Distribution:** goreleaser (GitHub Releases) + `go install`
