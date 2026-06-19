package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// TestNavLinks_CarryTokenOnAuthenticatedPage verifies that when a page is
// requested with a valid ?token=, the server injects that token onto the
// same-origin Send<->Files nav hrefs. This is the fix for issue #17: a cold
// first click from the authenticated Send page to Files must carry the token
// so the resulting page request sets the session cookie before the file-list
// API call fires (otherwise the API returns 401 and a "401" toast appears).
func TestNavLinks_CarryTokenOnAuthenticatedPage(t *testing.T) {
	const token = "secret123"
	r := NewRouter(cli.Config{Token: token})
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, page := range []string{"/", "/files"} {
		resp, err := http.Get(ts.URL + page + "?token=" + token)
		if err != nil {
			t.Fatalf("GET %s: unexpected error: %v", page, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status got %d, want %d", page, resp.StatusCode, http.StatusOK)
		}

		pageStr := string(body)
		// The opposite-direction nav link must carry the token so the next
		// page load (and its first API call) is authenticated and sets the
		// cookie. Brand href="/" deliberately stays token-less.
		if !strings.Contains(pageStr, "/files?token="+token) {
			t.Errorf("GET %s: nav must link to /files?token=<token>, body did not contain it", page)
		}
		if !strings.Contains(pageStr, "/?token="+token) {
			t.Errorf("GET %s: nav must link to /?token=<token>, body did not contain it", page)
		}
	}
}

// TestNavLinks_AuthenticatedPageSetsCookie confirms the token-bearing page
// request sets the session cookie, so subsequent API calls (made with no
// ?token= in the URL) authenticate via the cookie — the persistent credential.
func TestNavLinks_AuthenticatedPageSetsCookie(t *testing.T) {
	const token = "secret123"
	r := NewRouter(cli.Config{RootDir: t.TempDir(), Token: token})
	ts := httptest.NewServer(r)
	defer ts.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	// Cold visit to /files carrying the token (simulates clicking the
	// token-bearing Files nav link from an authenticated Send page).
	resp, err := client.Get(ts.URL + "/files?token=" + token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got bool
	for _, c := range jar.Cookies(resp.Request.URL) {
		if c.Name == "hyperdrop_session" && c.Value == token {
			got = true
		}
	}
	if !got {
		t.Fatal("expected hyperdrop_session cookie to be set by token-bearing page request")
	}

	// The cookie must now authenticate the API with no ?token= in the URL —
	// this is the no-401-on-cold-load guarantee.
	apiResp, err := client.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer apiResp.Body.Close()
	if apiResp.StatusCode != http.StatusOK {
		t.Fatalf("api/files with cookie: got %d, want %d (cold-load 401 regression)", apiResp.StatusCode, http.StatusOK)
	}
}

// TestNavLinks_UnauthenticatedPageLeaksNoToken is the security slice: a page
// request with no token and no cookie must not contain any token-bearing href
// (the nav stays plain / and /files). Guards against leaking a token into a
// response to a request that never proved knowledge of it.
func TestNavLinks_UnauthenticatedPageLeaksNoToken(t *testing.T) {
	const token = "secret123"
	r := NewRouter(cli.Config{Token: token})
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, page := range []string{"/", "/files"} {
		resp, err := http.Get(ts.URL + page)
		if err != nil {
			t.Fatalf("GET %s: unexpected error: %v", page, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: static pages must still serve without auth, got %d", page, resp.StatusCode)
		}

		if strings.Contains(string(body), token) {
			t.Errorf("GET %s: unauthenticated page response must not contain the token", page)
		}
		// The leak we guard against is a token-bearing nav href. A literal
		// "?token=" inside a JS comment is not a leak; an href carrying it is.
		if strings.Contains(string(body), `href="/files?token=`) ||
			strings.Contains(string(body), `href="/?token=`) {
			t.Errorf("GET %s: unauthenticated page response must not inject token into nav hrefs", page)
		}
	}
}

// TestNavLinks_WrongTokenDoesNotInject confirms that an unknown ?token= value
// does not cause token injection into nav hrefs (only a valid token is trusted
// to be echoed into same-origin hrefs).
func TestNavLinks_WrongTokenDoesNotInject(t *testing.T) {
	const token = "secret123"
	r := NewRouter(cli.Config{Token: token})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/files?token=wrongtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(body), "?token=") {
		t.Errorf("wrong token must not be injected into nav hrefs; body contained ?token=")
	}
}

// TestNavLinks_BrandLinkStaysTokenless pins the invariant that the brand
// anchor (href="/" class="brand"), which lives outside the in-page nav, never
// receives the token — even though it shares its href value with the Send nav
// link. This guards against a regression to a catch-all-rewrite that depended
// on the brand link's exact attribute serialization to skip it; here injection
// is scoped to the nav block, so the brand anchor is structurally excluded.
func TestNavLinks_BrandLinkStaysTokenless(t *testing.T) {
	const token = "secret123"
	r := NewRouter(cli.Config{Token: token})
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, page := range []string{"/", "/files"} {
		resp, err := http.Get(ts.URL + page + "?token=" + token)
		if err != nil {
			t.Fatalf("GET %s: unexpected error: %v", page, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// The brand anchor must remain exactly token-less.
		if strings.Contains(string(body), `class="brand"`+token) ||
			strings.Contains(string(body), `href="/?token=`+token+`" class="brand"`) {
			t.Errorf("GET %s: brand anchor must stay token-less", page)
		}
		if !strings.Contains(string(body), `href="/" class="brand"`) {
			t.Errorf("GET %s: brand anchor href must remain plain href=\"/\" class=\"brand\"", page)
		}
	}
}
