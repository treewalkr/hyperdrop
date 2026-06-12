Status: ready-for-agent

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

`POST /api/upload` handler that accepts multipart file uploads and streams them directly to disk within the root directory. Uses `multipart.Reader` (not `multipart.ParseForm()`) to avoid buffering entire files in memory.

The handler:
- Reads each file part from the multipart stream
- Sanitizes the filename via the path sandboxing function
- Streams each part to `rootDir/filename` using `io.Copy`
- Respects `--max-size` (if set) by checking `Content-Length` before streaming begins; rejects oversize uploads with 413 JSON `{"error": "file too large: max X MB"}`
- Returns 200 JSON `{"files": [{"name": "photo.jpg", "size": 4200000}]}` on success
- Returns appropriate error JSON for disk-full, permission-denied, and path-traversal attempts

The placeholder `index.html` gets a minimal Alpine.js dropzone: a clickable area that opens a file picker, posts selected files to `/api/upload` via `fetch`, and shows a simple success/error message. This is a bare-bones wiring — the full Aurora UI comes in issue 07.

## Acceptance criteria

- [ ] `POST /api/upload` with a multipart file saves the file to the root directory
- [ ] Multiple files in one upload request are all saved
- [ ] Large files (100MB+) stream without excessive memory usage
- [ ] Filenames with `..` or absolute paths are rejected by the sandbox
- [ ] When `--max-size` is set, oversize uploads return 413 with a clear error message
- [ ] Missing `Content-Type: multipart/form-data` returns 400
- [ ] Unauthenticated uploads return 401
- [ ] Tests use httptest with real multipart bodies (including large files)

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
- Issue 02 (path sandboxing)
- Issue 03 (token auth middleware)
