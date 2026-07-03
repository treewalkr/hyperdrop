package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/treewalkr/hyperdrop/internal/cli"
	"github.com/treewalkr/hyperdrop/internal/sandbox"
	"github.com/treewalkr/hyperdrop/internal/static"
)

// NewRouter builds a Chi mux with static file routes for the given config.
func NewRouter(cfg cli.Config) chi.Router {
	r, _ := newRouterWithHub(cfg)
	return r
}

// newRouterWithHub builds the router and returns the event Hub alongside it, so
// tests can inspect subscriber lifecycle (connect/disconnect cleanup).
func newRouterWithHub(cfg cli.Config) (chi.Router, *Hub) {
	r := chi.NewRouter()

	var assets fs.FS
	if cfg.Dev {
		assets = os.DirFS("internal/static")
	} else {
		assets = static.Assets
	}

	hub := newHub()

	// Static pages — no auth required. They are served through
	// servePageWithToken so that a request carrying a valid ?token= (the
	// form produced by clicking the token-bearing Send<->Files nav links)
	// sets the session cookie AND injects the token onto those same-origin
	// nav hrefs. This avoids the cold-load 401 described in issue #17: the
	// first page request after authentication carries ?token=, sets the
	// cookie, and from then on the cookie — not the URL — is the credential.
	// Unauthenticated page requests (no token, no cookie) serve the page
	// with plain token-less hrefs and never receive the token.
	r.Get("/", servePageWithToken(assets, "index.html", cfg.Token))
	r.Get("/files", servePageWithToken(assets, "files.html", cfg.Token))
	r.Get("/player", servePageWithToken(assets, "player.html", cfg.Token))
	r.Get("/shares", servePageWithToken(assets, "shares.html", cfg.Token))

	// Public share landing. The opaque token in the path is the recipient's
	// only credential; shareLandingHandler validates it and sets a scoped
	// cookie that fileAccessAuth below honors for the bound path alone. No
	// global token is required or revealed here.
	r.Get("/s/{token}", shareLandingHandler(assets, hub))

	// File bytes — owner global token OR scoped share token. These sit outside
	// tokenAuth so a share recipient (who has no global token) can still
	// stream/download the single file their link grants; any other path or
	// endpoint remains owner-only. chi registers by method+pattern, so GET
	// /api/files/* here and DELETE /api/files/* in the owner group coexist.
	r.Group(func(r chi.Router) {
		r.Use(fileAccessAuth(cfg, hub))
		r.Get("/api/stream/*", streamHandler(cfg))
		// chi does not auto-alias HEAD onto GET routes; register it explicitly
		// so the player can HEAD /api/stream/* for size/type without fetching
		// the body (http.ServeContent answers HEAD with headers only).
		r.Head("/api/stream/*", streamHandler(cfg))
		r.Get("/api/files/*", downloadHandler(cfg))
	})

	// Owner-only API — global token required.
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth(cfg.Token))
		r.Get("/api/ws", wsHandler(hub))
		r.Post("/api/upload", uploadHandler(cfg, hub))
		r.Get("/api/files", listHandler(cfg))
		r.Delete("/api/files/*", deleteHandler(cfg, hub))
		// Share-link management (create/list/revoke). Owner-only by virtue of
		// tokenAuth; a share recipient cannot mint or revoke links.
		r.Get("/api/shares", listSharesHandler(hub))
		r.Post("/api/shares", createShareHandler(cfg, hub))
		r.Delete("/api/shares/{token}", revokeShareHandler(hub))
	})

	return r, hub
}

type uploadResult struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

const (
	// progressChunk is the read/write granularity used while streaming an
	// uploaded part to disk. It bounds how much can be reported in a single
	// progress event: capping the read here guarantees the first event of an
	// upload cannot jump straight to total (so 0 < bytesWritten < total holds
	// for any upload larger than one chunk).
	progressChunk = 64 * 1024

	// progressInterval is the per-file flood guard on upload_progress events.
	// A naive per-chunk broadcast would yield total/progressChunk events for a
	// large file, flooding every connected browser. The handler instead emits
	// at most one event per progressInterval: the first chunk always fires
	// (the prior-emit time is the zero value, so elapsed ≫ interval), then at
	// most one per interval thereafter.
	progressInterval = 100 * time.Millisecond
)

// uploadProgressEvent carries the invariant fields shared by every progress
// broadcast for one file; bytesWritten and total are filled in per emit.
type uploadProgressEvent struct {
	path string
	name string
}

// copyProgressed streams src to dst in progressChunk-sized chunks, returning
// the total bytes copied. As bytes accumulate it broadcasts throttled
// upload_progress events through hub: total is the announced upload size (the
// whole multipart request's Content-Length) used only to render a fraction —
// the terminal file_uploaded event broadcast by the caller marks completion.
func copyProgressed(dst io.Writer, src io.Reader, hub *Hub, ev uploadProgressEvent, total int64) (int64, error) {
	var written int64
	buf := make([]byte, progressChunk)
	var last time.Time
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			nw, werr := dst.Write(buf[:n])
			written += int64(nw)
			now := time.Now()
			if now.Sub(last) >= progressInterval {
				last = now
				hub.broadcast(map[string]any{
					"type":         "upload_progress",
					"path":         ev.path,
					"name":         ev.name,
					"bytesWritten": written,
					"total":        total,
				})
			}
			if werr != nil {
				return written, werr
			}
			if nw < n {
				return written, io.ErrShortWrite
			}
		}
		if rerr == io.EOF {
			// Terminal flush: a fast upload can complete entirely inside one
			// throttle window, which would leave the bar pinned at the first
			// chunk's fraction. Emit the final byte count so clients always see
			// the bar climb toward 100% before the caller's file_uploaded marks
			// completion. This is a single event, so the per-100ms ceiling still
			// holds.
			hub.broadcast(map[string]any{
				"type":         "upload_progress",
				"path":         ev.path,
				"name":         ev.name,
				"bytesWritten": written,
				"total":        total,
			})
			return written, nil
		}
		if rerr != nil {
			return written, rerr
		}
	}
}

func uploadHandler(cfg cli.Config, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.MaxSize > 0 {
			if r.ContentLength > cfg.MaxSize {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
					"error": fmt.Sprintf("file too large: max %s", cli.HumanizeSize(cfg.MaxSize)),
				})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxSize)
		}

		// Resolve optional subdirectory target (?path=sub/dir). Default root.
		base, err := resolveTargetDir(cfg.RootDir, r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}

		reader, err := r.MultipartReader()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected multipart/form-data"})
			return
		}

		var saved []uploadResult
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			filename := rawFilename(part)
			if filename == "" {
				continue
			}

			dest, err := sandbox.SanitizePath(base, filename)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}

			// Directory uploads arrive as a single part whose filename carries
			// the relative path (e.g. "vacation/sub/a.txt"). SanitizePath already
			// resolved that to a safe nested dest; create any missing ancestor
			// directories so the file write succeeds for nested files. No-op for
			// a flat filename (filepath.Dir == ".").
			if dir := filepath.Dir(dest); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			}

			// Resolve a name collision inside the final directory rather than
			// overwriting, and open the slot atomically (O_EXCL) so two
			// concurrent uploads of the same name can't both win it (issue #16).
			f, dest, err := createUniqueFile(filepath.Dir(dest), filepath.Base(dest))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			// Derive the wire-format name from the actually-written dest. The
			// basename carries no separator (symlink-safe), and the directory is
			// taken straight from the sanitized+renamed path so the reported
			// location always matches what's on disk — never from the raw
			// request filename, which may carry ".." or "//" that SanitizePath
			// cleaned away. Wire format is forward slashes (webkitRelativePath).
			baseName := filepath.Base(dest)
			evPath := relFromRoot(cfg.RootDir, filepath.Dir(dest))
			writtenName := path.Join(evPath, baseName)

			n, err := copyProgressed(f, part, hub, uploadProgressEvent{
				path: evPath,
				// The REQUESTED name (the raw multipart filename), not the
				// renamed writtenName. Progress events fire mid-transfer, while
				// the uploader's card still carries the requested name — the
				// client matches them with (relPath || name) === data.name, and
				// relPath/name are exactly the multipart filename sent. Using
				// writtenName here (root-relative, and renamed on collision)
				// would mismatch for any upload into a subdirectory or any
				// collision rename, dropping the authoritative WS progress
				// signal. The renamed name reaches the client at completion via
				// writtenName in the response / file_uploaded below.
				name: filename,
			}, r.ContentLength)
			f.Close()
			if err != nil {
				os.Remove(dest)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			saved = append(saved, uploadResult{Name: writtenName, Size: n})
			hub.broadcast(map[string]any{
				"type": "file_uploaded",
				"path": evPath,
				"file": map[string]any{
					"name":     baseName,
					"size":     n,
					"category": categorize(baseName),
				},
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"files": saved})
	}
}

type fileEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mod_time"`
	IsDir    bool   `json:"is_dir"`
	Category string `json:"category"`
}

func listHandler(cfg cli.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := resolveTargetDir(cfg.RootDir, r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}

		entries, err := os.ReadDir(target)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		result := make([]fileEntry, 0)
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			result = append(result, fileEntry{
				Name:     e.Name(),
				Size:     info.Size(),
				ModTime:  info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				IsDir:    e.IsDir(),
				Category: categorize(e.Name()),
			})
		}

		writeJSON(w, http.StatusOK, result)
	}
}

// resolveTargetDir resolves the directory for an optional sub-path relative to
// rootDir. Empty sub returns root; traversal-escaping sub returns an error.
func resolveTargetDir(rootDir, sub string) (string, error) {
	if sub == "" {
		return rootDir, nil
	}
	return sandbox.SanitizePath(rootDir, sub)
}

// relFromRoot returns abs as a path relative to rootDir in URL-path form
// (forward slashes), with root represented as "". The result matches the
// ?path=<dir> value the list/upload handlers and the Files page use, so a
// broadcast event's "path" can be compared directly against the open view.
//
// rootDir is resolved the same way sandbox.SanitizePath resolves it (Abs +
// EvalSymlinks) so the comparison is consistent even when the root contains
// symlinked segments — e.g. macOS temp dirs where /var/folders links to
// /private/var/folders.
func relFromRoot(rootDir, abs string) string {
	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
			absRoot = resolved
		}
	}
	// Resolve abs too so both sides are on the same (de-symlinked) footing —
	// when ?path= is empty, base is the raw rootDir and may still carry a
	// symlinked segment while absRoot does not.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return rel
}

func categorize(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".pdf", ".doc", ".docx", ".txt", ".md", ".xls", ".xlsx",
		".csv", ".ppt", ".pptx", ".rtf", ".fig":
		return "doc"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico":
		return "img"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv":
		return "vid"
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return "zip"
	default:
		return "file"
	}
}

// playable reports whether a video file can be decoded by browsers' native
// <video> element. Containers like mkv/avi/flv/wmv are tagged "vid" by
// categorize but cannot play in-browser, so the player falls back to download.
func playable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".m4v", ".webm", ".ogv", ".mov":
		return true
	default:
		return false
	}
}

// escapeFilename produces a safe Content-Disposition filename token.
// If the name contains only safe chars, returns a quoted string.
// Otherwise returns RFC 6266 filename*=UTF-8” URL-encoded form.
func escapeFilename(name string) string {
	safe := true
	for _, r := range name {
		if r < ' ' || r == '"' || r == '\\' || r > '~' {
			safe = false
			break
		}
	}
	if safe {
		return `"` + name + `"`
	}
	return "UTF-8''" + url.PathEscape(name)
}

func downloadHandler(cfg cli.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// chi wildcards: /* captures as "/" + remainder
		requested := chi.URLParam(r, "*")
		requested = strings.TrimPrefix(requested, "/")
		serveFileContent(w, r, cfg, requested, true)
	}
}

// streamHandler serves file bytes inline (no Content-Disposition: attachment)
// so a <video src> element can play them. http.ServeContent still honors Range
// requests, so seeking works without extra code.
func streamHandler(cfg cli.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requested := chi.URLParam(r, "*")
		requested = strings.TrimPrefix(requested, "/")
		serveFileContent(w, r, cfg, requested, false)
	}
}

// serveFileContent streams file bytes with Range/206 support via
// http.ServeContent. The response always carries X-Content-Type-Options:
// nosniff. Content-Disposition is forced to attachment unless attachment is
// false AND the file is a browser-playable video type — only that case is
// served inline, so a <video> element can play it. Non-playable extensions
// on the inline/stream path are still forced to attachment to prevent
// uploaded HTML/SVG from executing as same-origin script (issue #42).
func serveFileContent(w http.ResponseWriter, r *http.Request, cfg cli.Config, requested string, attachment bool) {
	if requested == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filename required"})
		return
	}

	dest, err := sandbox.SanitizePath(cfg.RootDir, requested)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}

	f, err := os.Open(dest)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	name := filepath.Base(dest)
	// Security: anything served inline must be a browser-playable type (issue
	// #42). A non-playable extension (.html/.svg/etc.) reaching the stream
	// path would otherwise be rendered inline with a sniffable content type,
	// executing uploaded bytes as same-origin script. Force a download for
	// those. attachment==true (downloadHandler) already downloads; only the
	// inline/stream path needs the extra gate. X-Content-Type-Options:
	// nosniff is defense-in-depth so a sniffed type can't override the
	// disposition (slice of #47).
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if attachment || !playable(name) {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", escapeFilename(name)))
	}
	http.ServeContent(w, r, name, stat.ModTime(), f)
}

func deleteHandler(cfg cli.Config, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requested := chi.URLParam(r, "*")
		requested = strings.TrimPrefix(requested, "/")

		if requested == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filename required"})
			return
		}

		dest, err := sandbox.SanitizePath(cfg.RootDir, requested)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}

		if _, err := os.Stat(dest); os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
			return
		}

		if err := os.RemoveAll(dest); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "permission denied"})
			return
		}

		hub.broadcast(map[string]any{
			"type": "file_deleted",
			"path": relFromRoot(cfg.RootDir, filepath.Dir(dest)),
			"name": filepath.Base(dest),
		})

		writeJSON(w, http.StatusOK, map[string]string{"deleted": filepath.Base(dest)})
	}
}

// rawFilename extracts the unprocessed filename from the Content-Disposition header.
// Unlike multipart.Part.FileName(), it does NOT call filepath.Base(), so path
// traversal characters reach sandbox.SanitizePath for validation.
func rawFilename(part *multipart.Part) string {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return ""
	}
	return params["filename"]
}

// maxCollisionAttempts bounds how many " (N)" suffixes resolveCollidingName
// will try before giving up. A package var (not a const) so tests can lower it
// to exercise the exhaustion path without creating thousands of files.
var maxCollisionAttempts = 10000

// resolveCollidingName returns a path inside dir for a requested filename,
// renaming to "name (1).ext", "name (2).ext", … when the requested name
// already exists so uploads never silently overwrite (issue #16).
//
// Path-sandbox AC: the suffix is inserted into the basename only. Since the
// caller passes filepath.Base(dest) — a name with no path separator — the
// derived candidate cannot introduce traversal and stays within the already-
// sanitized directory, equivalent to per-candidate re-validation.
//
// This selects a name by Lstat; it does not create the file. Callers that
// must win the slot atomically under concurrency open the result with
// O_CREATE|O_EXCL and re-resolve on EEXIST (see createUniqueFile) — a bare
// os.Create would reintroduce a Lstat-then-Create TOCTOU race.
//
// Returns an error if the loop is exhausted (every candidate up to
// maxCollisionAttempts is taken) or a stat fails for a reason other than the
// candidate not existing; those real errors are surfaced rather than masked.
func resolveCollidingName(dir, requested string) (string, error) {
	target := filepath.Join(dir, requested)
	if free, err := slotFree(target); err != nil {
		return "", err
	} else if free {
		return target, nil
	}

	ext := filepath.Ext(requested)
	// A leading-dot name is a dotfile (e.g. ".gitignore", ".env.local"), not
	// "no name + extension" — keep the whole name as the base so the suffix
	// lands after it. Checking only the leading dot (not a dot count) handles
	// multi-dot dotfiles too: ".env.local" → ".env.local (1)", not the split
	// ".env (1).local".
	if strings.HasPrefix(requested, ".") {
		ext = ""
	}
	// If the requested name already carries a " (N)" counter (this package's
	// own suffix), continue the sequence rather than stacking a second one:
	// re-uploading "report (1).pdf" yields "report (2).pdf", not
	// "report (1) (1).pdf".
	base := stripCounterSuffix(strings.TrimSuffix(requested, ext))

	for i := 1; i <= maxCollisionAttempts; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		free, err := slotFree(candidate)
		if err != nil {
			return "", err
		}
		if free {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find non-colliding name for %q after %d attempts", requested, maxCollisionAttempts)
}

// createUniqueFile opens a new file in dir for requested, renaming on
// collision ("name (1).ext", …) so an upload never silently overwrites, and
// returns the opened file and its path.
//
// The candidate is created with O_CREATE|O_EXCL, which is atomic: two
// concurrent uploads of the same name cannot both win the same slot — the
// loser gets EEXIST and the loop moves to the next candidate. This closes the
// TOCTOU a stat-then-open resolver would leave open, and also refuses a symlink
// an attacker plants at a candidate path between attempts (O_EXCL on an
// existing symlink fails with EEXIST rather than opening through it).
func createUniqueFile(dir, requested string) (*os.File, string, error) {
	for {
		target, err := resolveCollidingName(dir, requested)
		if err != nil {
			return nil, "", err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return f, target, nil
		}
		if os.IsExist(err) {
			// Lost the race, or a name was taken between resolveCollidingName's
			// Lstat and this open. Re-resolve: the now-occupied slot is skipped.
			continue
		}
		return nil, "", err
	}
}

// stripCounterSuffix removes a trailing " (N)" counter — the form
// resolveCollidingName appends — from base so a re-upload of an already-
// suffixed name continues the sequence instead of stacking another counter.
// Names without a trailing " (N)" (or with a non-numeric N) are returned
// unchanged.
func stripCounterSuffix(base string) string {
	if !strings.HasSuffix(base, ")") {
		return base
	}
	open := strings.LastIndex(base, " (")
	if open < 0 {
		return base
	}
	num := base[open+2 : len(base)-1]
	if num == "" {
		return base
	}
	for _, r := range num {
		if r < '0' || r > '9' {
			return base
		}
	}
	return base[:open]
}

// slotFree reports whether path does not exist. A stat error that is not
// "not exist" (permission denied, I/O) is returned so callers don't mistake a
// real failure for a collision and silently skip past it.
func slotFree(path string) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return false, nil
	} else if os.IsNotExist(err) {
		return true, nil
	} else {
		return false, err
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func serveFile(fsys fs.FS, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := fsys.Open(name)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		rs, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, name, stat.ModTime(), rs)
	}
}

// servePageWithToken serves an embedded HTML page. When the request carries a
// valid ?token= query param, it does two things before serving:
//
//  1. Sets the hyperdrop_session cookie (same attributes as tokenAuth), so the
//     browser authenticates subsequent same-origin requests — including the
//     file-list API call the page fires on load — via the cookie rather than
//     the URL. This is the fix for issue #17: a cold first visit no longer
//     401s because the cookie is set by the page request itself.
//  2. Injects the token onto the same-origin Send<->Files nav hrefs, so the
//     next in-app navigation also carries ?token= and (re)sets the cookie.
//     This is not redundant with the cookie once the cookie exists: the
//     session cookie has no MaxAge/Expires, so it is cleared when the browser
//     closes. After a restart, in-app nav via plain hrefs would have neither
//     token nor cookie; the token-bearing href re-seeds the cookie.
//
// The token is only ever echoed into a response when the request already
// proved knowledge of it. Unauthenticated requests (no token, no cookie)
// receive the page verbatim, with token-less hrefs and no token in the body.
//
// A Referrer-Policy: same-origin header is set on every response so the
// token-bearing URL is never sent as a Referer to a cross-origin endpoint
// (the pages load Alpine from cdn.jsdelivr.net). Browsers default to
// strict-origin-when-cross-origin, which already strips the query string for
// cross-origin requests, but setting it explicitly makes the no-leak
// guarantee hold by construction rather than relying on the browser default.
func servePageWithToken(fsys fs.FS, name, validToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set before any potential write so it applies to every response path.
		w.Header().Set("Referrer-Policy", "same-origin")

		f, err := fsys.Open(name)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// No token to carry: serve verbatim with cache-friendly semantics.
		queryToken := r.URL.Query().Get("token")
		if validToken == "" || queryToken != validToken {
			rs, ok := f.(io.ReadSeeker)
			if !ok {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			http.ServeContent(w, r, name, stat.ModTime(), rs)
			return
		}

		// Valid token: set the cookie (cookie is the persistent credential),
		// then inject the token into the same-origin nav hrefs.
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    validToken,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})

		body, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The injected body is token-specific, so it must never be cached
		// (an ETag/Last-Modified here would let a token-bearing body be
		// reused across users). no-store makes that intent explicit; the
		// unauthenticated branch above keeps cache-friendly ServeContent.
		w.Header().Set("Cache-Control", "no-store")
		w.Write(injectNavToken(body, validToken))
	}
}

// injectNavToken rewrites the same-origin nav hrefs to carry ?token=<token>,
// scoped to the in-page <nav class="header-nav"> block:
//
//   - href="/"       -> href="/?token=<token>"         (the Send nav link)
//   - href="/files"  -> href="/files?token=<token>"     (the Files nav link)
//   - href="/shares" -> href="/shares?token=<token>"    (the Shares nav link)
//
// Scoping to the nav element — rather than rewriting every href="/" in the
// page and then trying to un-catch the brand anchor by matching its exact
// attribute serialization — means the brand link (href="/" class="brand", a
// sibling OUTSIDE the nav) and any external/data-URI link can never receive
// the token, regardless of how the markup's attributes are ordered. Each
// target href appears exactly once inside the nav.
//
// If the page structure changes such that the nav block can't be located,
// it fails safe: the page is served verbatim (no token injected, so no leak).
// Auth then degrades to the session cookie or the original token URL.
func injectNavToken(body []byte, token string) []byte {
	const navOpen = `<nav class="header-nav">`
	s := string(body)
	start := strings.Index(s, navOpen)
	if start < 0 {
		return body
	}
	end := strings.Index(s[start:], "</nav>")
	if end < 0 {
		return body
	}
	end += start + len("</nav>")

	encoded := url.QueryEscape(token)
	block := s[start:end]
	block = strings.Replace(block, `href="/files"`, `href="/files?token=`+encoded+`"`, 1)
	block = strings.Replace(block, `href="/shares"`, `href="/shares?token=`+encoded+`"`, 1)
	block = strings.Replace(block, `href="/"`, `href="/?token=`+encoded+`"`, 1)
	return []byte(s[:start] + block + s[end:])
}

// NetworkURL returns the full URL a user should open in their browser,
// including the token as a query parameter.
func NetworkURL(cfg cli.Config) string {
	host := cfg.Host
	if host == "0.0.0.0" {
		host = lookupLANIP()
	}
	return fmt.Sprintf("http://%s:%d/?token=%s", host, cfg.Port, url.QueryEscape(cfg.Token))
}

// lookupLANIP returns the first non-loopback IPv4 address.
// Note: may pick Docker or VPN interfaces on machines with those installed.
func lookupLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

// newHTTPServer constructs the *http.Server used by RunServer.
//
// Only ReadHeaderTimeout and IdleTimeout are set: together they mitigate
// slowloris-style denial of service (an unauthenticated attacker keeping a
// connection open by dripping headers or holding it idle) without an auth
// check. WriteTimeout and ReadTimeout are intentionally left zero because
// they would terminate large streamed uploads (uploadHandler) and long-lived
// /api/stream video responses mid-transfer.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// RunServer starts the HTTP server, prints the network URL to w, and blocks
// until the server exits.
func RunServer(cfg cli.Config, w io.Writer) error {
	r := NewRouter(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Fprintf(w, "%s\n", NetworkURL(cfg))

	srv := newHTTPServer(addr, r)
	return srv.ListenAndServe()
}

const sessionCookieName = "hyperdrop_session"

// tokenAuth returns middleware that validates token via cookie or ?token= query param.
func tokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check session cookie first.
			if c, err := r.Cookie(sessionCookieName); err == nil && c.Value == token {
				next.ServeHTTP(w, r)
				return
			}
			// Check ?token= query param.
			if t := r.URL.Query().Get("token"); t == token {
				http.SetCookie(w, &http.Cookie{
					Name:     sessionCookieName,
					Value:    token,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Path:     "/",
				})
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		})
	}
}

// shareTokenParam is the query parameter a share landing bakes into its
// stream/download URLs to identify the recipient's link. It travels in the URL
// rather than a cookie so two share links open in the same browser can't
// clobber each other's credential — each tab carries its own token in its own
// request URLs.
const shareTokenParam = "s"

// ownerRequest reports whether r carries the global owner credential (session
// cookie or ?token= matching validToken). It is the credential half of
// tokenAuth without the cookie-setting side effect, factored out so fileAccessAuth
// can accept the owner credential OR a scoped share credential.
func ownerRequest(r *http.Request, validToken string) bool {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value == validToken {
		return true
	}
	if t := r.URL.Query().Get("token"); t != "" && t == validToken {
		return true
	}
	return false
}

// shareAdmitted reports whether r carries a live share token (the ?s= param the
// landing bakes into its URLs) whose bound file equals the sandbox-resolved
// requested path. The path re-check on every request is the scope enforcement:
// a token unlocks exactly one AbsPath, and traversal/sibling paths are rejected.
func shareAdmitted(r *http.Request, hub *Hub, rootDir string) bool {
	tok := r.URL.Query().Get(shareTokenParam)
	if tok == "" {
		return false
	}
	rec, ok := hub.shares.Lookup(tok)
	if !ok {
		return false
	}
	requested := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	abs, err := sandbox.SanitizePath(rootDir, requested)
	return err == nil && abs == rec.AbsPath
}

// fileAccessAuth guards the byte-serving endpoints (/api/stream/*, /api/files/*).
// It admits:
//  1. The owner — any request carrying the global token (cookie or ?token=),
//     with full access to every path under root, same as before.
//  2. A share recipient — a request whose ?s= token names a live share whose
//     AbsPath equals the sandbox-resolved requested path. Any other path
//     returns 401, so a share link unlocks exactly one file.
//
// This is intentionally separate from tokenAuth so upload/list/delete stay
// owner-only: a share token never reaches those routes.
func fileAccessAuth(cfg cli.Config, hub *Hub) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ownerRequest(r, cfg.Token) || shareAdmitted(r, hub, cfg.RootDir) {
				next.ServeHTTP(w, r)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		})
	}
}

// shareJSON is the wire shape for share endpoints. ExpiresAt is a pointer so a
// never-expiring link serializes to a JSON null rather than the zero time.
type shareJSON struct {
	Token     string     `json:"token"`
	URL       string     `json:"url"`
	Path      string     `json:"path"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func toShareJSON(r *http.Request, rec ShareRecord) shareJSON {
	sj := shareJSON{
		Token:     rec.Token,
		URL:       ShareURL(r, rec.Token),
		Path:      rec.Path,
		CreatedAt: rec.CreatedAt,
	}
	if !rec.ExpiresAt.IsZero() {
		exp := rec.ExpiresAt
		sj.ExpiresAt = &exp
	}
	return sj
}

// createShareHandler mints a scoped share link for one file. Directories are
// rejected (single-file scope); the path is sandbox-resolved and its absolute
// form is what fileAccessAuth later matches against, so the binding cannot be
// rewritten by a path-trick on the recipient side.
func createShareHandler(cfg cli.Config, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path string `json:"path"`
			TTL  string `json:"ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		rel := strings.TrimSpace(req.Path)
		rel = strings.TrimPrefix(rel, "/") // tolerate an accidental leading slash
		if rel == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
			return
		}
		abs, err := sandbox.SanitizePath(cfg.RootDir, rel)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		info, err := os.Stat(abs)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
			return
		}
		if info.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot share a directory"})
			return
		}
		ttl, ok := parseTTL(req.TTL)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ttl"})
			return
		}
		display := relFromRoot(cfg.RootDir, abs)
		rec, err := hub.shares.Create(display, abs, ttl)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, toShareJSON(r, rec))
	}
}

// listSharesHandler returns every live share (newest-first) for the Shares page.
func listSharesHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		recs := hub.shares.List()
		out := make([]shareJSON, 0, len(recs))
		for _, rec := range recs {
			out = append(out, toShareJSON(r, rec))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// revokeShareHandler deletes a share link by token. Idempotent — revoking an
// already-gone link still returns 200.
func revokeShareHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hub.shares.Revoke(chi.URLParam(r, "token"))
		writeJSON(w, http.StatusOK, map[string]string{"ok": "revoked"})
	}
}

// encodeRelPath URL-encodes a forward-slash relative path segment-by-segment,
// keeping "/" as a separator — the same form the Files page produces with
// encPath() and serveFileContent consumes via chi's "*" wildcard.
func encodeRelPath(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// shareLandingView is the server-injected config for the stripped landing page.
// It is JSON-encoded (html-safe by encoding/json's default HTML escaping, so a
// crafted filename cannot break out of the <script> it's embedded in).
type shareLandingView struct {
	EncPath  string `json:"encPath"`
	Name     string `json:"name"`
	Token    string `json:"token"`
	Playable bool   `json:"playable"`
}

// shareLandingHandler serves the public recipient page at /s/{token}. On a
// valid, non-expired token it renders the stripped landing template with the
// bound file's path/token/playability injected; otherwise it renders a plain
// "expired or invalid" page (no token details leak). No cookie is set — the
// token is baked into the page's own stream/download URLs as ?s=, so each tab
// is independently credentialed.
func shareLandingHandler(assets fs.FS, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "same-origin")
		rec, ok := hub.shares.Lookup(chi.URLParam(r, "token"))
		if !ok {
			serveShareNotFound(w, r)
			return
		}
		base := filepath.Base(rec.AbsPath)
		serveShareLanding(assets, w, r, shareLandingView{
			EncPath:  encodeRelPath(rec.Path),
			Name:     base,
			Token:    rec.Token,
			Playable: playable(base),
		})
	}
}

// serveShareLanding renders share.html with the view JSON injected in place of
// the __SHARE_DATA__ placeholder. The body is per-token (the injected path),
// so it is served no-store like the token-bearing page responses.
func serveShareLanding(assets fs.FS, w http.ResponseWriter, r *http.Request, view shareLandingView) {
	f, err := assets.Open("share.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	raw, err := json.Marshal(view)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	body = bytes.ReplaceAll(body, []byte("__SHARE_DATA__"), raw)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(body)
}

// serveShareNotFound is the dead-link page for an expired/revoked/unknown token.
func serveShareNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	const page = `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1.0">` +
		`<title>HyperDrop — Link unavailable</title>` +
		`<style>body{background:#080b1a;color:#e8e8f8;font-family:system-ui,sans-serif;` +
		`display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0}` +
		`.card{max-width:420px;text-align:center;padding:2.5rem;border-radius:16px;` +
		`background:rgba(255,255,255,0.04);border:1px solid rgba(255,255,255,0.08)}` +
		`h1{font-size:1.1rem;margin:0 0 .5rem}p{color:#7878a0;margin:0;font-size:.9rem}</style></head>` +
		`<body><div class="card"><h1>This link is no longer available</h1>` +
		`<p>It has expired, been revoked, or the server was restarted.</p></div></body></html>`
	io.WriteString(w, page)
}

// ShareURL builds the recipient URL for a share token using the scheme and host
// the owner accessed the app through (r.Host — typically the LAN ip:port), so
// the link is reachable as-is by other devices on the same network. The scheme
// follows the request, so a TLS or reverse-proxied deployment yields an https
// link rather than forcing a downgrade.
func ShareURL(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/s/%s", scheme, r.Host, token)
}
