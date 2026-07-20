package contentanalytics

import (
	"context"
	"net/http"
	"time"
)

// AnalyticsProvider is the interchangeable platform-adapter port. Each platform
// implements it, so new platforms are added without changing the collector,
// trend engine, or reporting. Providers must never fabricate metrics — they
// return exactly what the platform API reports, and signal absent fields via
// ErrUnsupported rather than inventing values.
type AnalyticsProvider interface {
	// Name is the platform this provider targets.
	Name() Platform
	// Supports reports whether the provider can measure a content type.
	Supports(t ContentType) bool
	// FetchPublication refreshes a publication's platform-side facts (URL,
	// status, last-updated) — the publication half of the join.
	FetchPublication(ctx context.Context, p Publication) (Publication, error)
	// FetchMetrics returns the fully normalized snapshot for a publication on a
	// date. It composes reach/engagement/watch/click/growth. A provider MAY
	// implement this directly or delegate to the finer-grained fetchers below.
	FetchMetrics(ctx context.Context, p Publication, date Date) (MetricsSnapshot, error)
}

// ReachFetcher / EngagementFetcher / WatchFetcher / AudienceFetcher are optional
// finer-grained capabilities. A provider composes whichever the platform
// exposes; the collector treats an unimplemented capability as "not reported".
type ReachFetcher interface {
	FetchReach(ctx context.Context, p Publication, date Date) (Reach, error)
}
type EngagementFetcher interface {
	FetchEngagement(ctx context.Context, p Publication, date Date) (Engagement, error)
}
type WatchFetcher interface {
	FetchWatchTime(ctx context.Context, p Publication, date Date) (Watch, error)
}
type AudienceFetcher interface {
	FetchAudience(ctx context.Context, p Publication, date Date) (Growth, []TrafficSource, error)
}

// Repository persists publications and immutable snapshots. In-memory today; a
// SQLite adapter implements the same interface (see docs/content-analytics-schema.sql).
// Snapshots are append-only: a duplicate (publication, period, date) is rejected,
// never overwritten.
type Repository interface {
	// SavePublication upserts the publication record (its facts may change:
	// title, URL, status) — this is not historical, it is the current record.
	SavePublication(p Publication) error
	// Publications returns all tracked publications.
	Publications() []Publication
	// PublicationsByPlatform filters by platform.
	PublicationsByPlatform(pl Platform) []Publication

	// SaveSnapshot stores an immutable metrics snapshot. Returns ErrDuplicateSnapshot
	// if one already exists for (publicationID, period, date).
	SaveSnapshot(s MetricsSnapshot) error
	// SnapshotExists reports whether a snapshot is already stored (idempotency).
	SnapshotExists(publicationID string, period Period, date Date) bool
	// Snapshots returns snapshots for a publication+period, oldest first.
	Snapshots(publicationID string, period Period) []MetricsSnapshot
	// SnapshotsOn returns all snapshots for a period on a specific date.
	SnapshotsOn(period Period, date Date) []MetricsSnapshot
	// LatestSnapshot returns the most recent snapshot for a publication+period.
	LatestSnapshot(publicationID string, period Period) (MetricsSnapshot, bool)
	// Platforms returns the platforms with stored data.
	Platforms() []Platform
}

// MetricsPublisher is the CloudWatch port. A log adapter ships by default; a
// real CloudWatch PutMetricData adapter implements the same interface.
type MetricsPublisher interface {
	// Publish emits a batch of custom metrics under a namespace.
	Publish(ctx context.Context, namespace string, metrics []Metric) error
}

// Metric is one CloudWatch datum.
type Metric struct {
	Name       string
	Value      float64
	Unit       string // Count | Percent | Seconds | Milliseconds | None
	Dimensions map[string]string
	Timestamp  time.Time
}

// Clock is the time port (deterministic in tests).
type Clock func() time.Time

// Sleeper is the delay port, so retry/backoff is instant in tests.
type Sleeper func(ctx context.Context, d time.Duration)

// HTTPDoer is the HTTP port provider adapters use, so they are testable with a
// mock and never require real network/credentials in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
