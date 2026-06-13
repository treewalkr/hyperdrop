package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

func TestDownload_SingleFile_ServesAttachment(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/hello.txt", []byte("hello world"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/files/hello.txt?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	cd := resp.Header.Get("Content-Disposition")
	if cd != `attachment; filename="hello.txt"` {
		t.Errorf("Content-Disposition: got %q, want %q", cd, `attachment; filename="hello.txt"`)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("body: got %q, want %q", string(body), "hello world")
	}
}

func TestDownload_PathTraversal_Returns403(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, path := range []string{"../../etc/passwd", "../../../tmp/secret"} {
		t.Run(path, func(t *testing.T) {
			// URL-encode the path to avoid route interpretation issues
			req, _ := http.NewRequest("GET", ts.URL+"/api/files/"+path+"?token=secret123", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want 403. body: %s", resp.StatusCode, b)
			}
		})
	}
}

func TestDownload_NonexistentFile_Returns404(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/files/nope.txt?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestList_NoAuth_Returns401(t *testing.T) {
	cfg := cli.Config{RootDir: t.TempDir(), Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}

func TestDownload_NoAuth_Returns401(t *testing.T) {
	cfg := cli.Config{RootDir: t.TempDir(), Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}
