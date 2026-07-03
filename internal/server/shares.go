package server

import (
	"crypto/rand"
	"encoding/base64"
	"sort"
	"sync"
	"time"
)

// ShareRecord is one scoped, expiring share link. It grants its holder access
// to exactly one file (AbsPath) — the landing page plus stream/download bytes
// for that path and nothing else. ExpiresAt is the zero value when the link
// never expires (effective "until revoked or the server restarts").
//
// The store is process-lifetime and in-memory: a restart clears every link,
// matching the app's ephemeral global token. There is no persistence layer.
type ShareRecord struct {
	Token     string
	Path      string // forward-slash path relative to root, for owner-facing display
	AbsPath   string // sandbox-resolved absolute path; the binding enforced on serve
	CreatedAt time.Time
	ExpiresAt time.Time // zero => never expires
}

// expired reports whether a non-"never" link is past its expiry. Zero-value
// (never-expire) records are never expired by the clock.
func (r ShareRecord) expired() bool {
	return !r.ExpiresAt.IsZero() && time.Now().After(r.ExpiresAt)
}

// ShareManager is a goroutine-safe, in-memory map of share-token → record.
// Expired entries are reaped lazily on access (Lookup/List) rather than by a
// background sweeper, so the store owns no goroutines and tests don't leak.
type ShareManager struct {
	mu sync.RWMutex
	m  map[string]ShareRecord
}

func newShareManager() *ShareManager {
	return &ShareManager{m: make(map[string]ShareRecord)}
}

// TTL presets offered by the share UI. parseTTL resolves the preset keys (and
// the empty string) into these; "never" is encoded as a zero duration, not a
// value in this block.
const (
	shareTTLHour = time.Hour
	shareTTLDay  = 24 * time.Hour
	shareTTLWeek = 7 * 24 * time.Hour
)

// parseTTL resolves a preset key from the share UI into a duration. The empty
// string defaults to 24h so a bare create-without-selecting still yields a
// bounded link. "never" yields a zero duration (no expiry). Unknown keys return
// ok=false so the handler can 400 rather than silently bound a link.
func parseTTL(s string) (time.Duration, bool) {
	switch s {
	case "1h":
		return shareTTLHour, true
	case "24h", "":
		return shareTTLDay, true
	case "7d":
		return shareTTLWeek, true
	case "never":
		return 0, true
	default:
		return 0, false
	}
}

// Create mints a new share link bound to absPath. ttl==0 means no expiry.
func (sm *ShareManager) Create(path, absPath string, ttl time.Duration) (ShareRecord, error) {
	token, err := generateShareToken()
	if err != nil {
		return ShareRecord{}, err
	}
	rec := ShareRecord{
		Token:     token,
		Path:      path,
		AbsPath:   absPath,
		CreatedAt: time.Now(),
	}
	if ttl > 0 {
		rec.ExpiresAt = rec.CreatedAt.Add(ttl)
	}
	sm.mu.Lock()
	sm.m[token] = rec
	sm.mu.Unlock()
	return rec, nil
}

// Lookup returns the live record for a token, lazily reaping it if expired.
func (sm *ShareManager) Lookup(token string) (ShareRecord, bool) {
	sm.mu.RLock()
	rec, ok := sm.m[token]
	sm.mu.RUnlock()
	if !ok {
		return ShareRecord{}, false
	}
	if rec.expired() {
		sm.mu.Lock()
		// Re-check under write lock: another reader may have reaped it already.
		if cur, still := sm.m[token]; still && cur.expired() {
			delete(sm.m, token)
		}
		sm.mu.Unlock()
		return ShareRecord{}, false
	}
	return rec, true
}

// Revoke removes a link. Returns whether a link existed to revoke.
func (sm *ShareManager) Revoke(token string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	_, ok := sm.m[token]
	delete(sm.m, token)
	return ok
}

// List returns all live shares newest-first, reaping any expired ones it
// encounters. Used to render the owner's Shares page.
func (sm *ShareManager) List() []ShareRecord {
	now := time.Now()
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// Reap expired under the write lock while we're here.
	for k, rec := range sm.m {
		if !rec.ExpiresAt.IsZero() && rec.ExpiresAt.Before(now) {
			delete(sm.m, k)
		}
	}
	out := make([]ShareRecord, 0, len(sm.m))
	for _, rec := range sm.m {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// generateShareToken returns an opaque, URL-safe token carrying 128 bits of
// entropy from crypto/rand — unguessable and safe to place in a URL path
// (the /s/{token} landing route). 16 bytes → 22 base64-raw chars.
func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
