package publishing

import "time"

// Config holds publishing rules. Use DefaultConfig() and override.
type Config struct {
	// Enabled lists the platforms that are turned on. Empty means "all
	// registered publishers".
	Enabled []Platform
	// Priority is the publishing order (platforms not listed publish last, in
	// registration order).
	Priority []Platform
	// Retry is the default retry policy.
	Retry RetryPolicy
	// Timeout bounds a single publish attempt.
	Timeout time.Duration
	// DefaultVisibility is applied when content does not specify one.
	DefaultVisibility string
	// ContinueOnError keeps distributing to remaining platforms when one fails.
	ContinueOnError bool
}

// DefaultConfig returns production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:  nil, // all registered
		Priority: []Platform{PlatformGitHub, PlatformDevTo, PlatformHashnode, PlatformMedium, PlatformYouTube},
		Retry: RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  2 * time.Second,
			MaxDelay:   30 * time.Second,
			Multiplier: 2.0,
		},
		Timeout:           30 * time.Second,
		DefaultVisibility: "public",
		ContinueOnError:   true,
	}
}

// isEnabled reports whether a platform is enabled (empty Enabled = all).
func (c Config) isEnabled(p Platform) bool {
	if len(c.Enabled) == 0 {
		return true
	}
	for _, e := range c.Enabled {
		if e == p {
			return true
		}
	}
	return false
}
