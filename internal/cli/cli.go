package cli

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
)

// Config holds parsed CLI flags.
type Config struct {
	RootDir string // positional arg, default "."
	Host    string // --host, default "0.0.0.0"
	Port    int    // --port, default 8080
	Token   string // --token, default "" (auto-generate)
	MaxSize int64  // --max-size, default 0 (unlimited, in bytes)
	Dev     bool   // --dev, default false
}

// ParseArgs parses CLI arguments into a Config.
// args should be os.Args[1:].
func ParseArgs(args []string) (Config, error) {
	fs := flag.NewFlagSet("hyperdrop", flag.ContinueOnError)

	host := fs.String("host", "0.0.0.0", "bind address")
	port := fs.Int("port", 8080, "listen port")
	token := fs.String("token", "", "access token (auto-generated if empty)")
	maxSize := fs.Int64("max-size", 0, "max upload size in bytes (0 = unlimited)")
	dev := fs.Bool("dev", false, "serve static assets from disk")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	rootDir := "."
	if fs.NArg() > 0 {
		rootDir = fs.Arg(0)
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		return Config{}, fmt.Errorf("directory %q does not exist", rootDir)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("%q is not a directory", rootDir)
	}

	return Config{
		RootDir: rootDir,
		Host:    *host,
		Port:    *port,
		Token:   *token,
		MaxSize: *maxSize,
		Dev:     *dev,
	}, nil
}

const tokenChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateToken returns a random 8-character lowercase alphanumeric token.
func GenerateToken() (string, error) {
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tokenChars))))
		if err != nil {
			return "", fmt.Errorf("generate token: %w", err)
		}
		b[i] = tokenChars[n.Int64()]
	}
	return string(b), nil
}
