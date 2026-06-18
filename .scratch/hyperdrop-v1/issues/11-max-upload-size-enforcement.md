Status: ready-for-code-review

## Status notes

Implemented in PR #11 (feat/max-upload-size). All acceptance criteria met:
`--max-size` now parses human-readable strings (`500MB`→524288000 bytes, `2GB`,
`100KB`, `1.5GB`) via a custom `flag.Value` (`internal/cli`); bare numbers stay
bytes; invalid strings exit code 1. Upload enforcement (Content-Length→413,
`http.MaxBytesReader` for chunked) was already in place from #04; the 413 message
is now human-readable (`file too large: max 500.0 MB`). Covered by
`TestParseArgs_MaxSize_HumanReadable`, `TestParseArgs_MaxSize_Invalid`,
`TestHumanizeSize`, and the existing `TestUpload_MaxSizeExceeded/Chunked` suite.

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Wire the `--max-size` CLI flag into the upload handler to enforce upload size limits.

The `--max-size` flag accepts human-readable size strings: `500MB`, `2GB`, `1.5GB`, `100KB`. The flag is parsed into bytes at startup and stored in the server config.

When `--max-size` is set:
- Check the `Content-Length` header of the upload request before streaming begins
- If `Content-Length` exceeds the limit, reject immediately with 413 JSON `{"error": "file too large: max 500.0 MB"}`
- If `Content-Length` is missing or incorrect (transfer encoding chunked), track bytes written during streaming and abort mid-transfer if the limit is exceeded

When `--max-size` is not set (default), no limit is enforced.

## Acceptance criteria

- [ ] `--max-size 500MB` is parsed correctly to 524288000 bytes
- [ ] `--max-size 2GB`, `--max-size 100KB` parse correctly
- [ ] Upload exceeding the limit returns 413 with a clear error message including the max
- [ ] Upload within the limit succeeds normally
- [ ] Default (no flag) allows unlimited upload size
- [ ] Invalid size string (e.g. `--max-size abc`) prints an error and exits with code 1
- [ ] Tests verify parsing and enforcement at the HTTP level

## Blocked by

- Issue 04 (file upload streaming multipart)
