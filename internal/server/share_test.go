package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// doWithShareCookie issues req after attaching the recipient's share cookie,
// simulating a browser that landed on /s/{token} and now requests a file.
func doWithShareCookie(c *http.Client, req *http.Request, shareToken string) (*http.Response, error) {
	req.AddCookie(&http.Cookie{Name: shareCookieName, Value: shareToken})
	return c.Do(req)
}

// shareGet GETs url, optionally attaching a share cookie for scope tests.
func shareGet(t *testing.T, c *http.Client, url string, tokens ...string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if len(tokens) > 0 && tokens[0] != "" {
		req.AddCookie(&http.Cookie{Name: shareCookieName, Value: tokens[0]})
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func shareCreate(t *testing.T, ts *httptest.Server, rel, ttl string) map[string]any {
	t.Helper()
	body := `{"path":"` + rel + `","ttl":"` + ttl + `"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/shares?token=secret123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create share: got %d, want 200. body: %s", resp.StatusCode, b)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestShare_CreateRequiresOwnerToken(t *testing.T) {
	r := NewRouter(cli.Config{RootDir: t.TempDir(), Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/shares", strings.NewReader(`{"path":"a.mp4"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token create: got %d, want 401", resp.StatusCode)
	}
}

func TestShare_LandingSetsCookieAndStreamWorks(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("mp4bytes"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	created := shareCreate(t, ts, "a.mp4", "24h")
	token, _ := created["token"].(string)
	if token == "" {
		t.Fatal("no token in create response")
	}
	if u, _ := created["url"].(string); !strings.HasSuffix(u, "/s/"+token) {
		t.Errorf("url: got %q, want suffix /s/%s", u, token)
	}

	// Recipient lands on the opaque link (no global token). A jar captures the
	// scoped cookie the landing sets; subsequent stream requests carry it.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	landing, err := client.Get(ts.URL + "/s/" + token)
	if err != nil {
		t.Fatal(err)
	}
	landing.Body.Close()
	if landing.StatusCode != http.StatusOK {
		t.Fatalf("landing: got %d, want 200", landing.StatusCode)
	}

	// Stream the shared file with only the share cookie.
	resp := shareGet(t, client, ts.URL+"/api/stream/a.mp4")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("stream shared file: got %d, want 200. body: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "mp4bytes" {
		t.Errorf("body: got %q, want %q", string(body), "mp4bytes")
	}
}

// Scope enforcement: a share cookie must only unlock its bound path. Requesting
// any other file (even a sibling) is an unauthorized request.
func TestShare_CookieScopedToBoundPath(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("A"), 0644)
	os.WriteFile(root+"/b.mp4", []byte("B"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	created := shareCreate(t, ts, "a.mp4", "24h")
	token, _ := created["token"].(string)

	client := &http.Client{}
	// Bound path → 200.
	resp := shareGet(t, client, ts.URL+"/api/stream/a.mp4", token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bound path: got %d, want 200", resp.StatusCode)
	}
	// Sibling path → 401 (scope enforcement).
	resp2 := shareGet(t, client, ts.URL+"/api/stream/b.mp4", token)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("sibling path: got %d, want 401", resp2.StatusCode)
	}
	// Traversal attempt using the share cookie → 401.
	resp3 := shareGet(t, client, ts.URL+"/api/stream/"+url.PathEscape("../../etc/passwd"), token)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("traversal: got %d, want 401", resp3.StatusCode)
	}
}

// A share cookie must not reach owner-only endpoints (list/upload/delete).
func TestShare_CookieDoesNotGrantOwnerEndpoints(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("A"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	created := shareCreate(t, ts, "a.mp4", "never")
	token, _ := created["token"].(string)

	// List endpoint is owner-only.
	req, _ := http.NewRequest("GET", ts.URL+"/api/files", nil)
	resp, err := doWithShareCookie(http.DefaultClient, req, token)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("list with share cookie: got %d, want 401", resp.StatusCode)
	}
	// Delete endpoint stays owner-only.
	req2, _ := http.NewRequest("DELETE", ts.URL+"/api/files/a.mp4", nil)
	resp2, err := doWithShareCookie(http.DefaultClient, req2, token)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("delete with share cookie: got %d, want 401", resp2.StatusCode)
	}
}

func TestShare_RevokeKillsLink(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("A"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	created := shareCreate(t, ts, "a.mp4", "never")
	token, _ := created["token"].(string)

	// Before revoke: stream works.
	resp := shareGet(t, http.DefaultClient, ts.URL+"/api/stream/a.mp4", token)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("before revoke: got %d, want 200", resp.StatusCode)
	}

	// Revoke.
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/shares/"+token+"?token=secret123", nil)
	rev, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rev.Body.Close()
	if rev.StatusCode != http.StatusOK {
		t.Fatalf("revoke: got %d, want 200", rev.StatusCode)
	}

	// After revoke: landing 404s, stream 401s.
	landing, err := http.Get(ts.URL + "/s/" + token)
	if err != nil {
		t.Fatal(err)
	}
	landing.Body.Close()
	if landing.StatusCode != http.StatusNotFound {
		t.Errorf("landing after revoke: got %d, want 404", landing.StatusCode)
	}
	resp2 := shareGet(t, http.DefaultClient, ts.URL+"/api/stream/a.mp4", token)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("stream after revoke: got %d, want 401", resp2.StatusCode)
	}
}

// Regression: the owner's global token still works on the now-dual-authed
// stream/download endpoints (fileAccessAuth accepts the owner credential).
func TestShare_OwnerTokenStillStreams(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("A"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/stream/a.mp4?token=secret123")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("owner stream: got %d, want 200", resp.StatusCode)
	}

	// No credential at all → 401.
	resp2, err := http.Get(ts.URL + "/api/stream/a.mp4")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-cred stream: got %d, want 401", resp2.StatusCode)
	}
}

func TestShare_DirectoryRejected(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(root+"/sub", 0755)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := `{"path":"sub"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/shares?token=secret123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("share directory: got %d, want 400", resp.StatusCode)
	}
}

func TestShare_InvalidTTLRejected(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(root+"/a.mp4", []byte("A"), 0644)
	r := NewRouter(cli.Config{RootDir: root, Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := `{"path":"a.mp4","ttl":"2h"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/shares?token=secret123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid ttl: got %d, want 400", resp.StatusCode)
	}
}
