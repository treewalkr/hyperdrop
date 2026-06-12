# ADR 0001: Go single binary with Chi router and Alpine.js frontend

**Status**: Accepted

**Date**: 2026-06-12

## Context

HyperDrop is a personal CLI tool that starts an HTTP server for local file transfer and management over LAN. It needs to be easy to install (single binary), serve a polished web UI (the Aurora glassmorphism design system), and handle file upload/download with real-time updates across browsers.

We need to decide on the backend language/framework, the frontend approach, and how the two are packaged together.

## Decision

We will use:

- **Go** as the backend language, compiled to a single static binary
- **Chi** as the HTTP router (stdlib-compatible, lightweight)
- **Alpine.js** for frontend reactivity (drop-in, no build step)
- **`embed.FS`** to embed the HTML/CSS/JS into the Go binary at compile time
- **`goreleaser`** for automated binary distribution via GitHub Releases and Homebrew tap

## Rationale

### Why Go over Rust, Node.js, Python, Deno

- **Single binary with zero runtime deps**: Go compiles to a statically-linked binary. Users download one file and run it. No Node, Python, or runtime to install.
- **Excellent stdlib for HTTP and file I/O**: `net/http`, `os`, `path/filepath` cover everything needed. No heavy framework required.
- **Cross-compilation is trivial**: `GOOS=linux GOARCH=arm64 go build` produces a binary for a Raspberry Pi from a Mac. No toolchain setup.
- **Small binary size**: ~8-12MB with embedded assets. Deno compiled binaries are 80MB+. Rust would be smaller but with much higher development complexity for a simple HTTP server.
- **Personal tool = dev velocity matters**: Go is straightforward to read and write. Error handling is explicit. The project scope (5 endpoints, file operations) doesn't need Rust's safety guarantees or Node's npm ecosystem.

### Why Chi over Gin/Echo/stdlib-only

- **5 endpoints don't need a framework**: But they do need middleware (token auth, logging). Chi provides a clean middleware chain while staying 100% compatible with `net/http`.
- **No lock-in**: Every Chi handler is `func(w http.ResponseWriter, r *http.Request)`. No custom context type. If we ever drop Chi, zero refactoring needed.
- **Gin and Echo are overkill**: Their value (request binding, validation, custom context) doesn't justify the dependency for 5 simple endpoints with no structs to bind.

### Why Alpine.js over React/Vue/vanilla JS

- **No build step**: The existing prototype is vanilla HTML/CSS/JS. Alpine.js is added with a single `<script defer>` tag. No Vite, no webpack, no JSX compilation.
- **Embeddable as-is**: The HTML files go into Go's `embed.FS` directly. React/Vue with SFCs would require a build pipeline that outputs to a directory, adding complexity.
- **Reactive state for growing UI**: The prototype is ~1400 lines of manual DOM manipulation per page. Adding breadcrumb navigation, WebSocket updates, and real upload progress tracking in vanilla JS would create tangled imperative code. Alpine's `x-data`/`x-for`/`x-on` model handles this cleanly.
- **Tiny footprint**: ~15KB gzipped. No impact on the binary size.

### Why `embed.FS` over external static files

- **Single binary distribution**: The user never sees or needs the HTML files. Everything is inside the binary.
- **No file path issues**: No "where do I put the static files relative to the binary?" problem.
- **The prototype is already self-contained**: Each page is a single HTML file with inline CSS and JS. Perfect for embedding.

## Consequences

### Positive

- One `go install` or binary download gives a working tool. No setup.
- The frontend stays close to the existing prototype — incremental adoption of Alpine.js, not a rewrite.
- Go's concurrency model (goroutines) handles WebSocket broadcasting and streaming uploads naturally.
- `goreleaser` automates releases across macOS, Linux, and Windows.

### Negative

- Frontend changes require recompiling the Go binary (no hot-reload of embedded assets). Mitigated by serving from disk during development with a `--dev` flag.
- Alpine.js is sufficient for this scope but doesn't scale to complex component hierarchies. If the UI grows significantly, a migration to Preact or Vue would be needed.
- No SSR — all rendering happens client-side. Search engines don't index it (irrelevant for a LAN tool).

### Risks

- **Alpine.js abandonment**: It's maintained by a small team (Caleb Porzio). If it stops being maintained, the amount of Alpine-specific code is small enough to refactor to vanilla JS or migrate to another library.
- **Binary size growth**: As more assets are embedded (icons, additional pages), the binary grows. At the current scope (~3000 lines of HTML/CSS/JS), this is negligible (~50-100KB).
