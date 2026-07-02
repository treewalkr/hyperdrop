package server

import (
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

	// API routes — token auth required
	r.Route("/api", func(r chi.Router) {
		r.Use(tokenAuth(cfg.Token))
		r.Get("/ws", wsHandler(hub))
		r.Post("/upload", uploadHandler(cfg, hub))
		r.Get("/files", listHandler(cfg))
		r.Get("/files/*", downloadHandler(cfg))
		r.Delete("/files/*", deleteHandler(cfg, hub))
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

		relDir := relFromRoot(cfg.RootDir, base)

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
			// directories so os.Create succeeds for nested files. No-op for a
			// flat filename (filepath.Dir == ".").
			if dir := filepath.Dir(dest); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			}

			// Resolve a name collision inside the final directory rather than
			// overwriting (issue #16). The suffix is inserted into the basename
			// only, so the result stays within the already-sanitized directory
			// — no path-sandbox escape via the suffix.
			resolved, err := resolveCollidingName(filepath.Dir(dest), filepath.Base(dest))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			dest = resolved

			f, err := os.Create(dest)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			// The rename (if any) only touches the basename; the directory part
			// of the request is unchanged. Derive the actually-written basename
			// straight from dest — a basename has no separator, so this is
			// symlink-safe without needing filepath.Rel — and recombine it with
			// the request's directory for the wire-format name. The wire format
			// is always forward slashes (webkitRelativePath / DataTransfer).
			baseName := filepath.Base(dest)
			fileDir := path.Dir(filename)
			evPath := relDir
			if fileDir != "" && fileDir != "." {
				if evPath == "" {
					evPath = fileDir
				} else {
					evPath = evPath + "/" + fileDir
				}
			}
			writtenName := baseName
			if fileDir != "" && fileDir != "." {
				writtenName = fileDir + "/" + baseName
			}

			n, err := copyProgressed(f, part, hub, uploadProgressEvent{
				path: evPath,
				// Full relative filename (e.g. "vacation/sub/a (1).txt") so the
				// uploader can match progress to the exact entry even when two
				// files in different subfolders share a basename. The terminal
				// file_uploaded below uses the basename for display.
				name: writtenName,
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
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", escapeFilename(name)))
		http.ServeContent(w, r, name, stat.ModTime(), f)
	}
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
// Design note on the path-sandbox AC: rather than re-running SanitizePath on
// every candidate, the suffix is inserted into the basename only. Since the
// caller passes filepath.Base(dest) — a name with no path separator — the
// derived candidate cannot introduce traversal and stays within the already-
// sanitized directory. The guarantee ("no sandbox escape via the suffix") is
// therefore equivalent to per-candidate re-validation.
//
// The directory is re-stat'd for each candidate (no single pre-scan), so two
// concurrent uploads of the same name are unlikely to resolve identically —
// but a TOCTOU race remains between the final Lstat and the caller's Create:
// two concurrent uploads can both pick the same non-existing candidate and
// one will overwrite the other. This is best-effort, matching the residual
// TOCTOU already documented in sandbox.SanitizePath.
//
// It returns an error if the loop is exhausted (every candidate up to
// maxCollisionAttempts is taken) or if a stat fails for a reason other than
// the candidate not existing — those real errors are surfaced rather than
// masked as collisions.
func resolveCollidingName(dir, requested string) (string, error) {
	target := filepath.Join(dir, requested)
	if free, err := slotFree(target); err != nil {
		return "", err
	} else if free {
		return target, nil
	}

	ext := filepath.Ext(requested)
	// A leading-dot name whose only dot is the leading one (e.g. ".gitignore")
	// is a dotfile, not "no name + extension" — keep the dot in the base so the
	// suffix lands after the whole name instead of producing " (1).gitignore".
	if strings.HasPrefix(requested, ".") && strings.Count(requested, ".") == 1 {
		ext = ""
	}
	base := strings.TrimSuffix(requested, ext)

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

// injectNavToken rewrites the same-origin Send<->Files nav hrefs to carry
// ?token=<token>, scoped to the in-page <nav class="header-nav"> block:
//
//   - href="/files" -> href="/files?token=<token>"  (the Files nav link)
//   - href="/"      -> href="/?token=<token>"        (the Send nav link)
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

// RunServer starts the HTTP server, prints the network URL to w, and blocks
// until the server exits.
func RunServer(cfg cli.Config, w io.Writer) error {
	r := NewRouter(cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Fprintf(w, "%s\n", NetworkURL(cfg))

	return http.ListenAndServe(addr, r)
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
