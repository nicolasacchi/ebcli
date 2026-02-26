package ratelimit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	CacheFileName   = "ratelimits.json"
	FilePermissions = os.FileMode(0600)
)

// CacheEntry tracks rate limit state for a specific account+endpoint pair.
type CacheEntry struct {
	AccountUID string    `json:"account_uid"`
	Endpoint   string    `json:"endpoint"`
	Limit      int       `json:"limit"`
	Remaining  int       `json:"remaining"`
	RetryAfter time.Time `json:"retry_after,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Tracker manages rate limit state per account+endpoint.
type Tracker struct {
	mu        sync.Mutex
	entries   map[string]*CacheEntry
	cachePath string
	stderr    io.Writer
}

// NewTracker creates a rate limit tracker. Loads existing cache from disk.
func NewTracker(configDir string, stderr io.Writer) (*Tracker, error) {
	t := &Tracker{
		entries:   make(map[string]*CacheEntry),
		cachePath: filepath.Join(configDir, CacheFileName),
		stderr:    stderr,
	}
	t.load()
	return t, nil
}

// Update records rate limit information from API response headers.
func (t *Tracker) Update(accountUID, endpoint string, resp *http.Response) {
	if accountUID == "" || resp == nil {
		return
	}

	remaining, limit, ok := parseRateLimitHeaders(resp)
	if !ok {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	k := key(accountUID, endpoint)
	t.entries[k] = &CacheEntry{
		AccountUID: accountUID,
		Endpoint:   endpoint,
		Limit:      limit,
		Remaining:  remaining,
		UpdatedAt:  time.Now(),
	}

	if remaining <= 1 {
		fmt.Fprintf(t.stderr, "ebcli: warning: rate limit nearly exhausted for account %s endpoint %s (%d remaining)\n",
			accountUID[:8], endpoint, remaining)
	}
}

// RecordRetryAfter stores a Retry-After time.
func (t *Tracker) RecordRetryAfter(accountUID, endpoint string, retryAfter time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := key(accountUID, endpoint)
	entry, exists := t.entries[k]
	if !exists {
		entry = &CacheEntry{
			AccountUID: accountUID,
			Endpoint:   endpoint,
		}
		t.entries[k] = entry
	}
	entry.RetryAfter = retryAfter
	entry.UpdatedAt = time.Now()
}

// Check returns an error if rate limited. Returns nil if OK to proceed.
func (t *Tracker) Check(accountUID, endpoint string) error {
	if accountUID == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	k := key(accountUID, endpoint)
	entry, exists := t.entries[k]
	if !exists {
		return nil
	}

	if !entry.RetryAfter.IsZero() && time.Now().Before(entry.RetryAfter) {
		return fmt.Errorf("rate limited for account %s endpoint %s until %s",
			accountUID[:8], endpoint, entry.RetryAfter.Format(time.RFC3339))
	}

	return nil
}

// WaitTime returns how long to wait before retrying, or 0 if no wait needed.
func (t *Tracker) WaitTime(accountUID, endpoint string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := key(accountUID, endpoint)
	entry, exists := t.entries[k]
	if !exists {
		return 0
	}

	if !entry.RetryAfter.IsZero() && time.Now().Before(entry.RetryAfter) {
		return time.Until(entry.RetryAfter)
	}
	return 0
}

// Persist writes the current cache to disk.
func (t *Tracker) Persist() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Prune stale entries (older than 24h)
	cutoff := time.Now().Add(-24 * time.Hour)
	for k, e := range t.entries {
		if e.UpdatedAt.Before(cutoff) {
			delete(t.entries, k)
		}
	}

	if len(t.entries) == 0 {
		os.Remove(t.cachePath)
		return nil
	}

	data, err := json.MarshalIndent(t.entries, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(t.cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(t.cachePath, data, FilePermissions)
}

func (t *Tracker) load() {
	data, err := os.ReadFile(t.cachePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &t.entries)
}

func key(accountUID, endpoint string) string {
	return accountUID + ":" + endpoint
}

func parseRateLimitHeaders(resp *http.Response) (remaining, limit int, ok bool) {
	rStr := resp.Header.Get("X-Ratelimit-Remaining")
	lStr := resp.Header.Get("X-Ratelimit-Limit")
	if rStr == "" && lStr == "" {
		return 0, 0, false
	}

	remaining, _ = strconv.Atoi(rStr)
	limit, _ = strconv.Atoi(lStr)
	return remaining, limit, true
}

// ParseRetryAfter parses the Retry-After header as seconds or HTTP-date.
func ParseRetryAfter(resp *http.Response) (time.Time, bool) {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return time.Time{}, false
	}

	// Try as seconds
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Now().Add(time.Duration(secs) * time.Second), true
	}

	// Try as HTTP-date (RFC 7231)
	if t, err := http.ParseTime(val); err == nil {
		return t, true
	}

	return time.Time{}, false
}
