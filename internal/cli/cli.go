package cli

import (
	"crypto/rand"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
)

// Config holds parsed CLI flags.
type Config struct {
	RootDir     string // positional arg, default "."
	Host        string // --host, default "0.0.0.0"
	Port        int    // --port, default 8080
	Token       string // --token, default "" (auto-generate)
	MaxSize     int64  // --max-size, default 0 (unlimited, in bytes)
	Dev         bool   // --dev, default false
	ShowVersion bool   // --version, print version and exit
}

// ParseArgs parses CLI arguments into a Config.
// args should be os.Args[1:].
func ParseArgs(args []string) (Config, error) {
	fs := flag.NewFlagSet("hyperdrop", flag.ContinueOnError)

	host := fs.String("host", "0.0.0.0", "bind address")
	port := fs.Int("port", 8080, "listen port")
	token := fs.String("token", "", "access token (auto-generated if empty)")
	maxSize := new(sizeValue)
	fs.Var(maxSize, "max-size", "max upload size (e.g. 500MB, 2GB, 100KB; 0 = unlimited)")
	dev := fs.Bool("dev", false, "serve static assets from disk")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	// --version short-circuits before any directory validation so it works
	// regardless of the current working directory.
	if *showVersion {
		return Config{ShowVersion: true}, nil
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
		RootDir:     rootDir,
		Host:        *host,
		Port:        *port,
		Token:       *token,
		MaxSize:     int64(*maxSize),
		Dev:         *dev,
		ShowVersion: false,
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

// sizeValue is a flag.Value that parses human-readable size strings
// (500MB, 2GB, 1.5GB, 100KB) into bytes. A bare number is treated as bytes.
type sizeValue int64

func (v *sizeValue) String() string { return strconv.FormatInt(int64(*v), 10) }
func (v *sizeValue) Set(s string) error {
	n, err := ParseSize(s)
	if err != nil {
		return err
	}
	*v = sizeValue(n)
	return nil
}

// ParseSize converts a human-readable size string into bytes.
// Supported unit suffixes: B, K/KB, M/MB, G/GB, T/TB (binary, 1024-based).
// A missing unit means bytes. Examples: "500MB"→524288000, "1.5GB"→1610612736.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Split the leading numeric prefix (digits and a single dot) from the unit.
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	numStr := s[:i]
	unit := strings.TrimSpace(s[i:])
	if numStr == "" || numStr == "." {
		return 0, fmt.Errorf("invalid size %q: missing number", s)
	}
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %v", s, err)
	}
	if f < 0 {
		return 0, fmt.Errorf("invalid size %q: must not be negative", s)
	}

	var mult int64
	switch strings.ToUpper(unit) {
	case "", "B":
		mult = 1
	case "K", "KB":
		mult = 1 << 10
	case "M", "MB":
		mult = 1 << 20
	case "G", "GB":
		mult = 1 << 30
	case "T", "TB":
		mult = 1 << 40
	default:
		return 0, fmt.Errorf("invalid size %q: unknown unit %q", s, unit)
	}

	bytes := f * float64(mult)
	if bytes > math.MaxInt64 {
		return 0, fmt.Errorf("invalid size %q: overflow", s)
	}
	return int64(bytes), nil
}

// HumanizeSize renders a byte count back into a compact human-readable form
// (binary units), used for upload-limit error messages. Examples:
// 524288000→"500.0 MB", 2147483648→"2.0 GB", 102400→"100.0 KB", 500→"500 B".
func HumanizeSize(b int64) string {
	const unit = 1024
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp+1])
}
