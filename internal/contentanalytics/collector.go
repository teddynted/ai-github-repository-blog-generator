package contentanalytics

import (
	"context"
	"time"
)

// Collector retrieves performance metrics for tracked publications, normalizes
// them, and persists immutable daily snapshots. It is idempotent (never
// re-collects a day already stored), retries recoverable failures with
// exponential backoff, tolerates partial failures (one platform failing does not
// abort the run), and emits CloudWatch metrics for the whole collection.
type Collector struct {
	Config    Config
	Repo      Repository
	Metrics   MetricsPublisher
	Now       Clock
	Sleep     Sleeper
	providers map[Platform]AnalyticsProvider
}

// NewCollector wires a collector. A nil repo/metrics/clock get sane defaults.
func NewCollector(cfg Config, repo Repository, providers []AnalyticsProvider, now Clock) *Collector {
	if now == nil {
		now = time.Now
	}
	if repo == nil {
		repo = NewMemoryRepository()
	}
	c := &Collector{
		Config:    cfg,
		Repo:      repo,
		Metrics:   nopMetricsPublisher{},
		Now:       now,
		Sleep:     realSleeper,
		providers: map[Platform]AnalyticsProvider{},
	}
	for _, p := range providers {
		c.providers[p.Name()] = p
	}
	return c
}

// Register adds/replaces a provider — a new platform needs no core changes.
func (c *Collector) Register(p AnalyticsProvider) { c.providers[p.Name()] = p }

// ItemResult is one publication's collection outcome.
type ItemResult struct {
	PublicationID string   `json:"publicationId"`
	Platform      Platform `json:"platform"`
	Collected     bool     `json:"collected"`
	Skipped       bool     `json:"skipped"`
	Partial       bool     `json:"partial"`
	Retries       int      `json:"retries"`
	Error         string   `json:"error,omitempty"`
	Code          string   `json:"code,omitempty"`
}

// CollectResult reports the outcome of a collection run.
type CollectResult struct {
	Date       Date         `json:"date"`
	Collected  int          `json:"collected"`
	Skipped    int          `json:"skipped"`
	Failed     int          `json:"failed"`
	Partial    int          `json:"partial"`
	DailyViews int64        `json:"dailyViews"`
	DurationMs int64        `json:"durationMs"`
	Items      []ItemResult `json:"items"`
}

// SuccessRate is collected / attempted (collected+failed) as a percentage.
func (r CollectResult) SuccessRate() float64 {
	attempted := r.Collected + r.Failed
	if attempted == 0 {
		return 100
	}
	return round2(float64(r.Collected) / float64(attempted) * 100)
}

// CollectToday collects for today's date.
func (c *Collector) CollectToday(ctx context.Context) CollectResult {
	return c.Collect(ctx, dateOf(c.Now()))
}

// Collect measures every tracked, enabled publication for a date. A publication
// whose platform has no provider or is disabled is silently skipped.
func (c *Collector) Collect(ctx context.Context, date Date) CollectResult {
	start := c.Now()
	res := CollectResult{Date: date}
	var apiFailures, authFailures int

	for _, pub := range c.Repo.Publications() {
		if !c.Config.isEnabled(pub.Platform) {
			continue
		}
		prov, ok := c.providers[pub.Platform]
		if !ok {
			continue
		}
		item := c.collectOne(ctx, prov, pub, date)
		res.Items = append(res.Items, item)
		switch {
		case item.Skipped:
			res.Skipped++
		case item.Collected:
			res.Collected++
			if item.Partial {
				res.Partial++
			}
		default:
			res.Failed++
			apiFailures++
			if item.Code == "auth" {
				authFailures++
			}
		}
	}

	// Sum daily views actually collected for the CloudWatch DailyViews metric.
	for _, s := range c.Repo.SnapshotsOn(PeriodDaily, date) {
		res.DailyViews += s.Reach.Views
	}
	res.DurationMs = c.Now().Sub(start).Milliseconds()

	c.publishRunMetrics(ctx, res, apiFailures, authFailures)
	return res
}

// collectOne collects one publication with dedupe + retry/backoff.
func (c *Collector) collectOne(ctx context.Context, prov AnalyticsProvider, pub Publication, date Date) ItemResult {
	item := ItemResult{PublicationID: pub.ID, Platform: pub.Platform}

	// Idempotency: never re-collect a day already stored.
	if c.Repo.SnapshotExists(pub.ID, PeriodDaily, date) {
		item.Skipped = true
		return item
	}

	attemptStart := c.Now()
	var (
		snap    MetricsSnapshot
		lastErr error
	)
	maxAttempts := c.Config.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			item.Retries++
			c.Sleep(ctx, c.backoff(attempt-1))
		}
		s, err := prov.FetchMetrics(ctx, pub, date)
		if err != nil {
			lastErr = err
			if !isRecoverable(err) {
				break
			}
			continue
		}
		snap, lastErr = s, nil
		break
	}
	c.emitLatency(ctx, pub.Platform, c.Now().Sub(attemptStart))

	if lastErr != nil {
		item.Error = lastErr.Error()
		item.Code = errCode(lastErr)
		return item
	}

	snap = finalizeSnapshot(snap, pub.Platform, pub, date)
	snap.Period = PeriodDaily
	if snap.CollectedAt.IsZero() {
		snap.CollectedAt = c.Now()
	}
	if err := c.Repo.SaveSnapshot(snap); err != nil {
		item.Error = err.Error()
		item.Code = errCode(err)
		return item
	}
	item.Collected = true
	item.Partial = snap.Partial
	return item
}

func (c *Collector) backoff(attempt int) time.Duration {
	base := c.Config.BaseDelay
	if base <= 0 {
		base = 500 * time.Millisecond
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

// publishRunMetrics emits the collection-level CloudWatch metrics.
func (c *Collector) publishRunMetrics(ctx context.Context, res CollectResult, apiFailures, authFailures int) {
	if !c.Config.PublishToCloud || c.Metrics == nil {
		return
	}
	ts := c.Now()
	ns := c.Config.namespace()
	missing := 0
	if res.Collected == 0 && (res.Failed > 0 || res.Skipped == 0) {
		missing = 1
	}
	_ = c.Metrics.Publish(ctx, ns, []Metric{
		metric(MetricDailyViews, float64(res.DailyViews), "Count", nil, ts),
		metric(MetricPublishingSuccessRate, res.SuccessRate(), "Percent", nil, ts),
		metric(MetricFailedCollections, float64(res.Failed), "Count", nil, ts),
		metric(MetricAPIFailures, float64(apiFailures), "Count", nil, ts),
		metric(MetricAuthFailures, float64(authFailures), "Count", nil, ts),
		metric(MetricCollectionDuration, float64(res.DurationMs), "Milliseconds", nil, ts),
		metric(MetricMissingAnalytics, float64(missing), "Count", nil, ts),
	})
}

func (c *Collector) emitLatency(ctx context.Context, platform Platform, d time.Duration) {
	if !c.Config.PublishToCloud || c.Metrics == nil {
		return
	}
	_ = c.Metrics.Publish(ctx, c.Config.namespace(), []Metric{
		metric(MetricAPILatency, float64(d.Milliseconds()), "Milliseconds", map[string]string{"platform": string(platform)}, c.Now()),
	})
}

// realSleeper waits, respecting context cancellation.
func realSleeper(ctx context.Context, d time.Duration) {
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
