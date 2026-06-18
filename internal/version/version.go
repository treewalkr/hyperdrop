// Package version holds build-time metadata.
//
// Release binaries get Version/Commit/Date injected via goreleaser ldflags
// (see .goreleaser.yml). For all other builds — `go install`, `go build`,
// `go run` — the Go toolchain embeds VCS metadata into the binary, which we
// read back via runtime/debug as a fallback so `hyperdrop --version` still
// reports something meaningful.
package version

import (
	"fmt"
	"runtime/debug"
)

var (
	// Version is the semver of the release, e.g. "1.0.0". Injected by ldflags.
	Version = "dev"
	// Commit is the git SHA the binary was built from. Injected by ldflags.
	Commit = "none"
	// Date is the RFC3339 build timestamp. Injected by ldflags.
	Date = "unknown"
)

// String renders version metadata for the --version output.
func String() string {
	v, commit, date := resolve(Version, Commit, Date)
	return fmt.Sprintf("hyperdrop %s (commit %s, built %s)", v, commit, date)
}

// resolve returns the version triple, substituting values from the embedded
// build info whenever an ldflags default is still in place. Split out for
// testability.
func resolve(version, commit, date string) (string, string, string) {
	info, ok := debug.ReadBuildInfo()

	if version == "dev" && ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	var revision, builtAt string
	if ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.time":
				builtAt = s.Value
			}
		}
	}
	if commit == "none" && revision != "" {
		commit = revision
	}
	if date == "unknown" && builtAt != "" {
		date = builtAt
	}
	return version, commit, date
}
