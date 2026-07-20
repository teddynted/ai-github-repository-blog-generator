package contentanalytics

import "time"

// Config tunes collection, retry/backoff, retention, and reporting.
type Config struct {
	Enabled        []Platform    // platforms to collect; empty = all with a provider
	MaxRetries     int           // recoverable-error retries per publication
	BaseDelay      time.Duration // first backoff delay
	MaxDelay       time.Duration // backoff ceiling
	Multiplier     float64       // backoff growth factor
	PageSize       int           // provider pagination size
	RetentionDays  int           // trim series older than this
	TopN           int           // ranking sizes in reports
	CloudWatchNS   string        // CloudWatch namespace
	PublishToCloud bool          // emit CloudWatch metrics after collection
}

// DefaultConfig returns production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:        nil,
		MaxRetries:     3,
		BaseDelay:      500 * time.Millisecond,
		MaxDelay:       30 * time.Second,
		Multiplier:     2.0,
		PageSize:       50,
		RetentionDays:  400,
		TopN:           5,
		CloudWatchNS:   "ContentAnalytics",
		PublishToCloud: true,
	}
}

// isEnabled reports whether a platform should be collected.
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

// topN returns a sane ranking size.
func (c Config) topN() int {
	if c.TopN <= 0 {
		return 5
	}
	return c.TopN
}

// pageSize returns a sane pagination size.
func (c Config) pageSize() int {
	if c.PageSize <= 0 {
		return 50
	}
	return c.PageSize
}

// namespace returns the CloudWatch namespace.
func (c Config) namespace() string {
	if c.CloudWatchNS == "" {
		return "ContentAnalytics"
	}
	return c.CloudWatchNS
}
