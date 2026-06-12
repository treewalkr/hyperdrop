package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

func TestGET_Index(t *testing.T) {
	r := NewRouter(cli.Config{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Error("response body should contain HTML")
	}
	if !strings.Contains(string(body), "HyperDrop") {
		t.Error("response body should contain 'HyperDrop'")
	}
}

func TestGET_Files(t *testing.T) {
	r := NewRouter(cli.Config{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html") {
		t.Error("response body should contain HTML")
	}
	if !strings.Contains(string(body), "Files") {
		t.Error("response body should contain 'Files'")
	}
}

func TestNetworkURL(t *testing.T) {
	cfg := cli.Config{
		Host:  "0.0.0.0",
		Port:  8080,
		Token: "a7x9k2m4",
	}

	url := NetworkURL(cfg)

	if !strings.Contains(url, "8080") {
		t.Errorf("URL should contain port: got %q", url)
	}
	if !strings.Contains(url, "token=a7x9k2m4") {
		t.Errorf("URL should contain token: got %q", url)
	}
	if !strings.HasPrefix(url, "http://") {
		t.Errorf("URL should start with http://: got %q", url)
	}
}

func TestGET_DevMode_ServesFromDisk(t *testing.T) {
	// NOTE: os.Chdir mutates process cwd — not safe with t.Parallel().
	// Create a temp directory matching the internal/static layout.
	dir := t.TempDir()
	staticDir := filepath.Join(dir, "internal", "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		t.Fatal(err)
	}

	devContent := "<h1>DEV MODE</h1>"
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(devContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "files.html"), []byte("<h1>DEV FILES</h1>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change working directory so os.DirFS("internal/static") resolves.
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg := cli.Config{Dev: true}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "DEV MODE") {
		t.Errorf("dev mode should serve from disk, got: %s", string(body))
	}
}

func TestNetworkURL_Localhost(t *testing.T) {
	cfg := cli.Config{
		Host:  "127.0.0.1",
		Port:  3000,
		Token: "abc12345",
	}

	url := NetworkURL(cfg)

	expected := "http://127.0.0.1:3000/?token=abc12345"
	if url != expected {
		t.Errorf("got %q, want %q", url, expected)
	}
}

func TestNetworkURL_ZeroHostResolvesLAN(t *testing.T) {
	cfg := cli.Config{
		Host:  "0.0.0.0",
		Port:  8080,
		Token: "a7x9k2m4",
	}

	url := NetworkURL(cfg)

	// Should NOT contain 0.0.0.0 — should resolve to a real LAN IP.
	if strings.Contains(url, "0.0.0.0") {
		t.Errorf("URL should resolve 0.0.0.0 to LAN IP: got %q", url)
	}
	if !strings.Contains(url, fmt.Sprintf(":%d", cfg.Port)) {
		t.Errorf("URL should contain port %d: got %q", cfg.Port, url)
	}
}
