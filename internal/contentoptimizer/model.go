// Package contentoptimizer is the AI Content Optimization engine (Milestone 18).
// It consumes the historical analytics produced by Milestone 17
// (internal/contentanalytics), identifies high- and low-performing content
// patterns, generates grounded, explainable recommendations, analyzes and
// versions content-generation prompts, detects trends, and produces structured
// optimization intelligence for future content generation.
//
// It follows Clean Architecture and dependency inversion: the domain (this file)
// is pure; the pattern, trend, recommendation, prompt, reasoning, and optimizer
// engines depend only on ports (PatternDetector, TrendAnalyzer,
// RecommendationEngine, PromptAnalyzer, ReasoningProvider, Repository,
// MetricsPublisher, Clock); and the AI reasoning is an interchangeable provider
// (Amazon Bedrock / Anthropic) behind an interface, so future models require no
// business-logic changes.
//
// The optimizer is deterministic, explainable, auditable, reproducible, and
// GROUNDED: every recommendation references the analytics that produced it and
// carries a confidence score. It NEVER invents recommendations that the
// collected analytics cannot support, NEVER overwrites historical optimization
// data, and NEVER auto-replaces a prompt — prompt changes require human approval.
package contentoptimizer

import "time"

// SchemaVersion is the optimization document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Platform mirrors the analytics platform label (kept local to avoid coupling).
type Platform string

// ContentType mirrors the analytics content-type label.
type ContentType string

// Date is an ISO calendar date (YYYY-MM-DD).
type Date = string

// PerformanceRecord is one publication's normalized performance, the atomic unit
// the optimizer reasons over. Typed fields cover the common metrics; Extra
// carries any future metric without an architectural change — the optimizer
// supports new metrics by name.
type PerformanceRecord struct {
	PublicationID  string             `json:"publicationId"`
	Platform       Platform           `json:"platform"`
	ContentType    ContentType        `json:"contentType"`
	Title          string             `json:"title"`
	Tags           []string           `json:"tags,omitempty"`
	Keywords       []string           `json:"keywords,omitempty"`
	PublishedAt    time.Time          `json:"publishedAt,omitempty"`
	Views          int64              `json:"views"`
	Impressions    int64              `json:"impressions,omitempty"`
	CTR            float64            `json:"ctr,omitempty"`
	EngagementRate float64            `json:"engagementRate"`
	RetentionPct   float64            `json:"retentionPct,omitempty"`
	CompletionRate float64            `json:"completionRate,omitempty"`
	WatchMinutes   int64              `json:"watchTimeMinutes,omitempty"`
	Engagements    int64              `json:"engagements,omitempty"`
	Subscribers    int64              `json:"subscribersGained,omitempty"`
	Score          float64            `json:"score"` // M17 blended score (0–100)
	Extra          map[string]float64 `json:"extra,omitempty"`
}

// metric returns a named metric value, checking typed fields then Extra. This is
// how the optimizer supports future metrics without new code paths.
func (r PerformanceRecord) metric(name string) (float64, bool) {
	switch name {
	case "views":
		return float64(r.Views), true
	case "impressions":
		return float64(r.Impressions), true
	case "ctr":
		return r.CTR, true
	case "engagementRate":
		return r.EngagementRate, true
	case "retentionPct":
		return r.RetentionPct, true
	case "completionRate":
		return r.CompletionRate, true
	case "watchMinutes":
		return float64(r.WatchMinutes), true
	case "engagements":
		return float64(r.Engagements), true
	case "subscribers":
		return float64(r.Subscribers), true
	case "score":
		return r.Score, true
	}
	if r.Extra != nil {
		v, ok := r.Extra[name]
		return v, ok
	}
	return 0, false
}

// PlatformStat is one platform's aggregate performance (from M17).
type PlatformStat struct {
	Platform        Platform `json:"platform"`
	Publications    int      `json:"publications"`
	Views           int64    `json:"views"`
	Engagements     int64    `json:"engagements"`
	EngagementRate  float64  `json:"engagementRate"`
	CTR             float64  `json:"ctr,omitempty"`
	AvgRetentionPct float64  `json:"avgRetentionPct,omitempty"`
	WatchMinutes    int64    `json:"watchTimeMinutes,omitempty"`
}

// TagStat / TopicStat / TimeStat mirror M17 aggregate signals.
type TagStat struct {
	Tag            string  `json:"tag"`
	Publications   int     `json:"publications"`
	Views          int64   `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
}
type TopicStat struct {
	Keyword        string  `json:"keyword"`
	Publications   int     `json:"publications"`
	Views          int64   `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
}
type TimeStat struct {
	Weekday        string  `json:"weekday"`
	Hour           int     `json:"hour"`
	Publications   int     `json:"publications"`
	AvgViews       float64 `json:"avgViews"`
	EngagementRate float64 `json:"engagementRate"`
}

// SeriesPoint is a (date, value) sample in a historical trend series.
type SeriesPoint struct {
	Date  Date    `json:"date"`
	Value float64 `json:"value"`
}

// AnalyticsInput is the grounded input the optimizer consumes. It is populated
// from Milestone 17 output via FromContentAnalytics, but is defined
// independently so the optimizer is not coupled to the analytics package and can
// be fed from any source.
type AnalyticsInput struct {
	AsOf        Date                     `json:"asOf"`
	Records     []PerformanceRecord      `json:"records"`
	Platforms   []PlatformStat           `json:"platforms"`
	TopTags     []TagStat                `json:"topTags"`
	TopKeywords []TopicStat              `json:"topKeywords"`
	BestTimes   []TimeStat               `json:"bestPublishingTimes"`
	Series      map[string][]SeriesPoint `json:"series,omitempty"` // metric -> daily totals
}

// PatternKind classifies a detected pattern.
type PatternKind string

const (
	PatternWinning PatternKind = "winning"
	PatternLosing  PatternKind = "losing"
)

// Pattern is a detected content pattern with a human-readable signal chain and
// the evidence that grounds it.
type Pattern struct {
	ID          string      `json:"id"`
	Kind        PatternKind `json:"kind"`
	Name        string      `json:"name"`
	SignalChain []string    `json:"signalChain"` // e.g. High CTR → Strong Hook → Long Watch → Subs
	Description string      `json:"description"`
	Support     int         `json:"support"` // number of records supporting it
	Confidence  float64     `json:"confidence"`
	Evidence    []string    `json:"evidence"`
	Examples    []string    `json:"examples,omitempty"` // publication ids/titles
}

// RecommendationCategory groups recommendations by area.
type RecommendationCategory string

const (
	CatContentStrategy RecommendationCategory = "content-strategy"
	CatPublishing      RecommendationCategory = "publishing-strategy"
	CatSEO             RecommendationCategory = "seo"
	CatSocial          RecommendationCategory = "social-strategy"
	CatVideo           RecommendationCategory = "video"
	CatNarration       RecommendationCategory = "narration"
	CatArchitecture    RecommendationCategory = "architecture-visualization"
	CatThumbnail       RecommendationCategory = "thumbnail"
	CatPrompt          RecommendationCategory = "prompt"
	CatPlatform        RecommendationCategory = "platform"
)

// Recommendation is a grounded, actionable improvement with a confidence score
// and the analytics evidence that produced it.
type Recommendation struct {
	ID             string                 `json:"id"`
	Category       RecommendationCategory `json:"category"`
	Title          string                 `json:"title"`
	Detail         string                 `json:"detail"`
	Rationale      string                 `json:"rationale"`
	Evidence       []string               `json:"evidence"` // metrics that ground it
	Confidence     float64                `json:"confidence"`
	ExpectedImpact string                 `json:"expectedImpact"` // low | medium | high
	Priority       int                    `json:"priority"`       // 1 = highest
	PatternID      string                 `json:"patternId,omitempty"`
}

// Reasoning is grounded structured reasoning about performance (deterministic or
// AI-produced). Every field must trace to the analytics.
type Reasoning struct {
	Reviewer       string   `json:"reviewer"`
	WhyWell        []string `json:"whyPerformedWell"`
	WhyUnder       []string `json:"whyUnderperformed"`
	Evidence       []string `json:"supportingEvidence"`
	TradeOffs      []string `json:"tradeOffs,omitempty"`
	Improvements   []string `json:"suggestedImprovements"`
	Confidence     float64  `json:"confidence"`
	ExpectedImpact string   `json:"expectedImpact"`
}

// TrendDirection classifies a series' movement.
type TrendDirection string

const (
	TrendEmerging  TrendDirection = "emerging"
	TrendDeclining TrendDirection = "declining"
	TrendStable    TrendDirection = "stable"
)

// TopicTrend is one topic/tag's trajectory.
type TopicTrend struct {
	Topic      string         `json:"topic"`
	Direction  TrendDirection `json:"direction"`
	ChangePct  float64        `json:"changePct"`
	Recent     float64        `json:"recent"`
	Prior      float64        `json:"prior"`
	Confidence float64        `json:"confidence"`
}

// TrendReport summarizes multi-horizon trend analysis.
type TrendReport struct {
	Horizon           string       `json:"horizon"` // daily | weekly | monthly | quarterly | long-term
	Emerging          []TopicTrend `json:"emergingTopics"`
	Declining         []TopicTrend `json:"decliningTopics"`
	PublishingWindows []string     `json:"publishingWindows,omitempty"`
	SeasonalNotes     []string     `json:"seasonalNotes,omitempty"`
	ContentFatigue    []string     `json:"contentFatigue,omitempty"`
}

// ApprovalStatus is a prompt version's governance state.
type ApprovalStatus string

const (
	StatusDraft    ApprovalStatus = "draft"
	StatusProposed ApprovalStatus = "proposed"
	StatusApproved ApprovalStatus = "approved"
	StatusRejected ApprovalStatus = "rejected"
	StatusRetired  ApprovalStatus = "retired"
)

// PromptMetrics are a prompt version's measured outcomes.
type PromptMetrics struct {
	UsageCount        int     `json:"usageCount"`
	AvgCTR            float64 `json:"avgCtr"`
	AvgEngagementRate float64 `json:"avgEngagementRate"`
	AvgWatchMinutes   float64 `json:"avgWatchMinutes"`
	AvgReadingMinutes float64 `json:"avgReadingMinutes"`
	AvgScore          float64 `json:"avgScore"`
}

// PromptVersion is one immutable version of a content-generation prompt template.
// Versions are append-only; a new version never overwrites a prior one, so
// rollback is always possible.
type PromptVersion struct {
	TemplateID   string         `json:"templateId"`
	Version      int            `json:"version"`
	Body         string         `json:"body"`
	Notes        string         `json:"notes,omitempty"`
	Status       ApprovalStatus `json:"status"`
	Metrics      PromptMetrics  `json:"metrics"`
	CreatedAt    time.Time      `json:"createdAt"`
	ApprovedBy   string         `json:"approvedBy,omitempty"`
	ApprovedAt   *time.Time     `json:"approvedAt,omitempty"`
	SupersededBy int            `json:"supersededBy,omitempty"`
}

// PromptRecommendation proposes an improvement to a prompt template. It is a
// proposal only — it is never auto-applied; a human approves it, which creates a
// new PromptVersion.
type PromptRecommendation struct {
	TemplateID   string   `json:"templateId"`
	BaseVersion  int      `json:"baseVersion"`
	Aspect       string   `json:"aspect"` // hook | intro | storytelling | technical | seo | cta | formatting | pacing | transitions | educational-flow
	Suggestion   string   `json:"suggestion"`
	Rationale    string   `json:"rationale"`
	Evidence     []string `json:"evidence"`
	Confidence   float64  `json:"confidence"`
	ProposedBody string   `json:"proposedBody,omitempty"`
}

// PromptAnalysis is the result of analyzing a template's version history.
type PromptAnalysis struct {
	TemplateID      string                 `json:"templateId"`
	CurrentVersion  int                    `json:"currentVersion"`
	Versions        int                    `json:"versionCount"`
	BestVersion     int                    `json:"bestVersion"`
	Trend           string                 `json:"trend"` // improving | regressing | flat
	Recommendations []PromptRecommendation `json:"recommendations"`
}

// OptimizationReport is the top-level structured optimization intelligence.
type OptimizationReport struct {
	SchemaVersion    string           `json:"schemaVersion"`
	RunID            string           `json:"runId"`
	GeneratedAt      time.Time        `json:"generatedAt"`
	AsOf             Date             `json:"asOf"`
	ExecutiveSummary string           `json:"executiveSummary"`
	WinningPatterns  []Pattern        `json:"winningPatterns"`
	LosingPatterns   []Pattern        `json:"losingPatterns"`
	Recommendations  []Recommendation `json:"recommendations"`
	PromptAnalyses   []PromptAnalysis `json:"promptAnalyses"`
	Trends           []TrendReport    `json:"trends"`
	Reasoning        Reasoning        `json:"reasoning"`
	Platforms        []PlatformStat   `json:"platformComparison"`
	PriorityActions  []string         `json:"priorityImprovements"`
	FutureOpps       []string         `json:"futureOpportunities"`
	Confidence       float64          `json:"confidence"` // overall run confidence
	DataQuality      DataQuality      `json:"dataQuality"`
}

// DataQuality reports how much analytics grounded the run.
type DataQuality struct {
	Records          int     `json:"records"`
	Platforms        int     `json:"platforms"`
	SeriesMetrics    int     `json:"seriesMetrics"`
	CompletenessNote string  `json:"completenessNote"`
	Sufficiency      float64 `json:"sufficiency"` // 0–100
}
