package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// SanitizePath resolves and validates requestedPath against rootDir.
// Returns the cleaned absolute path if it stays within rootDir, or an error otherwise.
//
// Note: validates symlinks at check time but returns a string path, not a file descriptor.
// A TOCTOU race exists if the filesystem changes between validation and use.
// Callers needing stronger guarantees should evaluate fd-based approaches (e.g. openat2).
func SanitizePath(rootDir, requestedPath string) (string, error) {
	if filepath.IsAbs(requestedPath) {
		return "", errors.New("path escapes root directory")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	absRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolving root directory: %w", err)
	}
	joined := filepath.Join(absRoot, requestedPath)
	cleaned := filepath.Clean(joined)

	// A non-empty request that resolves to the root itself (e.g. "." or
	// "foo/..") is rejected: the root is never a valid file target, and a
	// caller that then does filepath.Dir/Base on the result would operate on
	// the root's parent — escaping the sandbox (issue #33). The empty request
	// is the documented "root itself" form and remains valid.
	if cleaned == absRoot && requestedPath != "" {
		return "", errors.New("path resolves to root directory")
	}

	// Direct prefix check: catches traversal regardless of filesystem state.
	if !strings.HasPrefix(cleaned, absRoot+string(filepath.Separator)) && cleaned != absRoot {
		return "", errors.New("path escapes root directory")
	}

	// Defense-in-depth: resolve symlinks in the deepest existing ancestor.
	dir := cleaned
	for dir != absRoot && dir != "/" {
		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			if !strings.HasPrefix(resolved, absRoot+string(filepath.Separator)) && resolved != absRoot {
				return "", errors.New("path escapes root directory")
			}
			break
		}
		dir = filepath.Dir(dir)
	}

	return cleaned, nil
}
