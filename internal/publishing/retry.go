package publishing

import (
	"context"
	"time"
)

// RetryEngine runs an operation with exponential backoff, retrying only
// recoverable failures.
type RetryEngine struct {
	Policy  RetryPolicy
	Sleep   Sleeper
	OnRetry func(attempt int, err error, delay time.Duration)
}

// Do runs fn until it succeeds, the retries are exhausted, a permanent error
// occurs, or the context is cancelled. It reports the number of attempts made.
func (e RetryEngine) Do(ctx context.Context, fn func(ctx context.Context) error) (attempts int, err error) {
	sleep := e.Sleep
	if sleep == nil {
		sleep = realSleeper
	}
	max := e.Policy.MaxRetries
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return attempt, ctx.Err()
		}
		err = fn(ctx)
		attempts = attempt + 1
		if err == nil {
			return attempts, nil
		}
		// Stop on permanent errors or when retries are exhausted.
		if !isRecoverable(err) || attempt >= max {
			return attempts, err
		}
		delay := e.backoff(attempt)
		if e.OnRetry != nil {
			e.OnRetry(attempt+1, err, delay)
		}
		sleep(ctx, delay)
	}
}

// backoff computes the delay for a given attempt (0-based): base * mult^attempt,
// capped at MaxDelay.
func (e RetryEngine) backoff(attempt int) time.Duration {
	base := e.Policy.BaseDelay
	if base <= 0 {
		base = time.Second
	}
	mult := e.Policy.Multiplier
	if mult < 1 {
		mult = 2
	}
	d := float64(base)
	for i := 0; i < attempt; i++ {
		d *= mult
	}
	delay := time.Duration(d)
	if e.Policy.MaxDelay > 0 && delay > e.Policy.MaxDelay {
		delay = e.Policy.MaxDelay
	}
	return delay
}
