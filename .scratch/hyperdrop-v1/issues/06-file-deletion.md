Status: done

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

`DELETE /api/files/{path}` handler that deletes a file or empty directory within the root directory. The path is validated through the sandboxing function before deletion.

Behavior:
- Deletes the file at the sanitized path using `os.Remove`
- For directories, uses `os.RemoveAll` to handle non-empty dirs
- Returns 200 JSON `{"deleted": "filename"}` on success
- Returns 404 JSON `{"error": "file not found"}` if the path doesn't exist
- Returns 403 for path traversal attempts
- Returns 500 for permission errors with `{"error": "permission denied"}`

## Acceptance criteria

- [ ] `DELETE /api/files/photo.jpg` removes the file from disk and returns 200
- [ ] Deleting a nonexistent file returns 404
- [ ] Path traversal on delete (`/api/files/../../etc/passwd`) returns 403
- [ ] Deleting a directory removes it and its contents
- [ ] Unauthenticated delete returns 401
- [ ] Tests use a temp directory and verify filesystem state after delete

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
- Issue 02 (path sandboxing)
- Issue 03 (token auth middleware)
