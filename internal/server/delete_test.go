package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

func deleteRequest(url string) (*http.Request, error) {
	return http.NewRequest("DELETE", url, nil)
}

func TestDelete_SingleFile_RemovesAndReturns200(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/photo.jpg", []byte("img data"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := deleteRequest(ts.URL + "/api/files/photo.jpg?token=secret123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["deleted"] != "photo.jpg" {
		t.Errorf("deleted: got %q, want %q", result["deleted"], "photo.jpg")
	}

	if _, err := os.Stat(root + "/photo.jpg"); !os.IsNotExist(err) {
		t.Fatal("file still exists on disk after delete")
	}
}

func TestDelete_NonexistentFile_Returns404(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := deleteRequest(ts.URL + "/api/files/nope.txt?token=secret123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 404. body: %s", resp.StatusCode, b)
	}
}

func TestDelete_PathTraversal_Returns403(t *testing.T) {
	root := t.TempDir()
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, path := range []string{"../../etc/passwd", "../../../tmp/secret"} {
		t.Run(path, func(t *testing.T) {
			req, _ := deleteRequest(ts.URL + "/api/files/" + path + "?token=secret123")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 403 {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("status: got %d, want 403. body: %s", resp.StatusCode, b)
			}
		})
	}
}

func TestDelete_Directory_RemovesContents(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(root+"/folder", 0755)
	os.WriteFile(root+"/folder/a.txt", []byte("a"), 0644)
	os.WriteFile(root+"/folder/b.txt", []byte("b"), 0644)
	cfg := cli.Config{RootDir: root, Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := deleteRequest(ts.URL + "/api/files/folder?token=secret123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200. body: %s", resp.StatusCode, b)
	}

	if _, err := os.Stat(root + "/folder"); !os.IsNotExist(err) {
		t.Fatal("directory still exists on disk after delete")
	}
}

func TestDelete_NoAuth_Returns401(t *testing.T) {
	cfg := cli.Config{RootDir: t.TempDir(), Token: "secret123"}
	r := NewRouter(cfg)
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := deleteRequest(ts.URL + "/api/files/photo.jpg")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}
}
