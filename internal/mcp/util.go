package mcp

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"
)

// parseIntDefault parses s as an int, returning def on failure/empty.
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func isForbidden(err error) bool {
	return errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthenticated)
}

func isTimeout(err error) bool { return errors.Is(err, ErrTimeout) }

// sortTools orders tools by name for deterministic discovery output.
func sortTools(ts []Tool) {
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].Name < ts[j].Name })
}

// sortResources orders resources by URI.
func sortResources(rs []Resource) {
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].URI < rs[j].URI })
}

// rateLimiter is a tiny per-caller token bucket protecting servers from abuse
// (resource limits). It is monotonic-clock based and safe for concurrent use.
type rateLimiter struct {
	mu     sync.Mutex
	perMin int
	window map[string][]time.Time
	now    func() time.Time
}

func newRateLimiter(perMinute int, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{perMin: perMinute, window: map[string][]time.Time{}, now: now}
}

// allow reports whether the caller may make another call now (and records it).
func (r *rateLimiter) allow(caller string) bool {
	if r == nil || r.perMin <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cut := r.now().Add(-time.Minute)
	kept := r.window[caller][:0]
	for _, t := range r.window[caller] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.perMin {
		r.window[caller] = kept
		return false
	}
	r.window[caller] = append(kept, r.now())
	return true
}
