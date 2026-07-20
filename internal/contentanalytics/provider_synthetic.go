package contentanalytics

import (
	"context"
	"hash/fnv"
	"time"
)

// SyntheticProvider is a deterministic AnalyticsProvider for tests and offline
// demos. It fabricates NO real-world data — it generates reproducible synthetic
// metrics from the publication id and date, so the whole pipeline (collection,
// snapshots, trends, reports, dashboards) can run with no network or secrets.
//
// It implements the fine-grained fetcher capabilities, so FetchMetrics exercises
// the same composeMetrics normalization path a real multi-endpoint provider uses.
type SyntheticProvider struct {
	Plat       Platform
	Start      Date
	BaseViews  int64
	DailyViews int64
	EngRate    float64 // engagements as a fraction of views
	Video      bool    // include watch metrics
	Supported  []ContentType

	// Failure injection for retry/backoff and auth-alarm tests.
	FailFirst   int  // fail this many times with a recoverable error, then succeed
	Recoverable bool // whether injected failures are recoverable
	Auth        bool // always fail with a permanent auth error

	failed int
}

// NewSyntheticProvider builds a synthetic provider for a platform.
func NewSyntheticProvider(pl Platform, start Date, baseViews, dailyViews int64, video bool) *SyntheticProvider {
	return &SyntheticProvider{
		Plat: pl, Start: start, BaseViews: baseViews, DailyViews: dailyViews,
		EngRate: 0.08, Video: video, Recoverable: true,
	}
}

func (p *SyntheticProvider) Name() Platform { return p.Plat }

func (p *SyntheticProvider) Supports(t ContentType) bool {
	if len(p.Supported) == 0 {
		return true
	}
	for _, s := range p.Supported {
		if s == t {
			return true
		}
	}
	return false
}

// offset makes each publication's numbers distinct but deterministic.
func offsetOf(id string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum32() % 500)
}

func (p *SyntheticProvider) views(pub Publication, date Date) int64 {
	days := daysBetween(p.Start, date)
	if days < 0 {
		days = 0
	}
	return p.BaseViews + offsetOf(pub.ID) + p.DailyViews*int64(days)
}

func (p *SyntheticProvider) FetchPublication(ctx context.Context, pub Publication) (Publication, error) {
	if p.Auth {
		return pub, errAuth("synthetic auth failure")
	}
	pub.Platform = p.Plat
	pub.Status = "published"
	if pub.PublishedAt.IsZero() {
		pub.PublishedAt = parseDate(p.Start)
	}
	return pub, nil
}

// injectFailure applies the configured failure injection.
func (p *SyntheticProvider) injectFailure() error {
	if p.Auth {
		return errAuth("synthetic auth failure")
	}
	if p.failed < p.FailFirst {
		p.failed++
		if p.Recoverable {
			return errServer("synthetic transient failure")
		}
		return errMissing("synthetic permanent failure")
	}
	return nil
}

func (p *SyntheticProvider) FetchReach(ctx context.Context, pub Publication, date Date) (Reach, error) {
	v := p.views(pub, date)
	return Reach{Views: v, Impressions: v * 3, UniqueViewers: v * 8 / 10, AudienceReach: v * 12 / 10}, nil
}

func (p *SyntheticProvider) FetchEngagement(ctx context.Context, pub Publication, date Date) (Engagement, error) {
	v := p.views(pub, date)
	total := int64(float64(v) * p.EngRate)
	return Engagement{
		Likes:    total * 6 / 10,
		Comments: total * 2 / 10,
		Shares:   total * 1 / 10,
		Saves:    total * 1 / 10,
	}, nil
}

func (p *SyntheticProvider) FetchWatchTime(ctx context.Context, pub Publication, date Date) (Watch, error) {
	if !p.Video {
		return Watch{}, ErrUnsupported
	}
	v := p.views(pub, date)
	return Watch{
		WatchTimeMinutes: v * 4,
		AvgViewSeconds:   210,
		RetentionPct:     55 + float64(offsetOf(pub.ID)%20),
		CompletionRate:   48,
	}, nil
}

func (p *SyntheticProvider) FetchAudience(ctx context.Context, pub Publication, date Date) (Growth, []TrafficSource, error) {
	v := p.views(pub, date)
	g := Growth{SubscribersGained: v / 100, FollowersGained: v / 120}
	src := []TrafficSource{
		{Source: "search", Views: v * 4 / 10, Percent: 40},
		{Source: "feed", Views: v * 35 / 100, Percent: 35},
		{Source: "external", Views: v * 25 / 100, Percent: 25},
	}
	return g, src, nil
}

func (p *SyntheticProvider) FetchMetrics(ctx context.Context, pub Publication, date Date) (MetricsSnapshot, error) {
	if err := p.injectFailure(); err != nil {
		return MetricsSnapshot{}, err
	}
	s, err := composeMetrics(ctx, p, pub, date)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	// Add a link-click signal so CTR is exercised on the article-style platforms.
	if !p.Video {
		s.Click.LinkClicks = s.Reach.Views / 20
	} else {
		s.Click.ThumbnailCTR = 6.5
	}
	s.CollectedAt = time.Now()
	return finalizeSnapshot(s, p.Plat, pub, date), nil
}
