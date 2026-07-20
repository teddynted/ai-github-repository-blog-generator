package contentoptimizer

import (
	"context"
	"time"
)

// PatternDetector identifies winning and losing content patterns from analytics.
// Implementations must be deterministic and ground every pattern in the records.
type PatternDetector interface {
	Detect(input AnalyticsInput) (winning, losing []Pattern)
}

// TrendAnalyzer detects multi-horizon trends (emerging/declining topics,
// publishing windows, seasonal effects, content fatigue) from the input series.
type TrendAnalyzer interface {
	Analyze(input AnalyticsInput) []TrendReport
}

// RecommendationEngine turns patterns + analytics into grounded, confidence-scored
// recommendations. Every recommendation must reference its evidence.
type RecommendationEngine interface {
	Recommend(input AnalyticsInput, winning, losing []Pattern) []Recommendation
}

// PromptAnalyzer analyzes prompt version history and proposes improvements. It
// NEVER auto-applies a change — it returns proposals for human approval.
type PromptAnalyzer interface {
	Analyze(templateID string, versions []PromptVersion, input AnalyticsInput) PromptAnalysis
}

// ReasoningProvider produces grounded structured reasoning about performance.
// It is the interchangeable AI port (Bedrock/Ollama); a deterministic
// implementation is always available as a fallback.
type ReasoningProvider interface {
	Reason(ctx context.Context, input AnalyticsInput, winning, losing []Pattern) (Reasoning, error)
}

// Repository persists optimization outputs. All writes are append-only —
// historical optimization data is never overwritten. In-memory today; a SQLite
// adapter implements the same interface (see docs/content-optimizer-schema.sql).
type Repository interface {
	// SaveReport stores an immutable optimization report (keyed by RunID).
	SaveReport(r OptimizationReport) error
	Reports() []OptimizationReport
	LatestReport() (OptimizationReport, bool)

	// SaveRecommendations appends recommendations to the audit history.
	SaveRecommendations(runID string, recs []Recommendation) error
	Recommendations() []Recommendation

	// Patterns library (append-only).
	SavePattern(p Pattern) error
	Patterns() []Pattern

	// Trend history (append-only).
	SaveTrend(runID string, t TrendReport) error
	Trends() []TrendReport

	// Prompt version repository (append-only; supports rollback by reading history).
	SavePromptVersion(v PromptVersion) error
	PromptVersions(templateID string) []PromptVersion
	LatestPromptVersion(templateID string) (PromptVersion, bool)
	PromptTemplates() []string

	// Approvals audit.
	SaveApproval(a Approval) error
	Approvals() []Approval
}

// Approval is an immutable human decision on a proposal (prompt or recommendation).
type Approval struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"` // prompt | recommendation
	TargetID  string         `json:"targetId"`
	Version   int            `json:"version,omitempty"`
	Status    ApprovalStatus `json:"status"`
	Decider   string         `json:"decider"`
	Reason    string         `json:"reason,omitempty"`
	DecidedAt time.Time      `json:"decidedAt"`
}

// MetricsPublisher is the CloudWatch port. A log adapter ships by default.
type MetricsPublisher interface {
	Publish(ctx context.Context, namespace string, metrics []Metric) error
}

// Metric is one CloudWatch datum.
type Metric struct {
	Name       string
	Value      float64
	Unit       string
	Dimensions map[string]string
	Timestamp  time.Time
}

// Clock is the time port (deterministic in tests).
type Clock func() time.Time
