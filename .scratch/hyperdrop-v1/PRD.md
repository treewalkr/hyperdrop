Status: ready-for-agent

# HyperDrop v1 — PRD

## Problem Statement

Moving files between devices on the same local network is a friction-heavy workflow. You either need a shared cloud service (upload, sync, download — slow and roundabout), a USB drive (physical, inconvenient), or a command-line tool like `scp`/`python -m http.server` (no UI, no progress feedback, no mobile support). There's no single tool that lets you open a browser on your phone or laptop, drag a file into a beautiful dropzone, and have it land instantly on the host machine — with a polished, real-time UI, zero setup, and no account required.

## Solution

HyperDrop is a single-binary CLI tool written in Go. You run `hyperdrop` in a directory, it starts an HTTP server on your LAN, prints a URL with an access token, and serves a polished glassmorphism web UI (the Aurora design system). Any device on the same network opens that URL, authenticates with the token, and can:

- **Upload** files via drag-and-drop or file picker (streaming multipart, no temp files)
- **Download** files from the host directory by clicking them
- **Delete** files through the web portal (with confirmation modal)
- **Navigate** subdirectories via breadcrumb navigation

All file operations are sandboxed to the root directory. A WebSocket connection pushes real-time file system events (file added, file deleted) to all connected browsers so everyone sees the same state without refreshing.

## User Stories

### CLI & Startup

1. As a user, I want to run `hyperdrop` in any directory to start serving files from that directory, so that I don't need to configure anything
2. As a user, I want `hyperdrop` to print a network URL (e.g. `http://192.168.1.42:8080/?token=a7x9k2m4`) on startup, so that I can share it with other devices
3. As a user, I want the token to be auto-generated as a random 8-character string, so that I get security by default without thinking about it
4. As a user, I want to specify a custom token via `--token mysecret`, so that I can use a memorable password for repeated sessions
5. As a user, I want to specify a custom directory via `hyperdrop /path/to/dir`, so that I don't have to `cd` first
6. As a user, I want the server to bind to `0.0.0.0` by default, so that other LAN devices can reach it
7. As a user, I want to restrict access to localhost only via `--host 127.0.0.1`, so that I can use it privately
8. As a user, I want to specify a custom port via `--port 3000`, so that I can avoid port conflicts
9. As a user, I want the server to use port 8080 by default, so that I don't have to remember to specify one
10. As a user, I want to set a max upload size via `--max-size 500MB`, so that I can prevent accidental large uploads
11. As a user, I want to see a clear error message if the specified directory doesn't exist or isn't readable, so that I know what went wrong

### Authentication & Security

12. As a LAN user, I want to authenticate by opening a URL that includes the token, so that I'm signed in immediately without typing anything
13. As a LAN user, I want the server to set a session cookie after I present a valid token, so that I don't have to re-authenticate on every request
14. As a LAN user, I want unauthenticated requests to be rejected, so that random devices on my network can't access my files without the token
15. As a user, I want all file operations to be sandboxed to the root directory, so that nobody can access files outside the shared directory via path traversal
16. As a user, I want paths containing `..` to be rejected, so that directory traversal attacks are prevented
17. As a user, I want absolute paths that escape the root to be rejected, so that symlinks or encoded paths can't bypass the sandbox

### Send Page (Upload)

18. As a sender, I want to see a drag-and-drop zone on the Send page, so that I can upload files by dragging them from my file manager
19. As a sender, I want to click the dropzone to open a file picker, so that I can browse and select files to upload
20. As a sender, I want to upload multiple files at once, so that I don't have to do it one at a time
21. As a sender, I want to see upload progress for each file (percentage, speed, bytes transferred), so that I know how long I'll wait
22. As a sender, I want to see a success indicator when a file finishes uploading, so that I know it landed on the host
23. As a sender, I want to see an error indicator if an upload fails, so that I can retry it
24. As a sender, I want to cancel an in-progress upload, so that I can abort a mistake
25. As a sender, I want the upload to stream directly to disk without buffering to a temp file first, so that the server doesn't run out of memory on large files
26. As a sender, I want uploads exceeding the max size limit to be rejected with a clear error, so that I know why my file was refused
27. As a sender, I want the Send page to show the Aurora glassmorphism design with animated aurora background, so that the experience feels polished and unique

### Files Page (Browse & Download)

28. As a browser, I want to see all files in the current directory on the Files page, so that I know what's available
29. As a browser, I want to switch between Table, Grid, and Feed view variants, so that I can pick the layout that suits me
30. As a browser, I want to sort files by name, date, or size, so that I can find what I need quickly
31. As a browser, I want to search files by name, extension, or source device, so that I can filter a large directory
32. As a browser, I want to see a file count and total size stat in the header, so that I understand the scope of files at a glance
33. As a browser, I want to click a file to download it, so that I can transfer it to my device
34. As a browser, I want to see file type badges (doc/img/vid/zip/generic) with the five-color Aurora language, so that I can visually distinguish file types
35. As a browser, I want to delete a file via a button with a confirmation modal, so that I don't accidentally remove something important
36. As a browser, I want to see an empty state message when there are no files, so that the page doesn't look broken
37. As a browser, I want to select multiple files with checkboxes, so that I can perform batch operations
38. As a browser, I want to select all files with a header checkbox, so that I can quickly select everything

### Directory Navigation

39. As a browser, I want to click on a folder to navigate into it, so that I can browse subdirectories
40. As a browser, I want to see a breadcrumb trail showing my current path, so that I always know where I am
41. As a browser, I want to click a breadcrumb segment to jump back to that directory, so that I can navigate up the tree quickly
42. As a browser, I want to upload files into the current subdirectory, so that I can organize files into folders

### Real-Time Updates

43. As a connected user, I want to see new files appear in real-time when someone else uploads, so that I don't have to refresh
44. As a connected user, I want to see files disappear in real-time when someone deletes them, so that my view stays current
45. As a connected user, I want the WebSocket to reconnect automatically if the connection drops, so that I don't lose real-time updates

### Cross-Device & Mobile

46. As a mobile user, I want the Send page to be fully usable on my phone screen, so that I can upload from my phone
47. As a mobile user, I want the Files page to adapt to a narrow screen, so that I can browse and download files on mobile
48. As a mobile user, I want touch-friendly action buttons that don't require hover, so that I can interact on a touchscreen

### Distribution & Installation

49. As a user, I want to download a single binary for my OS (macOS, Linux, Windows), so that I can run it without installing anything
50. As a macOS user, I want to install via Homebrew (`brew install hyperdrop`), so that it fits my existing workflow
51. As a user, I want the binary to be small (under 15MB), so that it downloads quickly

### Developer Experience

52. As a developer, I want a `--dev` flag that serves static assets from disk instead of `embed.FS`, so that I can iterate on the frontend without recompiling
53. As a developer, I want the project to build with a standard `go build`, so that there's no complex build pipeline
54. As a developer, I want goreleaser configured for automated releases, so that I can ship new versions across all platforms

## Implementation Decisions

### Technology Stack (per ADR 0001)

- **Backend**: Go with Chi HTTP router. Every handler is `func(w http.ResponseWriter, r *http.Request)` — stdlib-compatible, no framework lock-in.
- **Frontend**: Alpine.js for reactivity, added via `<script defer>`. No build step. HTML/CSS/JS embedded into the Go binary via `embed.FS`.
- **Distribution**: goreleaser for GitHub Releases and Homebrew tap.

### API Surface

The server exposes these HTTP endpoints:

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | Serve `index.html` (Send page) |
| `GET` | `/files` | Serve `files.html` (Files page) |
| `POST` | `/api/upload` | Multipart file upload (streaming to disk) |
| `GET` | `/api/files` | JSON listing of current directory contents |
| `GET` | `/api/files/{path}` | Download a file |
| `DELETE` | `/api/files/{path}` | Delete a file or folder |
| `GET` | `/api/ws` | WebSocket for real-time file system events |

### Authentication Flow

1. On startup, generate a random 8-character token (or use `--token` flag).
2. Incoming requests are checked for a session cookie first; if valid, proceed.
3. If no cookie, check `?token=` query parameter; if valid, set a session cookie and proceed.
4. If neither is valid, return 401. Static assets (`index.html`, `files.html`) are served without auth so the login page loads.

### Path Sandboxing

A `sanitizePath(rootDir, requestedPath) (string, error)` function resolves all paths:

- Uses `filepath.Abs()` and `filepath.Rel()` to resolve the real path.
- Compares the resolved path against the root directory prefix.
- Rejects paths containing `..`, absolute paths outside root, and any resolved path that doesn't start with root.
- This function is a standalone testable unit extracted from HTTP handlers.

### File Upload

- Uses `multipart.Reader` for streaming reads — no `multipart.ParseForm()` which buffers entirely in memory.
- Streams each file part directly to the target path using `io.Copy`.
- Respects `--max-size` by checking the `Content-Length` header before streaming starts.
- Reports upload progress via the WebSocket (bytes written so far / total).

### WebSocket Events

- The server maintains a list of connected WebSocket clients (a goroutine-safe subscriber list).
- File system events (`file_uploaded`, `file_deleted`) are broadcast to all connected clients as JSON messages.
- The WebSocket connection lives at `/api/ws` and requires the same authentication as other endpoints.
- Client-side Alpine.js listens for these events and updates reactive state.

### Frontend Architecture

- **Send page** (`index.html`): Contains three dropzone variants (Nebula, Orbit, Stream). In production, one variant is selected by default. Each dropzone handles drag-and-drop and click-to-browse. File cards show upload progress with pause/cancel actions.
- **Files page** (`files.html`): Three view variants (Table, Grid, Feed). Supports sorting (name, date, size), search, multi-select, breadcrumbs for directory navigation. File cards show download and delete actions.
- **Shared components**: Aurora animated background (4 CSS blobs), glass header with nav links, toast notification system, confirmation modal.
- The prototype variant switcher (floating pill at bottom) is removed in production; the default variant is used.
- Alpine.js manages reactive state: file lists, upload progress, search queries, sort state, WebSocket events.

### Design System (Aurora)

The Aurora design system from `DESIGN.md` is the visual language:

- **Canvas**: `#080b1a` midnight navy with four animated aurora blobs (18–25s cycles, 120px blur).
- **Glass surfaces**: `backdrop-filter: blur(20px)`, `rgba(255,255,255,0.04)` background, `rgba(255,255,255,0.08)` border.
- **Dual accent**: Aurora violet `#7c6cf0` + aurora teal `#4ecdc4` as 135° gradient.
- **File type colors**: doc (purple), img (teal), vid (coral), zip (amber), file (muted) — each with 18% background tint.
- **28 color tokens, 12 type tokens, 28 component definitions** as specified in DESIGN.md.

### CLI Flags

| Flag | Default | Purpose |
|------|---------|---------|
| (positional) | `.` (current directory) | Root directory to serve |
| `--token` | auto-generated 8 chars | Access token |
| `--host` | `0.0.0.0` | Bind address |
| `--port` | `8080` | Listen port |
| `--max-size` | unlimited | Max upload size (e.g. `500MB`) |
| `--dev` | false | Serve static assets from disk |

### Project Structure

```
cmd/hyperdrop/       # main() — CLI flag parsing, server bootstrap
internal/
  auth/               # Token validation, session cookie middleware
  handler/            # Chi route handlers (upload, download, delete, list, ws)
  sandbox/            # Path sanitization (sanitizePath function)
  watcher/            # File system event watcher + WebSocket broadcaster
  static/             # Embedded HTML/CSS/JS (embed.FS)
```

### Error Handling

- All API errors return JSON: `{"error": "descriptive message"}` with appropriate HTTP status codes.
- Upload errors (disk full, permission denied, max size exceeded) are reported via the WebSocket so the sender sees them in real-time.
- The frontend shows toast notifications for successes, errors, and informational events.

## Testing Decisions

### What Makes a Good Test

Tests verify external behavior, not implementation details. A good test:
- Hits the public interface (HTTP endpoint, CLI flag output, function signature) without reaching into internals.
- Asserts on observable outcomes (HTTP status codes, response bodies, file system state, WebSocket messages) rather than internal state.
- Is resilient to refactoring — renaming a handler function or restructuring internal packages doesn't break tests.

### Test Seams (in priority order)

1. **HTTP API seam** (integration tests via `net/http/httptest`)
   - Spin up the full Chi router in test mode.
   - Test every endpoint: auth flow, upload, download, delete, list, WebSocket.
   - Cover success paths, auth rejection, path traversal attempts, max-size enforcement.
   - This is the highest-level seam and covers the most ground.

2. **Path sandboxing seam** (unit tests)
   - `sanitizePath(root, requested) (string, error)` is a pure function.
   - Test: valid paths resolve correctly, `..` traversal is rejected, absolute paths outside root are rejected, symlinks are handled.
   - This is the security boundary — deserves exhaustive standalone testing.

3. **WebSocket seam** (integration tests)
   - Connect a test WebSocket client, trigger file operations, verify broadcast messages arrive.
   - Test: multiple clients receive events, disconnect/reconnect behavior.

4. **CLI seam** (unit/integration tests)
   - Test flag parsing with different argument combinations.
   - Test startup output (network URL format, token presence).
   - Refactor to accept `io.Writer` so output can be captured in tests.

### Prior Art

No existing tests in the codebase (greenfield project). The Go standard library patterns (`httptest.NewServer`, `testing.T`) are the baseline. Chi's stdlib compatibility means no custom test utilities are needed.

## Out of Scope

- **User accounts / multi-user permissions** — single token grants full access.
- **File preview / thumbnail generation** — files are shown by name and metadata only.
- **Resumable uploads** — uploads are single-shot streams; no chunked resume protocol.
- **Upload pause/resume on the server** — the prototype shows a pause button, but v1 uploads stream to completion; pause cancels the upload.
- **Folder upload from the browser** — only individual files (multiple files yes, folder structure no).
- **Encrypted transfers** — all traffic is plaintext HTTP over LAN.
- **QR code for URL sharing** — the URL is printed to terminal only.
- **Internationalization** — English UI only.
- **Access logging to file** — logs go to stdout only.
- **Rate limiting** — no request throttling in v1.
- **Custom theme selection** — Aurora is the only theme in production; other prototype variants (Paper, Neon, Gradient) remain in the prototype gallery only.
- **`prefers-reduced-motion` handling** — aurora blob animations run unconditionally in v1.

## Further Notes

- The prototype gallery at `prototype/` contains five visual directions. Aurora was chosen as the production design. The others (Paper, Neon, Gradient, v0) are retained for reference but won't be built into the binary.
- The prototype variant switcher (floating pill with arrow buttons) is a design exploration tool. In production, each page uses a single default variant: Nebula dropzone for Send, Table view for Files (users can still switch view variants on the Files page).
- The `--dev` flag is critical for developer velocity — it serves HTML/CSS/JS from disk so frontend changes are visible on browser refresh without recompiling Go.
- Future versions may add: folder upload, QR code sharing, file preview thumbnails, upload pause/resume, and a TUI dashboard showing connected clients.
