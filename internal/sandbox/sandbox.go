package sandbox

import (
	"errors"
	"path/filepath"
	"strings"
)

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
		return "", err
	}
	joined := filepath.Join(absRoot, requestedPath)
	cleaned := filepath.Clean(joined)

	// Resolve symlinks in the deepest existing ancestor.
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
