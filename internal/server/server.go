package server

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/treewalkr/hyperdrop/internal/cli"
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

	// TODO: add token auth middleware — token is generated but never enforced
	r.Get("/", serveFile(assets, "index.html"))
	r.Get("/files", serveFile(assets, "files.html"))

	return r
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
