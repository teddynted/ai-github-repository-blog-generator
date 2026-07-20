package socialintel

import "time"

// Config holds intelligence-platform settings.
type Config struct {
	// Enabled lists the platforms to collect. Empty means all registered.
	Enabled []Platform
	// Retry is the collection retry policy.
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
	// RetentionDays caps how far back trend analysis reaches (0 = unlimited).
	RetentionDays int
	// TopN bounds the size of ranked lists in the briefing.
	TopN int
}

// DefaultConfig returns production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:       nil,
		MaxRetries:    3,
		BaseDelay:     2 * time.Second,
		MaxDelay:      30 * time.Second,
		Multiplier:    2.0,
		RetentionDays: 400,
		TopN:          5,
	}
}

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

func (c Config) topN() int {
	if c.TopN <= 0 {
		return 5
	}
	return c.TopN
}
