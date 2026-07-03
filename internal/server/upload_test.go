package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// TestUpload_BroadcastsThrottledProgress is the CI-deterministic gate for issue
// #15: server-reported upload progress over the WebSocket event bus. It uploads
// a large file through httptest while subscribed to the Hub and asserts the
// broadcast path emits throttled progress events plus the terminal completion.
//
// Determinism, no LAN and no sleeps required:
//   - Hub.broadcast is synchronous and queues into the subscriber channel
//     before the upload handler returns, so all events are queued by the time
//     the HTTP response arrives — draining the channel is sleep-free.
//   - copyProgressed reads in 64KB chunks, so the first progress emit cannot
//     jump straight to total; with a >64KB upload the first emit satisfies
//     0 < bytesWritten < total regardless of read granularity.
func TestUpload_BroadcastsThrottledProgress(t *testing.T) {
	const contentSize = 1 << 20 // 1 MiB → 16 × 64KB chunks

	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Subscribe BEFORE the upload so every broadcast is captured.
	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), contentSize)); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	totalBody := int64(body.Len()) // multipart body length = r.ContentLength

	req, err := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// All broadcasts happened synchronously inside the handler; drain what's
	// queued without waiting on a clock.
	var progress []map[string]any
	var sawUploaded bool
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("unmarshal event: %v (raw=%s)", err, raw)
			}
			switch ev["type"] {
			case "upload_progress":
				progress = append(progress, ev)
			case "file_uploaded":
				sawUploaded = true
			}
		default:
			break drain
		}
	}

	// (a) At least one mid-upload progress event with 0 < bytesWritten < total.
	var sawMidUpload bool
	for _, ev := range progress {
		bw, _ := ev["bytesWritten"].(float64)
		total, _ := ev["total"].(float64)
		name, _ := ev["name"].(string)
		if name != "big.bin" {
			t.Errorf("progress event name: got %v, want big.bin", ev["name"])
		}
		if total != float64(totalBody) {
			t.Errorf("progress total: got %v, want %d", ev["total"], totalBody)
		}
		if bw > 0 && bw < total {
			sawMidUpload = true
		}
	}
	if len(progress) == 0 {
		t.Fatal("no upload_progress events emitted")
	}
	if !sawMidUpload {
		t.Errorf("no upload_progress event with 0 < bytesWritten < total; got %+v", progress)
	}

	// Terminal flush: the final byte count is always emitted so a fast upload
	// (whose body lands inside one throttle window) still climbs the bar to
	// ~100% before file_uploaded. The flush carries the full file content size.
	var maxWritten float64
	for _, ev := range progress {
		if bw, _ := ev["bytesWritten"].(float64); bw > maxWritten {
			maxWritten = bw
		}
	}
	if maxWritten != float64(contentSize) {
		t.Errorf("terminal flush missing: max bytesWritten=%v, want %d (full file)", maxWritten, contentSize)
	}

	// (b) Terminal completion event.
	if !sawUploaded {
		t.Error("no terminal file_uploaded event emitted")
	}

	// (c) Throttle ceiling: fewer events than naive per-64KB-chunk flooding.
	ceiling := int64(contentSize) / (64 * 1024)
	if int64(len(progress)) >= ceiling {
		t.Errorf("throttle ceiling violated: got %d progress events, want < %d (totalBytes/64KB)", len(progress), ceiling)
	}
}

func TestUpload_SingleFile_Saves(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// File on disk
	saved, err := os.ReadFile(filepath.Join(root, "hello.txt"))
	if err != nil {
		t.Fatalf("file not saved: %v", err)
	}
	if string(saved) != "hello world" {
		t.Errorf("file content: got %q, want %q", string(saved), "hello world")
	}

	// Response JSON
	var result struct {
		Files []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("response JSON decode: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files: got %d, want 1", len(result.Files))
	}
	if result.Files[0].Name != "hello.txt" {
		t.Errorf("file name: got %q, want %q", result.Files[0].Name, "hello.txt")
	}
	if result.Files[0].Size != 11 {
		t.Errorf("file size: got %d, want 11", result.Files[0].Size)
	}
}

func TestUpload_MultipleFiles_AllSaved(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, name := range []string{"a.txt", "b.txt"} {
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("content of " + name)); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// Both files on disk
	for _, name := range []string{"a.txt", "b.txt"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("file %s not saved: %v", name, err)
		}
		want := "content of " + name
		if string(data) != want {
			t.Errorf("file %s: got %q, want %q", name, string(data), want)
		}
	}

	var result struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("response JSON decode: %v", err)
	}
	if len(result.Files) != 2 {
		t.Errorf("files: got %d, want 2", len(result.Files))
	}
}

func TestUpload_PathQuery_SavesToSubdir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(root+"/photos/vacation", 0755)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "sunset.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("pic"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path=photos/vacation&token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// Saved inside the subdir, not at root.
	saved, err := os.ReadFile(filepath.Join(root, "photos", "vacation", "sunset.jpg"))
	if err != nil {
		t.Fatalf("file not saved to subdir: %v", err)
	}
	if string(saved) != "pic" {
		t.Errorf("content: got %q, want %q", string(saved), "pic")
	}
	if _, err := os.Stat(filepath.Join(root, "sunset.jpg")); !os.IsNotExist(err) {
		t.Errorf("file must not land at root: %v", err)
	}
}

func TestUpload_PathQuery_TraversalRejected(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "evil.txt")
	part.Write([]byte("pwned"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path=../../etc&token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("status: got %d, want 403. body: %s", resp.StatusCode, b)
	}
}

// TestUpload_DotFilename_Rejected is the regression gate for issue #33: a
// multipart upload with filename="." cleans to the upload root itself, and
// createUniqueFile then runs against the root's PARENT directory — writing a
// stray file outside the sandbox. SanitizePath now rejects any non-empty path
// that resolves to the root (covering both "." and e.g. "foo/..") with an
// error, so uploadHandler returns HTTP 400 and nothing lands outside the root.
func TestUpload_DotFilename_Rejected(t *testing.T) {
	for _, filename := range []string{".", "foo/.."} {
		t.Run(filename, func(t *testing.T) {
			root := t.TempDir()
			cfg := cli.Config{RootDir: root, Token: "secret123"}
			r := NewRouter(cfg)
			ts := httptest.NewServer(r)
			defer ts.Close()

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", filename)
			if err != nil {
				t.Fatal(err)
			}
			part.Write([]byte("pwned"))
			writer.Close()

			req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status: got %d, want 400. body: %s", resp.StatusCode, b)
			}

			// Nothing may escape the root: no stray file appears in the root's
			// parent named after the root's basename (the pre-fix escape). The
			// root dir itself sits in the parent under that name, so only flag
			// non-directory entries.
			parent := filepath.Dir(root)
			rootBase := filepath.Base(root)
			if entries, err := os.ReadDir(parent); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					if e.Name() == rootBase || strings.HasPrefix(e.Name(), rootBase+" ") {
						t.Errorf("stray file escaped root into parent: %s", filepath.Join(parent, e.Name()))
					}
				}
			}
		})
	}
}

func TestUpload_PathTraversal_Rejected(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, filename := range []string{"../../../etc/passwd", "/etc/passwd"} {
		t.Run(filename, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", filename)
			if err != nil {
				t.Fatal(err)
			}
			part.Write([]byte("pwned"))
			writer.Close()

			req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want 400. body: %s", resp.StatusCode, b)
			}
		})
	}
}

func TestUpload_MissingContentType_Returns400(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", bytes.NewBufferString("not multipart"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "expected multipart/form-data" {
		t.Errorf("error: got %q, want %q", result["error"], "expected multipart/form-data")
	}
}

func TestUpload_MaxSizeExceeded_Returns413(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123", MaxSize: 1024} // 1KB limit
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(make([]byte, 2048)) // 2KB — exceeds limit
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, want 413", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "file too large: max 1.0 KB" {
		t.Errorf("error: got %q, want %q", result["error"], "file too large: max 1.0 KB")
	}
}

func TestUpload_MaxSize_AllowsWithinLimit(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123", MaxSize: 1024}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "small.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("ok")) // well under the 1KB cap
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestUpload_MaxSize_DefaultUnlimited(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"} // MaxSize=0 → unlimited
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(make([]byte, 10000)) // would exceed any reasonable cap
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200 (default unlimited)", resp.StatusCode)
	}
}

func TestUpload_MaxSizeChunked_Returns413(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123", MaxSize: 256}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Use a pipe so Content-Length is unknown (simulates chunked transfer).
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		part, _ := writer.CreateFormFile("file", "big.txt")
		// Write more than MaxSize — MaxBytesReader should cut it off.
		part.Write(make([]byte, 4096))
		writer.Close()
	}()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", pr)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// Explicitly unset Content-Length to simulate chunked.
	req.ContentLength = -1

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// MaxBytesReader triggers a 413 or the multipart read fails.
	// The handler should not return 200.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("status: got 200, expected non-200 (chunked bypass should be caught)")
	}
}

func TestUpload_NoAuth_Returns401(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("data"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}

// TestUpload_NestedFilename_CreatesDirs covers directory uploads (issue #30):
// a part whose filename carries a relative path ("a/b/c.txt") must land at the
// nested location and auto-create the intermediate directories.
func TestUpload_NestedFilename_CreatesDirs(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "vacation/sub/sunset.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("pic"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	saved, err := os.ReadFile(filepath.Join(root, "vacation", "sub", "sunset.jpg"))
	if err != nil {
		t.Fatalf("nested file not saved: %v", err)
	}
	if string(saved) != "pic" {
		t.Errorf("content: got %q, want %q", string(saved), "pic")
	}
	// Intermediate directories must have been created.
	for _, seg := range []string{"vacation", filepath.Join("vacation", "sub")} {
		if info, err := os.Stat(filepath.Join(root, seg)); err != nil || !info.IsDir() {
			t.Errorf("intermediate dir %q not created: %v", seg, err)
		}
	}
	// Nested upload under an existing ?path= base combines both segments.
	// (currentPath is always an existing dir in the UI; the frontend only
	// navigates into loaded folders, so the base exists before upload.)
	t.Run("with path base", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "gallery"), 0o755)
		cfg := cli.Config{RootDir: root, Token: "secret123"}
		r := NewRouter(cfg)
		ts := httptest.NewServer(r)
		defer ts.Close()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "inner/deep.txt")
		part.Write([]byte("x"))
		writer.Close()

		req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path=gallery&token=secret123", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
		}
		if _, err := os.ReadFile(filepath.Join(root, "gallery", "inner", "deep.txt")); err != nil {
			t.Fatalf("nested-under-base file not saved: %v", err)
		}
	})
}

// TestUpload_NestedFilename_TraversalRejected ensures a relative path in the
// filename still cannot escape the root: SanitizePath must reject "../".
func TestUpload_NestedFilename_TraversalRejected(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, filename := range []string{"../evil.txt", "sub/../../evil.txt", "ok/../../../evil.txt"} {
		t.Run(filename, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", filename)
			part.Write([]byte("pwned"))
			writer.Close()

			req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want 400. body: %s", resp.StatusCode, b)
			}
		})
	}

	// And nothing escaped the root on disk.
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(err) {
		t.Errorf("file escaped root: %v", err)
	}
}

// TestUpload_NestedFilename_BroadcastNormalized verifies the file_uploaded
// event for a nested upload carries name=basename and path=the file's actual
// directory, so the Files-page live update places the row in the right view.
func TestUpload_NestedFilename_BroadcastNormalized(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "vacation/sub/sunset.jpg")
	part.Write([]byte("pic"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	var uploaded map[string]any
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ev["type"] == "file_uploaded" {
				uploaded = ev
				break drain
			}
		default:
			break drain
		}
	}
	if uploaded == nil {
		t.Fatal("no file_uploaded event")
	}
	if got := uploaded["path"]; got != "vacation/sub" {
		t.Errorf("event path: got %v, want vacation/sub", got)
	}
	file, _ := uploaded["file"].(map[string]any)
	if file == nil || file["name"] != "sunset.jpg" {
		t.Errorf("event file.name: got %v, want sunset.jpg", uploaded["file"])
	}
}

// resolveCollidingName is the policy seam for issue #16: when a requested
// filename already exists in the target directory, it derives a non-colliding
// name (name (1).ext, name (2).ext, …) instead of silently overwriting. These
// tests pin the naming policy directly without the multipart plumbing.

// TestResolveCollidingName_FirstWriteNoCollision: the requested name does not
// exist, so it is returned unchanged.
func TestResolveCollidingName_FirstWriteNoCollision(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveCollidingName(dir, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "a.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_SingleCollision: the requested name exists, so the
// next available name is "name (1).ext".
func TestResolveCollidingName_SingleCollision(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)

	got, err := resolveCollidingName(dir, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "a (1).txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_RepeatedCollision: a.txt plus a (1).txt and
// a (2).txt all exist, so the resolver must walk past the gap to a (3).txt.
// This pins the per-candidate re-check (each suffix stat'd against the live
// directory) rather than a single pre-scan that could stop at the first slot.
func TestResolveCollidingName_RepeatedCollision(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "a (1).txt", "a (2).txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := resolveCollidingName(dir, "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "a (3).txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// The suffix can never change the directory of the result — it stays in
	// the same dir as the request (no sandbox escape via the suffix).
	if gotDir := filepath.Dir(got); gotDir != dir {
		t.Errorf("result escaped dir: got %q, want %q", gotDir, dir)
	}
}

// TestResolveCollidingName_NoExtension: a name with no extension still gets the
// " (N)" suffix rather than a trailing dot.
func TestResolveCollidingName_NoExtension(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644)

	got, err := resolveCollidingName(dir, "README")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "README (1)")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_Dotfile: a leading-dot name whose only dot is the
// leading one is treated as extension-less, so the suffix goes after the whole
// name (".gitignore (1)") rather than producing a leading-space " (1).gitignore".
func TestResolveCollidingName_Dotfile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("x"), 0o644)

	got, err := resolveCollidingName(dir, ".gitignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, ".gitignore (1)")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_MultiDotDotfile: a leading-dot name with more than
// one dot (e.g. ".env.local") is still treated as extension-less — the suffix
// lands after the whole name (".env.local (1)") rather than splitting it into
// ".env" + ".local" and producing the mangled ".env (1).local".
func TestResolveCollidingName_MultiDotDotfile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env.local"), []byte("x"), 0o644)

	got, err := resolveCollidingName(dir, ".env.local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, ".env.local (1)")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_ContinuesExistingCounter: when the requested name
// already carries a " (N)" counter and collides, the sequence continues rather
// than stacking a second counter — "report (1).pdf" (with no "report.pdf")
// resolves to "report (2).pdf", not "report (1) (1).pdf".
func TestResolveCollidingName_ContinuesExistingCounter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "report (1).pdf"), []byte("x"), 0o644)

	got, err := resolveCollidingName(dir, "report (1).pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "report (2).pdf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_PreservesLiteralCounterWhenFree: a requested name
// ending in " (N)" that does NOT collide is returned unchanged — the counter
// is only stripped/continued when a collision forces a rename.
func TestResolveCollidingName_PreservesLiteralCounterWhenFree(t *testing.T) {
	dir := t.TempDir()

	got, err := resolveCollidingName(dir, "report (1).pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "report (1).pdf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveCollidingName_ExhaustionReturnsError: when every candidate up to
// the attempt cap is taken, the resolver gives up with an error instead of
// looping forever or silently overwriting.
func TestResolveCollidingName_ExhaustionReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Fill every slot the resolver will try: the requested name + cap suffixes.
	maxCollisionAttempts = 3
	t.Cleanup(func() { maxCollisionAttempts = 10000 })
	for _, name := range []string{"a.txt", "a (1).txt", "a (2).txt", "a (3).txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := resolveCollidingName(dir, "a.txt")
	if err == nil {
		t.Fatal("expected exhaustion error, got nil")
	}
}

// TestUpload_NameCollision_RenamesAndReports is the handler-seam gate for issue
// #16: uploading a name that already exists must NOT overwrite — it lands at
// "a (1).txt", and that actually-written name is what the response and the
// file_uploaded broadcast report (so the UI and other clients see the new file
// under its real name).
func TestUpload_NameCollision_RenamesAndReports(t *testing.T) {
	root := t.TempDir()
	// Pre-existing file at the same name — the thing we must not clobber.
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("new"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// (a) Original untouched, new content landed at "a (1).txt".
	if got, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(got) != "original" {
		t.Errorf("original overwritten: got %q, want %q", string(got), "original")
	}
	gotNew, err := os.ReadFile(filepath.Join(root, "a (1).txt"))
	if err != nil {
		t.Fatalf("renamed file not saved: %v", err)
	}
	if string(gotNew) != "new" {
		t.Errorf("renamed content: got %q, want %q", string(gotNew), "new")
	}

	// (b) Response reports the actually-written name.
	var result struct {
		Files []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("response JSON decode: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "a (1).txt" {
		t.Errorf("response name: got %+v, want a (1).txt", result.Files)
	}

	// (c) Broadcast reports the actually-written name.
	var uploaded map[string]any
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ev["type"] == "file_uploaded" {
				uploaded = ev
				break drain
			}
		default:
			break drain
		}
	}
	if uploaded == nil {
		t.Fatal("no file_uploaded event")
	}
	file, _ := uploaded["file"].(map[string]any)
	if file == nil || file["name"] != "a (1).txt" {
		t.Errorf("broadcast file.name: got %v, want a (1).txt", uploaded["file"])
	}
}

// TestUpload_NameCollision_RepeatedSlot walks the resolver past existing
// "a (1).txt"/"a (2).txt": the new upload lands at "a (3).txt" and that exact
// name is broadcast and returned (the "report the name actually written" AC for
// repeated collisions).
func TestUpload_NameCollision_RepeatedSlot(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "a (1).txt", "a (2).txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "a.txt")
	part.Write([]byte("new"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	if _, err := os.ReadFile(filepath.Join(root, "a (3).txt")); err != nil {
		t.Errorf("expected a (3).txt on disk: %v", err)
	}

	var result struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Files) != 1 || result.Files[0].Name != "a (3).txt" {
		t.Errorf("response name: got %+v, want a (3).txt", result.Files)
	}

	var uploaded map[string]any
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			json.Unmarshal(raw, &ev)
			if ev["type"] == "file_uploaded" {
				uploaded = ev
				break drain
			}
		default:
			break drain
		}
	}
	file, _ := uploaded["file"].(map[string]any)
	if file == nil || file["name"] != "a (3).txt" {
		t.Errorf("broadcast file.name: got %v, want a (3).txt", uploaded["file"])
	}
}

// TestUpload_NameCollision_NestedBroadcastPath covers a collision on a nested
// upload: filename "vacation/sub/a.txt" already exists on disk, so the upload
// lands at "a (1).txt" inside that subdirectory, and the file_uploaded event
// carries the unchanged directory (path=vacation/sub) with the renamed basename
// so the Files-page live update places the row in the open view.
func TestUpload_NameCollision_NestedBroadcastPath(t *testing.T) {
	root := t.TempDir()
	// Pre-existing nested file — the collision target.
	nested := filepath.Join(root, "vacation", "sub", "a.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "vacation/sub/a.txt")
	part.Write([]byte("new"))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// Original untouched; new content at the nested renamed slot.
	if got, _ := os.ReadFile(nested); string(got) != "original" {
		t.Errorf("original overwritten: got %q, want %q", string(got), "original")
	}
	if got, err := os.ReadFile(filepath.Join(root, "vacation", "sub", "a (1).txt")); err != nil || string(got) != "new" {
		t.Errorf("nested renamed slot: got %q, %v, want new", got, err)
	}

	// Response carries the full relative renamed name.
	var result struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Files) != 1 || result.Files[0].Name != "vacation/sub/a (1).txt" {
		t.Errorf("response name: got %+v, want vacation/sub/a (1).txt", result.Files)
	}

	// Broadcast: unchanged directory + renamed basename.
	var uploaded map[string]any
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			json.Unmarshal(raw, &ev)
			if ev["type"] == "file_uploaded" {
				uploaded = ev
				break drain
			}
		default:
			break drain
		}
	}
	if uploaded == nil {
		t.Fatal("no file_uploaded event")
	}
	if got := uploaded["path"]; got != "vacation/sub" {
		t.Errorf("event path: got %v, want vacation/sub", got)
	}
	file, _ := uploaded["file"].(map[string]any)
	if file == nil || file["name"] != "a (1).txt" {
		t.Errorf("broadcast file.name: got %v, want a (1).txt", uploaded["file"])
	}
}

// TestUpload_NameCollision_ProgressUsesRequestedName pins the fix for a
// regression where the upload_progress event's name was switched to the renamed
// writtenName: that broke the client's card matcher (relPath||name === data.name)
// for any upload into a subdirectory and for collision renames, since the card
// still carries the REQUESTED name mid-transfer. Progress must carry the
// requested name; the renamed name reaches the client only at completion (via
// the response / file_uploaded). Here a collision upload must emit progress
// events named "a.txt" — not "a (1).txt" — while the response still reports the
// renamed name.
func TestUpload_NameCollision_ProgressUsesRequestedName(t *testing.T) {
	const contentSize = 1 << 20 // >64KB so at least one mid-upload progress emit fires

	root := t.TempDir()
	// Pre-existing file — forces a rename to "a (1).txt".
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r, hub := newRouterWithHub(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	sub := hub.subscribe()
	defer hub.unsubscribe(sub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "a.txt")
	part.Write(bytes.Repeat([]byte("x"), contentSize))
	writer.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?token=secret123", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	// Drain progress + completion events queued synchronously by the handler.
	var progress []map[string]any
	var renamed string
drain:
	for {
		select {
		case raw := <-sub.out:
			var ev map[string]any
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			switch ev["type"] {
			case "upload_progress":
				progress = append(progress, ev)
			case "file_uploaded":
				f, _ := ev["file"].(map[string]any)
				renamed, _ = f["name"].(string)
			}
		default:
			break drain
		}
	}

	if len(progress) == 0 {
		t.Fatal("no upload_progress events emitted")
	}
	for _, ev := range progress {
		if name, _ := ev["name"].(string); name != "a.txt" {
			t.Errorf("progress event name: got %q, want %q (the requested name, not the renamed one)", name, "a.txt")
		}
	}
	// The renamed name still reaches completion signals.
	if renamed != "a (1).txt" {
		t.Errorf("file_uploaded name: got %q, want %q", renamed, "a (1).txt")
	}
	var result struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Files) != 1 || result.Files[0].Name != "a (1).txt" {
		t.Errorf("response name: got %+v, want a (1).txt", result.Files)
	}
}
