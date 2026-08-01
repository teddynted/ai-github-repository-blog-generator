package socialintel

import (
	"context"
	"net/http"
	"time"
)

// Provider is the interchangeable platform-adapter port. Each social platform
// implements it, so new platforms are added without changing the collector or
// analytics. Providers must never fabricate metrics — they return what the
// platform API reports.
type Provider interface {
	// Name is the platform this provider targets.
	Name() Platform
	// CollectAccount returns the account-level snapshot for a date.
	CollectAccount(ctx context.Context, date Date) (AccountSnapshot, error)
	// CollectContent returns the per-content snapshots for a date.
	CollectContent(ctx context.Context, date Date) ([]ContentSnapshot, error)
}

// Repository persists immutable snapshots. Historical metrics are append-only
// and never overwritten. In-memory today; a SQLite adapter implements the same
// interface (see docs/social-intel-schema.sql).
type Repository interface {
	// SaveAccountSnapshot stores a snapshot. It must be idempotent per
	// (platform, date) — a duplicate for the same day is rejected, not overwritten.
	SaveAccountSnapshot(s AccountSnapshot) error
	SaveContentSnapshot(s ContentSnapshot) error
	// AccountSnapshotExists reports whether a snapshot already exists (dedupe).
	AccountSnapshotExists(p Platform, date Date) bool
	// AccountSeries returns account snapshots for a platform, oldest first.
	AccountSeries(p Platform) []AccountSnapshot
	// ContentSnapshots returns content snapshots for a platform on a date.
	ContentSnapshots(p Platform, date Date) []ContentSnapshot
	// ContentHistory returns all snapshots for one content item, oldest first.
	ContentHistory(p Platform, contentID string) []ContentSnapshot
	// Platforms returns the platforms with stored data.
	Platforms() []Platform
}

// AIInsighter is the grounded AI-insight port (Bedrock/Anthropic). It receives the
// collected metrics and returns qualitative notes; it must ground everything in
// those metrics and never invent statistics.
type AIInsighter interface {
	Insights(ctx context.Context, grounding InsightGrounding) (AINotes, error)
}

// InsightGrounding is the metric context passed to the AI insighter.
type InsightGrounding struct {
	Date          Date
	Performance   []PlatformPerformance
	Growth        []GrowthMetrics
	TopContent    []ContentEffectiveness
	CrossPlatform CrossPlatform
}

// AINotes are the AI insighter's qualitative outputs.
type AINotes struct {
	Reviewer        string            `json:"reviewer"`
	Summary         string            `json:"summary"`
	Insights        []BusinessInsight `json:"insights"`
	Recommendations []Recommendation  `json:"recommendations"`
}

// Monitor is the observability port (CloudWatch). A log adapter ships by default.
type Monitor interface {
	Count(name string, value float64, dims map[string]string)
	Duration(name string, d time.Duration, dims map[string]string)
	Error(name string, err error, dims map[string]string)
}

// Clock is the time port (deterministic in tests).
type Clock func() time.Time

// HTTPDoer is the HTTP port provider adapters use, so they are testable with a
// mock and never require real network/credentials in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
