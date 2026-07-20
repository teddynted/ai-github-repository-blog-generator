package contentoptimizer

import "time"

// Config tunes the optimization run.
type Config struct {
	MinSupport       int           // minimum records for a pattern to be reported
	TopN             int           // ranking sizes in reports/recommendations
	WinningPct       float64       // top percentile threshold for "winning" (e.g. 0.75)
	LosingPct        float64       // bottom percentile threshold for "losing" (e.g. 0.25)
	TrendChangePct   float64       // |change| to call a topic emerging/declining (%)
	MaxReasonRetries int           // reasoning-provider retries before deterministic fallback
	ReasonTimeout    time.Duration // per reasoning attempt
	CloudWatchNS     string        // CloudWatch namespace
	PublishToCloud   bool          // emit CloudWatch metrics
}

// DefaultConfig returns production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		MinSupport:       2,
		TopN:             5,
		WinningPct:       0.75,
		LosingPct:        0.25,
		TrendChangePct:   20,
		MaxReasonRetries: 2,
		ReasonTimeout:    30 * time.Second,
		CloudWatchNS:     "ContentOptimization",
		PublishToCloud:   true,
	}
}

func (c Config) topN() int {
	if c.TopN <= 0 {
		return 5
	}
	return c.TopN
}

func (c Config) minSupport() int {
	if c.MinSupport <= 0 {
		return 1
	}
	return c.MinSupport
}

func (c Config) winningPct() float64 {
	if c.WinningPct <= 0 || c.WinningPct >= 1 {
		return 0.75
	}
	return c.WinningPct
}

func (c Config) losingPct() float64 {
	if c.LosingPct <= 0 || c.LosingPct >= 1 {
		return 0.25
	}
	return c.LosingPct
}

func (c Config) trendChangePct() float64 {
	if c.TrendChangePct <= 0 {
		return 20
	}
	return c.TrendChangePct
}

func (c Config) namespace() string {
	if c.CloudWatchNS == "" {
		return "ContentOptimization"
	}
	return c.CloudWatchNS
}
