package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// TestSendPage_AuroraMarkers guards the Send page (index.html) production UI:
// the Aurora design system must be present, the prototype variant switcher and
// the non-default variants must be stripped. This is a smoke test over the
// public HTTP boundary; visual fidelity is verified by human review.
func TestSendPage_AuroraMarkers(t *testing.T) {
	r := NewRouter(cli.Config{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	page := string(body)

	want := []string{
		"aurora-blob",
		"blob-1", "blob-2", "blob-3", "blob-4",
		"nebula-dropzone", // only the Nebula variant is shipped
		"app-header",      // glass header
		"alpinejs",        // Alpine.js via CDN
		"x-data",          // Alpine reactive root
		"toast-container", // toast notifications
		"modal-overlay",   // confirmation modal
		"/api/upload",     // wired to the real backend upload endpoint
	}
	for _, marker := range want {
		if !strings.Contains(page, marker) {
			t.Errorf("Send page missing marker %q", marker)
		}
	}

	// The prototype shipped three layout variants + a floating switcher pill.
	// Production keeps only Nebula; the others and the switcher must be gone.
	banned := []string{
		"prototype-switcher",
		"variant-b",
		"variant-c",
		"orbit-dropzone",
		"simulateUpload", // prototype-only simulated upload helper
	}
	for _, marker := range banned {
		if strings.Contains(page, marker) {
			t.Errorf("Send page must not ship prototype leftover %q", marker)
		}
	}
}
