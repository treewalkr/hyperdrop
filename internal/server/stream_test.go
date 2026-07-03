package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// TestStream_SingleFile_ServesInline serves bytes WITHOUT Content-Disposition
// so a <video src> can play them inline.
func TestStream_SingleFile_ServesInline(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/clip.mp4", []byte("not real video bytes"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/stream/clip.mp4?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		t.Errorf("Content-Disposition: got %q, want empty (inline)", cd)
	}
	if ar := resp.Header.Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges: got %q, want %q", ar, "bytes")
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "video/") {
		t.Errorf("Content-Type: got %q, want video/*", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "not real video bytes" {
		t.Errorf("body: got %q, want %q", string(body), "not real video bytes")
	}
}

// TestStream_RangeRequest_Returns206 verifies seek support — http.ServeContent
// honors Range so a <video> element can scrub the timeline.
func TestStream_RangeRequest_Returns206(t *testing.T) {
	root := t.TempDir()
	payload := strings.Repeat("x", 4096)
	os.WriteFile(root+"/clip.mp4", []byte(payload), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/stream/clip.mp4?token=secret123", nil)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status: got %d, want 206", resp.StatusCode)
	}
	if cr := resp.Header.Get("Content-Range"); !strings.HasPrefix(cr, "bytes 0-1023/4096") {
		t.Errorf("Content-Range: got %q, want bytes 0-1023/4096", cr)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 1024 {
		t.Errorf("body length: got %d, want 1024", len(body))
	}
}

func TestStream_NonexistentFile_Returns404(t *testing.T) {
	cfg := cli.Config{RootDir: t.TempDir(), Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/stream/nope.mp4?token=secret123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestStream_PathTraversal_Returns403(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, path := range []string{"../../etc/passwd", "../../../tmp/secret"} {
		t.Run(path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", ts.URL+"/api/stream/"+path+"?token=secret123", nil)
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

func TestStream_NoAuth_Returns401(t *testing.T) {
	cfg := cli.Config{RootDir: t.TempDir(), Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/stream/clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}

// TestStream_HEAD_ReturnsMetadata locks in the probe path used by the player
// page (fetch HEAD to read size/type without the body). chi does not auto-alias
// HEAD onto GET routes, so the route registers HEAD explicitly.
func TestStream_HEAD_ReturnsMetadata(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/clip.mp4", []byte("payload-bytes-here"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("HEAD", ts.URL+"/api/stream/clip.mp4?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		t.Errorf("Content-Disposition: got %q, want empty", cd)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Errorf("Content-Length: missing on HEAD probe")
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("HEAD body: got %d bytes, want 0", len(body))
	}
}

// TestPlayable covers the gating predicate mirrored on the client.
func TestPlayable(t *testing.T) {
	cases := map[string]bool{
		"movie.mp4": true,
		"clip.MP4":  true, // case-insensitive
		"clip.m4v":  true,
		"clip.webm": true,
		"clip.ogv":  true,
		"clip.mov":  true,
		"movie.mkv": false, // tagged vid by categorize but not browser-decodable
		"movie.avi": false,
		"movie.flv": false,
		"song.mp3":  false,
		"noext":     false,
	}
	for name, want := range cases {
		if got := playable(name); got != want {
			t.Errorf("playable(%q) = %v, want %v", name, got, want)
		}
	}
}
