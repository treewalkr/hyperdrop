package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

func TestList_EmptyDir_ReturnsEmptyArray(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/files?token=secret123", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}

	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("files: got %d entries, want 0", len(result))
	}
}

func TestList_PopulatedDir_ReturnsEntries(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/hello.txt", []byte("hello world"), 0644)
	os.Mkdir(root+"/subdir", 0755)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/files?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var entries []fileEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(entries))
	}

	// Build lookup by name
	byName := map[string]*fileEntry{}
	for i := range entries {
		byName[entries[i].Name] = &entries[i]
	}

	// Check file entry
	f := byName["hello.txt"]
	if f == nil {
		t.Fatal("hello.txt not found")
	}
	if f.Size != 11 {
		t.Errorf("hello.txt size: got %d, want 11", f.Size)
	}
	if f.IsDir {
		t.Error("hello.txt IsDir: got true, want false")
	}
	if _, err := time.Parse(time.RFC3339, f.ModTime); err != nil {
		t.Errorf("hello.txt mod_time not ISO 8601: %q, err: %v", f.ModTime, err)
	}

	// Check dir entry
	d := byName["subdir"]
	if d == nil {
		t.Fatal("subdir not found")
	}
	if !d.IsDir {
		t.Error("subdir IsDir: got false, want true")
	}
}

func TestList_SubdirQuery_ReturnsSubdirContents(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(root+"/photos/vacation", 0755)
	os.WriteFile(root+"/photos/vacation/sunset.jpg", []byte("x"), 0644)
	os.WriteFile(root+"/photos/keep.txt", []byte("y"), 0644) // sibling dir, must NOT appear

	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/files?path=photos/vacation&token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	var entries []fileEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries: got %d, want 1 (only vacation contents)", len(entries))
	}
	if entries[0].Name != "sunset.jpg" {
		t.Errorf("entry name: got %q, want %q", entries[0].Name, "sunset.jpg")
	}
}

func TestList_PathTraversalOnSubdir_Returns403(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/inside.txt", []byte("x"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, q := range []string{"../etc", "/etc", "sub/../../.."} {
		t.Run(q, func(t *testing.T) {
			req, _ := http.NewRequest("GET", ts.URL+"/api/files?path="+url.QueryEscape(q)+"&token=secret123", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("status for path=%q: got %d, want 403. body: %s", q, resp.StatusCode, b)
			}
		})
	}
}

func TestList_CategoryMapping(t *testing.T) {
	root := t.TempDir()
	// Create files covering each category
	for _, name := range []string{
		"report.pdf",  // doc
		"photo.jpg",   // img
		"clip.mp4",    // vid
		"archive.zip", // zip
		"unknown.xyz", // file
		"noext",       // file (no extension)
		"PHOTO.PNG",   // img (uppercase)
	} {
		os.WriteFile(root+"/"+name, []byte("x"), 0644)
	}

	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/files?token=secret123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var entries []fileEntry
	json.NewDecoder(resp.Body).Decode(&entries)

	want := map[string]string{
		"report.pdf":  "doc",
		"photo.jpg":   "img",
		"clip.mp4":    "vid",
		"archive.zip": "zip",
		"unknown.xyz": "file",
		"noext":       "file",
		"PHOTO.PNG":   "img",
	}

	byName := map[string]string{}
	for _, e := range entries {
		byName[e.Name] = e.Category
	}

	for name, expected := range want {
		got := byName[name]
		if got != expected {
			t.Errorf("%s: got %q, want %q", name, got, expected)
		}
	}
}
