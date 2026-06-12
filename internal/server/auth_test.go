package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

func TestStaticAssets_NoAuthRequired(t *testing.T) {
	r := NewRouter(cli.Config{Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, path := range []string{"/", "/files"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: unexpected error: %v", path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: got %d, want %d (static assets should bypass auth)", path, resp.StatusCode, http.StatusOK)
		}
	}
}

func TestAPI_WrongToken_Returns401(t *testing.T) {
	r := NewRouter(cli.Config{Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files?token=wrongtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("response is not valid JSON: %s", body)
	}
	if result["error"] != "unauthorized" {
		t.Errorf(`error: got %q, want "unauthorized"`, result["error"])
	}
}

func TestAPI_ValidSessionCookie_Returns200(t *testing.T) {
	r := NewRouter(cli.Config{Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/api/files", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{
		Name:  "hyperdrop_session",
		Value: "secret123",
	})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAPI_ValidTokenQuery_SetsSessionCookie(t *testing.T) {
	r := NewRouter(cli.Config{Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files?token=secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "hyperdrop_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected hyperdrop_session cookie to be set")
	}
	if sessionCookie.Value != "secret123" {
		t.Errorf("cookie value: got %q, want %q", sessionCookie.Value, "secret123")
	}
	if !sessionCookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite: got %d, want %d", sessionCookie.SameSite, http.SameSiteLaxMode)
	}
}

func TestAPI_NoTokenNoCookie_Returns401(t *testing.T) {
	r := NewRouter(cli.Config{Token: "secret123"})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("response is not valid JSON: %s", body)
	}
	if result["error"] != "unauthorized" {
		t.Errorf(`error: got %q, want "unauthorized"`, result["error"])
	}
}
