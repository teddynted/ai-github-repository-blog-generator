package socialintel

import (
	"context"
	"time"
)

// Collector runs daily collection across the registered providers, persisting
// immutable snapshots. It dedupes (never re-collects a day already stored),
// retries recoverable failures with exponential backoff, and emits monitoring
// signals. It supports manual runs, historical imports, backfill, and
// incremental synchronization.
type Collector struct {
	Config    Config
	Repo      Repository
	Monitor   Monitor
	Now       Clock
	Sleep     func(context.Context, time.Duration)
	providers map[Platform]Provider
}

// NewCollector wires a collector. A nil repo/monitor get in-memory/no-op
// defaults; a nil clock uses time.Now.
func NewCollector(cfg Config, repo Repository, providers []Provider, now Clock) *Collector {
	if now == nil {
		now = time.Now
	}
	if repo == nil {
		repo = NewMemoryRepository()
	}
	c := &Collector{
		Config:    cfg,
		Repo:      repo,
		Monitor:   nopMonitor{},
		Now:       now,
		Sleep:     func(ctx context.Context, d time.Duration) { sleepCtx(ctx, d) },
		providers: map[Platform]Provider{},
	}
	for _, p := range providers {
		c.providers[p.Name()] = p
	}
	return c
}

// Register adds/replaces a provider (a new platform, no core changes).
func (c *Collector) Register(p Provider) { c.providers[p.Name()] = p }

// CollectResult reports the outcome of a collection run.
type CollectResult struct {
	Date      Date                        `json:"date"`
	Platforms map[Platform]PlatformResult `json:"platforms"`
}

// PlatformResult is one platform's collection outcome.
type PlatformResult struct {
	Collected bool   `json:"collected"`
	Skipped   bool   `json:"skipped"` // already had a snapshot for the day
	Content   int    `json:"content"`
	Retries   int    `json:"retries"`
	Error     string `json:"error,omitempty"`
}

// Collect runs collection for a date across all enabled providers.
func (c *Collector) Collect(ctx context.Context, date Date) CollectResult {
	res := CollectResult{Date: date, Platforms: map[Platform]PlatformResult{}}
	for platform, provider := range c.providers {
		if !c.Config.isEnabled(platform) {
			continue
		}
		res.Platforms[platform] = c.collectOne(ctx, provider, date)
	}
	return res
}

// CollectToday collects for today's date.
func (c *Collector) CollectToday(ctx context.Context) CollectResult {
	return c.Collect(ctx, dateOf(c.Now()))
}

// Backfill collects for every day in [from, to] that is missing, oldest first.
// This supports historical imports and gap recovery — existing days are skipped
// (never overwritten).
func (c *Collector) Backfill(ctx context.Context, from, to Date) []CollectResult {
	var out []CollectResult
	for d := from; d <= to; d = addDays(d, 1) {
		out = append(out, c.Collect(ctx, d))
		if daysBetween(from, d) > 3650 { // safety bound
			break
		}
	}
	return out
}

// collectOne collects one platform for one date with dedupe + retry.
func (c *Collector) collectOne(ctx context.Context, provider Provider, date Date) PlatformResult {
	platform := provider.Name()
	dims := map[string]string{"platform": string(platform), "date": date}

	// Dedupe: never re-collect a day already stored (idempotent).
	if c.Repo.AccountSnapshotExists(platform, date) {
		c.Monitor.Count("collection.skipped", 1, dims)
		return PlatformResult{Skipped: true}
	}

	start := c.Now()
	var (
		acct    AccountSnapshot
		content []ContentSnapshot
		retries int
		lastErr error
	)
	maxAttempts := c.Config.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			retries++
			c.Monitor.Count("collection.retry", 1, dims)
			c.Sleep(ctx, c.backoff(attempt-1))
		}
		a, aerr := provider.CollectAccount(ctx, date)
		if aerr != nil {
			lastErr = aerr
			if !isRecoverable(aerr) {
				break
			}
			continue
		}
		cs, cerr := provider.CollectContent(ctx, date)
		if cerr != nil {
			lastErr = cerr
			if !isRecoverable(cerr) {
				break
			}
			continue
		}
		acct, content, lastErr = a, cs, nil
		break
	}
	c.Monitor.Duration("collection.duration", c.Now().Sub(start), dims)

	if lastErr != nil {
		c.Monitor.Error("collection.failed", lastErr, dims)
		return PlatformResult{Retries: retries, Error: lastErr.Error()}
	}

	// Persist immutable snapshots.
	acct.Platform, acct.Date = platform, date
	if acct.CollectedAt.IsZero() {
		acct.CollectedAt = c.Now()
	}
	if err := c.Repo.SaveAccountSnapshot(acct); err != nil {
		c.Monitor.Error("db.write.account", err, dims)
		return PlatformResult{Retries: retries, Error: err.Error()}
	}
	saved := 0
	for _, s := range content {
		s.Platform, s.Date = platform, date
		if s.CollectedAt.IsZero() {
			s.CollectedAt = c.Now()
		}
		if err := c.Repo.SaveContentSnapshot(s); err == nil {
			saved++
		}
	}
	c.Monitor.Count("collection.success", 1, dims)
	c.Monitor.Count("db.write.content", float64(saved), dims)
	return PlatformResult{Collected: true, Content: saved, Retries: retries}
}

func (c *Collector) backoff(attempt int) time.Duration {
	base := c.Config.BaseDelay
	if base <= 0 {
		base = time.Second
	}
	mult := c.Config.Multiplier
	if mult < 1 {
		mult = 2
	}
	d := float64(base)
	for i := 0; i < attempt; i++ {
		d *= mult
	}
	delay := time.Duration(d)
	if c.Config.MaxDelay > 0 && delay > c.Config.MaxDelay {
		delay = c.Config.MaxDelay
	}
	return delay
}

func sleepCtx(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
