Status: ready-for-agent

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Subdirectory navigation on the Files page. The API and UI work together to let users browse into folders and upload to subdirectories.

Backend changes:
- `GET /api/files` accepts an optional `?path=sub/dir` query parameter. When present, lists the contents of `root/sub/dir` instead of root. The path is validated through the sandboxing function.
- `POST /api/upload` accepts an optional `?path=sub/dir` query parameter. When present, uploads target `root/sub/dir/` instead of root.

Frontend changes:
- A breadcrumb trail appears above the file listing showing the current path as clickable segments (e.g. `Home > Photos > Vacation`).
- Clicking a folder in the file listing navigates into it (updates the current path and refreshes the listing).
- Clicking a breadcrumb segment jumps back to that directory level.
- Uploads from the Send page target the current subdirectory (the path is stored in Alpine.js state and passed to the upload endpoint).

## Acceptance criteria

- [ ] `GET /api/files?path=subdir` returns listing for that subdirectory
- [ ] Clicking a folder in the Files page navigates into it
- [ ] Breadcrumb trail shows the current path as clickable segments
- [ ] Clicking a breadcrumb segment navigates back to that level
- [ ] `GET /api/files?path=../etc` returns 403 (sandbox enforced)
- [ ] Uploads respect the current subdirectory path
- [ ] Breadcrumb "Home" returns to root listing
- [ ] Tests cover subdirectory listing, path traversal on subdirs, and upload targeting

## Blocked by

- Issue 05 (file listing and download)
- Issue 08 (Aurora Files page)
