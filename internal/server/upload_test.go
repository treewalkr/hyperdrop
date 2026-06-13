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
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

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
		Files []struct{ Name string `json:"name"` } `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("response JSON decode: %v", err)
	}
	if len(result.Files) != 2 {
		t.Errorf("files: got %d, want 2", len(result.Files))
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
	if result["error"] != "file too large: max 1024 bytes" {
		t.Errorf("error: got %q", result["error"])
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
