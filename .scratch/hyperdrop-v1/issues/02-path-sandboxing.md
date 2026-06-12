Status: done

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

A pure `sanitizePath(rootDir, requestedPath) (string, error)` function that resolves and validates all file operation paths against the root directory. This is the security boundary for the entire application.

The function must:
- Resolve the real absolute path using `filepath.Abs()` and `filepath.Rel()`
- Reject any resolved path that doesn't start with the root directory prefix
- Reject paths containing `..` traversal
- Reject absolute paths that escape root
- Handle symlinks by resolving the real path before comparison
- Return the cleaned, validated absolute path on success

This function is a standalone testable unit in `internal/sandbox/` with exhaustive tests covering: valid paths, `..` traversal attempts, absolute paths outside root, symlinks pointing outside root, URL-encoded characters, mixed case on case-insensitive filesystems, and empty paths.

## Acceptance criteria

- [ ] `sanitizePath("/root", "subdir/file.txt")` returns `/root/subdir/file.txt` without error
- [ ] `sanitizePath("/root", "../etc/passwd")` returns an error
- [ ] `sanitizePath("/root", "/etc/passwd")` returns an error
- [ ] `sanitizePath("/root", "subdir/../../etc/passwd")` returns an error
- [ ] Symlinks pointing outside root are rejected
- [ ] Valid nested paths resolve correctly
- [ ] All edge cases have dedicated unit tests

## Blocked by

None — can start immediately (parallel with issues 01 and 02)
