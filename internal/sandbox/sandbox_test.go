package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizePath_ValidNestedPath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := SanitizePath(root, "subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absRoot, _ := filepath.EvalSymlinks(root)
	want := filepath.Join(absRoot, "subdir", "file.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizePath_DotDotTraversal(t *testing.T) {
	root := t.TempDir()

	_, err := SanitizePath(root, "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for .. traversal, got nil")
	}
}

func TestSanitizePath_AbsolutePathOutsideRoot(t *testing.T) {
	root := t.TempDir()

	_, err := SanitizePath(root, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path outside root, got nil")
	}
}

func TestSanitizePath_DotDotMixedWithValid(t *testing.T) {
	root := t.TempDir()

	_, err := SanitizePath(root, "subdir/../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for mixed traversal, got nil")
	}
}

func TestSanitizePath_SymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	_, err := SanitizePath(root, "link")
	if err == nil {
		t.Fatal("expected error for symlink outside root, got nil")
	}
}

func TestSanitizePath_SymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}

	absRoot, _ := filepath.EvalSymlinks(root)
	got, err := SanitizePath(root, "alias/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(absRoot, "alias", "file.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizePath_EmptyPath(t *testing.T) {
	root := t.TempDir()

	absRoot, _ := filepath.EvalSymlinks(root)
	got, err := SanitizePath(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != absRoot {
		t.Errorf("got %q, want %q", got, absRoot)
	}
}
