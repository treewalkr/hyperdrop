Status: ready-for-human

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Cross-platform binary distribution via goreleaser. The goal: one `goreleaser release` command produces binaries for macOS (amd64 + arm64), Linux (amd64 + arm64), and Windows (amd64), uploads them to GitHub Releases, and updates a Homebrew tap.

Configuration:
- `.goreleaser.yml` at repo root
- Build targets: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`, `windows/amd64`
- Binary name: `hyperdrop`
- Archive format: tarball for macOS/Linux, zip for Windows
- Homebrew tap repository: separate repo (e.g. `treewalkr/homebrew-tap`)
- Ldflags to inject version/commit at build time
- Checksums file generated automatically
- Binary should be under 15MB

GitHub Actions workflow for automated releases on tag push (`v*`).

This issue requires manual verification of release artifacts across platforms.

## Acceptance criteria

- [ ] `goreleaser build --snapshot --clean` produces binaries for all 5 targets
- [ ] Binary size is under 15MB for each platform
- [ ] `goreleaser release` uploads artifacts to GitHub Releases on tag push
- [ ] Homebrew formula is updated in the tap repo
- [ ] `brew install treewalkr/tap/hyperdrop` works on macOS
- [ ] Version is embedded via ldflags (`hyperdrop --version` prints version)
- [ ] GitHub Actions workflow triggers on `v*` tag push

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
