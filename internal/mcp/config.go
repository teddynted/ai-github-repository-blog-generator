package mcp

import "time"

// Config controls the client's cross-cutting behavior. Zero value is usable
// (sensible secure-by-default settings applied by normalize()).
type Config struct {
	// Caller identifies the principal for authorization + audit (e.g. "release-worker").
	Caller string
	// CallTimeout bounds a single tool/resource call. Default 30s.
	CallTimeout time.Duration
	// MaxRetries is how many extra attempts a recoverable failure gets. Default 2.
	MaxRetries int
	// RetryBackoff is the base delay between retries (linear). Default 100ms.
	RetryBackoff time.Duration
	// RatePerMinute caps calls per caller per minute (0 = unlimited). Default 0.
	RatePerMinute int
	// AuditEnabled toggles audit logging. Default true.
	AuditEnabled bool
	// AllowedServers, when non-empty, restricts which server ids may be used
	// (least privilege — a caller sees only the servers it needs).
	AllowedServers []string
}

func (c Config) normalize() Config {
	if c.Caller == "" {
		c.Caller = "anonymous"
	}
	if c.CallTimeout <= 0 {
		c.CallTimeout = 30 * time.Second
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 2
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = 100 * time.Millisecond
	}
	return c
}

func (c Config) serverAllowed(id string) bool {
	if len(c.AllowedServers) == 0 {
		return true
	}
	return contains(c.AllowedServers, id)
}
