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

// DailyUsage tracks per-connection daily API access count.
type DailyUsage struct {
	Date      string    `json:"date"`       // YYYY-MM-DD
	Count     int       `json:"count"`      // calls made today
	MaxPerDay int       `json:"max_per_day"` // 0 = unlimited
	UpdatedAt time.Time `json:"updated_at"`
}

// cacheFile is the on-disk format for the rate limit cache.
type cacheFile struct {
	Entries    map[string]*CacheEntry `json:"entries,omitempty"`
	DailyUsage map[string]*DailyUsage `json:"daily_usage,omitempty"`
}

// Tracker manages rate limit state per account+endpoint and daily usage per connection.
type Tracker struct {
	mu         sync.Mutex
	entries    map[string]*CacheEntry
	daily      map[string]*DailyUsage // keyed by connection name
	cachePath  string
	stderr     io.Writer
}

// NewTracker creates a rate limit tracker. Loads existing cache from disk.
func NewTracker(configDir string, stderr io.Writer) (*Tracker, error) {
	t := &Tracker{
		entries:   make(map[string]*CacheEntry),
		daily:     make(map[string]*DailyUsage),
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

// CheckDaily returns an error if the connection has reached its daily access limit.
// Returns nil if maxPerDay is 0 (unlimited) or if under the limit.
func (t *Tracker) CheckDaily(connectionName string, maxPerDay int) error {
	if maxPerDay <= 0 {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	usage, exists := t.daily[connectionName]
	if !exists || usage.Date != today {
		return nil // no usage today
	}

	if usage.Count >= maxPerDay {
		return fmt.Errorf("daily limit reached for %s (%d/%d today). Try again tomorrow", connectionName, usage.Count, maxPerDay)
	}

	if usage.Count >= maxPerDay-1 {
		fmt.Fprintf(t.stderr, "ebcli: warning: last daily access for %s (%d/%d)\n", connectionName, usage.Count+1, maxPerDay)
	}

	return nil
}

// RecordDaily increments the daily access counter for a connection.
func (t *Tracker) RecordDaily(connectionName string, maxPerDay int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	usage, exists := t.daily[connectionName]
	if !exists || usage.Date != today {
		// New day or first usage
		usage = &DailyUsage{
			Date:      today,
			MaxPerDay: maxPerDay,
		}
		t.daily[connectionName] = usage
	}

	usage.Count++
	usage.MaxPerDay = maxPerDay
	usage.UpdatedAt = time.Now()
}

// RemainingToday returns how many accesses remain today. Returns -1 for unlimited.
func (t *Tracker) RemainingToday(connectionName string, maxPerDay int) int {
	if maxPerDay <= 0 {
		return -1
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	usage, exists := t.daily[connectionName]
	if !exists || usage.Date != today {
		return maxPerDay
	}

	remaining := maxPerDay - usage.Count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// DailyUsageFor returns the current daily usage for a connection (count, max).
// Returns (0, 0) if no usage tracked.
func (t *Tracker) DailyUsageFor(connectionName string) (count, maxPerDay int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	usage, exists := t.daily[connectionName]
	if !exists || usage.Date != today {
		return 0, 0
	}
	return usage.Count, usage.MaxPerDay
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

	// Prune stale daily usage (not today)
	today := time.Now().Format("2006-01-02")
	for k, d := range t.daily {
		if d.Date != today {
			delete(t.daily, k)
		}
	}

	if len(t.entries) == 0 && len(t.daily) == 0 {
		os.Remove(t.cachePath)
		return nil
	}

	cf := cacheFile{
		Entries:    t.entries,
		DailyUsage: t.daily,
	}
	data, err := json.MarshalIndent(cf, "", "  ")
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

	// Try new format first (has "entries" or "daily_usage" keys)
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err == nil && (cf.Entries != nil || cf.DailyUsage != nil) {
		if cf.Entries != nil {
			t.entries = cf.Entries
		}
		if cf.DailyUsage != nil {
			t.daily = cf.DailyUsage
		}
		return
	}

	// Fall back to old format (plain map of entries)
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
