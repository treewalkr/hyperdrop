package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/cli"
)

// TestFilesPage_AuroraMarkers guards the Files page (files.html) production UI:
// the Aurora design system and all three view variants must be present, the
// prototype mock data must be stripped, and the page must be wired to the real
// API. This is a smoke test over the public HTTP boundary; visual fidelity is
// verified by human review.
func TestFilesPage_AuroraMarkers(t *testing.T) {
	r := NewRouter(cli.Config{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/files")
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
		"variant-a",      // table view
		"variant-b",      // grid view — all three variants are shipped on Files
		"variant-c",      // feed view
		"variant-switcher", // floating variant switcher (Files keeps it, unlike Send)
		"app-header",     // glass header
		"alpinejs",       // Alpine.js via CDN
		"x-data",         // Alpine reactive root
		"toast-container", // toast notifications
		"modal-overlay",  // confirmation modal
		"/api/files",     // wired to the real backend listing endpoint
		"this device",    // placeholder sender (no sender field exists yet)
		"No files yet",   // empty state
	}
	for _, marker := range want {
		if !strings.Contains(page, marker) {
			t.Errorf("Files page missing marker %q", marker)
		}
	}

	// The prototype shipped a static MOCK_FILES array and per-file sender
	// variance. Production must drive all state from the API; no mock data or
	// randomized state may remain.
	banned := []string{
		"MOCK_FILES", // prototype-only mock dataset
		"Math.random", // no randomized mock state
	}
	for _, marker := range banned {
		if strings.Contains(page, marker) {
			t.Errorf("Files page must not ship prototype leftover %q", marker)
		}
	}
}
