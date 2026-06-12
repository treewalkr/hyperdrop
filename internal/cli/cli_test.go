package cli

import (
	"os"
	"strings"
	"testing"
)

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

func TestParseArgs_NonexistentDir(t *testing.T) {
	_, err := ParseArgs([]string{"/no/such/directory/hyperdrop_test"})
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
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
