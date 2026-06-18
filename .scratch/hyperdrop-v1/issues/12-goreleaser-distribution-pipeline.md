Status: done

## Status notes

Shipped via PR #12 (merged, squash commit `0ae276f`).

**Distribution decision (changed from original spec):** Homebrew was dropped.
The original spec asked for a Homebrew tap (separate `treewalkr/homebrew-tap`
repo + PAT). During implementation we decided to ship two zero-infrastructure
paths instead — pre-built binaries via GitHub Releases and `go install` for
source builds — which cover the same audience with no tap repo or PAT to
maintain. A same-repo Homebrew cask remains a cheap follow-up if the
Mac-without-Go audience matters later.

**Verified locally:**
- `go test ./...` + `go vet ./...` green
- `goreleaser check` passes
- `goreleaser build --snapshot --clean` produces all 5 targets, each 5.9–6.4 MB (< 15 MB)
- `hyperdrop --version` correct under both ldflags (release) and build-info (no-ldflags / `go install`) builds

**Remaining operational step (human):** push the first `v*` tag to exercise
the release workflow end-to-end and confirm the GitHub Release is published.

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Cross-platform binary distribution via goreleaser, plus `go install`. One
`goreleaser release` command (triggered by a `v*` tag push) produces binaries
for macOS (amd64 + arm64), Linux (amd64 + arm64), and Windows (amd64), and
uploads them to GitHub Releases. Users can also install from source via
`go install`.

Configuration:
- `.goreleaser.yml` at repo root
- Build targets: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`, `windows/amd64`
- Binary name: `hyperdrop`
- Archive format: tarball for macOS/Linux, zip for Windows
- Ldflags to inject version/commit/date at build time
- Checksums file generated automatically
- Binary should be under 15MB
- `go install`-able: module is `github.com/treewalkr/hyperdrop`, main at `./cmd/hyperdrop`

GitHub Actions workflow for automated releases on tag push (`v*`).

## Acceptance criteria

- [x] `goreleaser build --snapshot --clean` produces binaries for all 5 targets
- [x] Binary size is under 15MB for each platform
- [x] `goreleaser release` uploads artifacts to GitHub Releases on tag push (workflow wired; first real `v*` tag push pending)
- [x] `go install github.com/treewalkr/hyperdrop/cmd/hyperdrop@latest` works (module + main path correct)
- [x] Version is embedded via ldflags (`hyperdrop --version` prints version)
- [x] `--version` also reports real version/commit/date for source builds, via a `runtime/debug.ReadBuildInfo()` fallback
- [x] GitHub Actions workflow triggers on `v*` tag push

### Dropped from original spec

- ~~Homebrew formula is updated in the tap repo~~ — Homebrew dropped (see Status notes)
- ~~`brew install treewalkr/tap/hyperdrop` works on macOS~~ — Homebrew dropped

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
