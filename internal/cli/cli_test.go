package cli

import (
	"os"
	"strings"
	"testing"
)

func TestParseArgs_Version(t *testing.T) {
	cfg, err := ParseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Fatal("ShowVersion: got false, want true")
	}

	// --version must not require a valid root directory: other fields are left
	// zero so the caller short-circuits to printing version info.
	if cfg.RootDir != "" {
		t.Errorf("RootDir: got %q, want empty (version short-circuits validation)", cfg.RootDir)
	}
}

func TestParseArgs_Defaults(t *testing.T) {
	cfg, err := ParseArgs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RootDir != "." {
		t.Errorf("RootDir: got %q, want %q", cfg.RootDir, ".")
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 8080)
	}
	if cfg.Token != "" {
		t.Errorf("Token: got %q, want empty (auto-generate)", cfg.Token)
	}
	if cfg.Dev {
		t.Errorf("Dev: got true, want false")
	}
}

func TestParseArgs_CustomToken(t *testing.T) {
	cfg, err := ParseArgs([]string{"--token", "mysecret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "mysecret" {
		t.Errorf("Token: got %q, want %q", cfg.Token, "mysecret")
	}
}

func TestParseArgs_FlagOverrides(t *testing.T) {
	cfg, err := ParseArgs([]string{"--host", "127.0.0.1", "--port", "3000", "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 3000 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 3000)
	}
	if cfg.RootDir != "/tmp" {
		t.Errorf("RootDir: got %q, want %q", cfg.RootDir, "/tmp")
	}
}

func TestGenerateToken(t *testing.T) {
	tok, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) != 8 {
		t.Errorf("token length: got %d, want 8", len(tok))
	}

	// Must be lowercase alphanumeric.
	for _, c := range tok {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789", c) {
			t.Errorf("token contains unexpected char %q", c)
		}
	}

	// Two calls should produce different tokens.
	tok2, _ := GenerateToken()
	if tok == tok2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestParseArgs_MaxSize_HumanReadable(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"500MB", 524288000},
		{"2GB", 2147483648},
		{"100KB", 102400},
		{"1.5GB", 1610612736},
		{"500", 500}, // bare number = bytes
		{"0", 0},     // zero = unlimited
	}
	for _, c := range cases {
		cfg, err := ParseArgs([]string{"--max-size", c.in})
		if err != nil {
			t.Errorf("--max-size %q: unexpected error: %v", c.in, err)
			continue
		}
		if cfg.MaxSize != c.want {
			t.Errorf("--max-size %q: got %d bytes, want %d", c.in, cfg.MaxSize, c.want)
		}
	}
}

func TestParseArgs_MaxSize_Invalid(t *testing.T) {
	for _, in := range []string{"abc", "10XB", "-5MB", "MB", "1.2.3GB"} {
		if _, err := ParseArgs([]string{"--max-size", in}); err == nil {
			t.Errorf("--max-size %q: expected error, got nil", in)
		}
	}
}

func TestParseArgs_NonexistentDir(t *testing.T) {
	_, err := ParseArgs([]string{"/no/such/directory/hyperdrop_test"})
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestHumanizeSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{524288000, "500.0 MB"},
		{2147483648, "2.0 GB"},
		{102400, "100.0 KB"},
		{500, "500 B"},
		{0, "0 B"},
	}
	for _, c := range cases {
		if got := HumanizeSize(c.in); got != c.want {
			t.Errorf("HumanizeSize(%d): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseArgs_NotADirectory(t *testing.T) {
	f, err := os.CreateTemp("", "hyperdrop_test_file")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	_, err = ParseArgs([]string{f.Name()})
	if err == nil {
		t.Fatal("expected error for file (not directory), got nil")
	}
}
