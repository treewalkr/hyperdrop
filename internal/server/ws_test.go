package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

const testToken = "tok12345"

// dialWS dials the WS endpoint. Caller closes the conn on success.
func dialWS(t *testing.T, ts *httptest.Server, token string) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := ts.URL + "/api/ws"
	if token != "" {
		url += "?token=" + token
	}
	c, _, err := websocket.Dial(ctx, url, nil)
	return c, err
}

func TestWS_Unauthenticated_Rejected(t *testing.T) {
	r := NewRouter(cli.Config{Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Middleware rejects before upgrade: 401 on a plain GET.
	resp, err := ts.Client().Get(ts.URL + "/api/ws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("status: got %d, want 401", resp.StatusCode)
	}

	// And a real WS dial must fail (no 101).
	if _, err := dialWS(t, ts, ""); err == nil {
		t.Fatal("expected WS dial to fail without token")
	}
}

func TestWS_Authenticated_UpgradeSucceeds(t *testing.T) {
	r := NewRouter(cli.Config{Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("authenticated dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")
}

// uploadOne POSTs a single multipart file to /api/upload.
func uploadOne(t *testing.T, ts *httptest.Server, token, filename, content string) {
	uploadOneTo(t, ts, token, "", filename, content)
}

// uploadOneTo POSTs into the given ?path=<dir> subdirectory ("" = root).
func uploadOneTo(t *testing.T, ts *httptest.Server, token, dir, filename, content string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(content))
	w.Close()
	target := ts.URL + "/api/upload?token=" + token
	if dir != "" {
		target += "&path=" + dir
	}
	req, _ := http.NewRequest("POST", target, body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status: got %d, want 200", resp.StatusCode)
	}
}

// readEvent reads one WS message and decodes it as JSON.
func readEvent(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, raw, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var ev map[string]any
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatalf("ws decode %q: %v", raw, err)
	}
	return ev
}

func TestWS_Upload_BroadcastsFileUploaded(t *testing.T) {
	root := t.TempDir()
	r := NewRouter(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	uploadOne(t, ts, testToken, "photo.jpg", "pic")

	ev := readEvent(t, c)
	if ev["type"] != "file_uploaded" {
		t.Fatalf("type: got %v, want file_uploaded", ev["type"])
	}
	file, ok := ev["file"].(map[string]any)
	if !ok {
		t.Fatalf("missing file payload: %v", ev)
	}
	if file["name"] != "photo.jpg" {
		t.Errorf("file.name: got %v, want photo.jpg", file["name"])
	}
	if file["category"] != "img" {
		t.Errorf("file.category: got %v, want img", file["category"])
	}
}

// deleteOne DELETEs a file via /api/files/{name}.
func deleteOne(t *testing.T, ts *httptest.Server, token, name string) {
	t.Helper()
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/files/"+name+"?token="+token, nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status: got %d, want 200", resp.StatusCode)
	}
}

func TestWS_Delete_BroadcastsFileDeleted(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "gone.txt"), []byte("x"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	deleteOne(t, ts, testToken, "gone.txt")

	ev := readEvent(t, c)
	if ev["type"] != "file_deleted" {
		t.Fatalf("type: got %v, want file_deleted", ev["type"])
	}
	if ev["name"] != "gone.txt" {
		t.Errorf("name: got %v, want gone.txt", ev["name"])
	}
}

func TestWS_Disconnect_RemovesSubscriber(t *testing.T) {
	root := t.TempDir()
	r, hub := newRouterWithHub(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Give the handler a moment to subscribe after the handshake.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && hub.subscribers() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.subscribers(); got != 1 {
		t.Fatalf("after connect: subscribers got %d, want 1", got)
	}

	// Close the client; server must drop the subscriber.
	c.Close(websocket.StatusNormalClosure, "")
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && hub.subscribers() > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.subscribers(); got != 0 {
		t.Errorf("after disconnect: subscribers got %d, want 0 (leak)", got)
	}
}

func TestWS_MultipleClients_AllReceive(t *testing.T) {
	root := t.TempDir()
	r := NewRouter(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	a, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial a: %v", err)
	}
	defer a.Close(websocket.StatusNormalClosure, "")
	b, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial b: %v", err)
	}
	defer b.Close(websocket.StatusNormalClosure, "")

	uploadOne(t, ts, testToken, "shared.txt", "hi")

	for _, c := range []*websocket.Conn{a, b} {
		ev := readEvent(t, c)
		if ev["type"] != "file_uploaded" {
			t.Errorf("client missed event: got %v", ev["type"])
		}
	}
}

// TestWS_Upload_PathScoped pins the "path" field carried on file_uploaded so
// the client can tell which directory the event describes. Root is "".
func TestWS_Upload_PathScoped(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "photos", "2024"), 0755)
	r := NewRouter(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Into a nested directory; the event must name that directory.
	uploadOneTo(t, ts, testToken, "photos/2024", "pic.jpg", "x")

	ev := readEvent(t, c)
	if ev["type"] != "file_uploaded" {
		t.Fatalf("type: got %v, want file_uploaded", ev["type"])
	}
	if got, want := ev["path"], "photos/2024"; got != want {
		t.Errorf("path: got %v, want %q", got, want)
	}

	// Into root; path must be "" so a root view matches.
	uploadOneTo(t, ts, testToken, "", "doc.pdf", "x")
	ev = readEvent(t, c)
	if got, want := ev["path"], ""; got != want {
		t.Errorf("root path: got %v, want %q", got, want)
	}
}

// TestWS_Delete_PathScoped pins the "path" field on file_deleted: it must be
// the deleted file's parent directory, not filepath.Base(dest).
func TestWS_Delete_PathScoped(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "photos", "2024")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "gone.jpg"), []byte("x"), 0644)
	// A same-named file at root, to confirm the client guard keys on path, not name.
	os.WriteFile(filepath.Join(root, "gone.jpg"), []byte("y"), 0644)

	r := NewRouter(cli.Config{RootDir: root, Token: testToken})
	ts := httptest.NewServer(r)
	defer ts.Close()

	c, err := dialWS(t, ts, testToken)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	deleteOne(t, ts, testToken, "photos/2024/gone.jpg")

	ev := readEvent(t, c)
	if ev["type"] != "file_deleted" {
		t.Fatalf("type: got %v, want file_deleted", ev["type"])
	}
	if got, want := ev["name"], "gone.jpg"; got != want {
		t.Errorf("name: got %v, want %q", got, want)
	}
	if got, want := ev["path"], "photos/2024"; got != want {
		t.Errorf("path: got %v, want %q (parent dir, not bare name)", got, want)
	}
}
