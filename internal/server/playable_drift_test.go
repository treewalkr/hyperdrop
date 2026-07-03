package server

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/treewalkr/hyperdrop/internal/static"
)

// The playable-extension set is mirrored in three places (server.playable,
// files.html PLAYABLE_EXTS, player.html PLAYABLE) because the pages are static
// HTML with no server templating. This test is the drift backstop: if one copy
// changes, this fails until all three agree.

// canonicalPlayableExts is the single source of truth this test enforces.
var canonicalPlayableExts = []string{".mp4", ".m4v", ".webm", ".ogv", ".mov"}

// TestPlayable_MatchesCanonical pins server.playable to the canonical set.
func TestPlayable_MatchesCanonical(t *testing.T) {
	for _, ext := range canonicalPlayableExts {
		if !playable("clip" + ext) {
			t.Errorf("playable(%q) = false, want true", "clip"+ext)
		}
	}
	// Everything else playable() accepts must be in the canonical set — i.e. the
	// switch has no stray cases. Sample a handful of common non-playable exts.
	for _, ext := range []string{".mkv", ".avi", ".flv", ".wmv", ".mp3", ".txt"} {
		if playable("clip" + ext) {
			t.Errorf("playable(%q) = true, want false", "clip"+ext)
		}
	}
}

// TestPlayable_ClientListsMatchCanonical parses the PLAYABLE arrays out of the
// embedded static pages and asserts they equal the canonical set, so an edit to
// either page can't silently diverge from the server's gating.
func TestPlayable_ClientListsMatchCanonical(t *testing.T) {
	cases := []struct {
		file string
		decl string // the "NAME =" prefix to locate the array literal
	}{
		{"files.html", "PLAYABLE_EXTS"},
		{"player.html", "PLAYABLE"},
	}
	want := append([]string(nil), canonicalPlayableExts...)
	sort.Strings(want)

	literalRe := regexp.MustCompile(`\[\s*('[^']*'(?:\s*,\s*'[^']*')*)\s*\]`)
	elemRe := regexp.MustCompile(`'([^']*)'`)

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			raw, err := fs.ReadFile(static.Assets, c.file)
			if err != nil {
				t.Fatalf("read %s from embed FS: %v", c.file, err)
			}
			// Find the line declaring this constant, then the first array literal on it.
			declRe := regexp.MustCompile(`(?m)^\s*const\s+` + regexp.QuoteMeta(c.decl) + `\s*=\s*(\[.*\])\s*;?\s*$`)
			m := declRe.FindStringSubmatch(string(raw))
			if m == nil {
				t.Fatalf("could not find `%s = [...]` declaration in %s", c.decl, c.file)
			}
			lit := literalRe.FindString(m[1])
			if lit == "" {
				t.Fatalf("no array literal in %s declaration: %q", c.decl, m[1])
			}
			got := elemRe.FindAllStringSubmatch(lit, -1)
			var gotExts []string
			for _, g := range got {
				gotExts = append(gotExts, strings.ToLower(g[1]))
			}
			sort.Strings(gotExts)

			if !equalSorted(gotExts, want) {
				t.Errorf("%s %s = %v, want %v (canonical)", c.file, c.decl, gotExts, want)
			}
		})
	}
}

func equalSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
