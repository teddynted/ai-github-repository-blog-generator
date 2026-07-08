// Package retry provides bounded exponential-backoff retries for transient
// failures. Only errors classified as retryable (upstream/unavailable) are
// retried; client errors (invalid input, unauthorized, not found) fail fast.
// AWS SDK calls are not wrapped here — the SDK has its own retry middleware.
package retry

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Config controls retry behaviour.
type Config struct {
	MaxAttempts int           // total attempts (>=1)
	BaseDelay   time.Duration // initial backoff
	MaxDelay    time.Duration // cap on backoff
}

// Default is a sensible policy for network calls.
var Default = Config{MaxAttempts: 3, BaseDelay: 500 * time.Millisecond, MaxDelay: 5 * time.Second}

func (c Config) withDefaults() Config {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 1
	}
	if c.MaxDelay < c.BaseDelay {
		c.MaxDelay = c.BaseDelay
	}
	return c
}

// IsRetryable reports whether an error should be retried. Transient upstream /
// unavailable errors are retryable; everything else fails fast.
func IsRetryable(err error) bool {
	switch apperror.CodeOf(err) {
	case apperror.CodeUpstream, apperror.CodeUnavailable:
		return true
	default:
		return false
	}
}

// Do runs fn, retrying retryable failures with exponential backoff + jitter up
// to cfg.MaxAttempts. It respects context cancellation between attempts.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	cfg = cfg.withDefaults()
	delay := cfg.BaseDelay
	var err error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if !IsRetryable(err) || attempt == cfg.MaxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitter(delay)):
		}
		if delay = delay * 2; delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}
	return err
}

// jitter returns a randomised delay in [d/2, d] to avoid thundering herds.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := int64(d / 2)
	if half <= 0 {
		return d
	}
	return time.Duration(half + rand.Int64N(half+1))
}
