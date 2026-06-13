Status: done

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

File listing and download API endpoints.

`GET /api/files` returns a JSON array of files in the root directory. Each entry includes: `name`, `size` (bytes), `mod_time` (ISO 8601), `is_dir` (boolean), `category` (doc/img/vid/zip/file based on extension). The category mapping uses the same extension groups from the prototype:
- doc: pdf, doc, docx, txt, md, xls, xlsx, csv, ppt, pptx, rtf, fig
- img: png, jpg, jpeg, gif, svg, webp, bmp, ico
- vid: mp4, mov, avi, mkv, webm, flv, wmv
- zip: zip, rar, 7z, tar, gz, bz2, xz
- file: everything else

`GET /api/files/{path}` serves a file for download using `Content-Disposition: attachment` with the filename. Supports range requests for large files. Path is validated through the sandboxing function.

The placeholder `files.html` gets a minimal Alpine.js list that fetches from `/api/files` on load and renders file names with download links.

## Acceptance criteria

- [ ] `GET /api/files` returns JSON array with name, size, mod_time, is_dir, category
- [ ] Empty directory returns `[]`
- [ ] Files are categorized correctly by extension
- [ ] `GET /api/files/photo.jpg` downloads the file with correct Content-Disposition header
- [ ] Path traversal on download (`/api/files/../../etc/passwd`) returns 403
- [ ] Downloading a nonexistent file returns 404
- [ ] Unauthenticated requests return 401
- [ ] Tests cover listing populated/empty dirs, download success, and security edge cases

## Blocked by

- Issue 01 (CLI bootstrap and static serving)
- Issue 02 (path sandboxing)
- Issue 03 (token auth middleware)
