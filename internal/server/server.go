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
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/treewalkr/hyperdrop/internal/cli"
	"github.com/treewalkr/hyperdrop/internal/sandbox"
	"github.com/treewalkr/hyperdrop/internal/static"
)

// NewRouter builds a Chi mux with static file routes for the given config.
func NewRouter(cfg cli.Config) chi.Router {
	r := chi.NewRouter()

	var assets fs.FS
	if cfg.Dev {
		assets = os.DirFS("internal/static")
	} else {
		assets = static.Assets
	}

	// Static assets — no auth required
	r.Get("/", serveFile(assets, "index.html"))
	r.Get("/files", serveFile(assets, "files.html"))

	// API routes — token auth required
	r.Route("/api", func(r chi.Router) {
		r.Use(tokenAuth(cfg.Token))
		r.Post("/upload", uploadHandler(cfg))
		r.Get("/files", listHandler(cfg))
		r.Get("/files/*", downloadHandler(cfg))
		r.Delete("/files/*", deleteHandler(cfg))
	})

	return r
}

type uploadResult struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func uploadHandler(cfg cli.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.MaxSize > 0 {
			if r.ContentLength > cfg.MaxSize {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
					"error": fmt.Sprintf("file too large: max %d bytes", cfg.MaxSize),
				})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxSize)
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

			dest, err := sandbox.SanitizePath(cfg.RootDir, filename)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}

			f, err := os.Create(dest)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			n, err := io.Copy(f, part)
			f.Close()
			if err != nil {
				os.Remove(dest)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			saved = append(saved, uploadResult{Name: filename, Size: n})
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
		entries, err := os.ReadDir(cfg.RootDir)
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
// Otherwise returns RFC 6266 filename*=UTF-8'' URL-encoded form.
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

func deleteHandler(cfg cli.Config) http.HandlerFunc {
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
