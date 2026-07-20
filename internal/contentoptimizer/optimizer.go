package contentoptimizer

import (
	"context"
	"time"
)

// Optimizer orchestrates one optimization run: detect patterns, analyze trends,
// generate recommendations, reason over performance (AI with deterministic
// fallback), analyze prompt templates, assemble the report, persist everything
// append-only, and emit CloudWatch metrics. Runs are idempotent — re-running with
// the same RunID returns the stored report rather than duplicating work.
type Optimizer struct {
	Config      Config
	Repo        Repository
	Patterns    PatternDetector
	Trends      TrendAnalyzer
	Recommender RecommendationEngine
	Prompts     PromptAnalyzer
	Reasoner    ReasoningProvider
	Fallback    ReasoningProvider
	Metrics     MetricsPublisher
	Now         Clock
}

// NewOptimizer wires an optimizer with the default deterministic engines and a
// deterministic reasoner. Supply a ModelReasoner via WithReasoner for AI reasoning.
func NewOptimizer(cfg Config, repo Repository, now Clock) *Optimizer {
	if now == nil {
		now = time.Now
	}
	if repo == nil {
		repo = NewMemoryRepository()
	}
	det := NewDeterministicReasoner()
	return &Optimizer{
		Config:      cfg,
		Repo:        repo,
		Patterns:    NewPatternDetector(cfg),
		Trends:      NewTrendAnalyzer(cfg),
		Recommender: NewRecommendationEngine(cfg),
		Prompts:     NewPromptAnalyzer(cfg),
		Reasoner:    det,
		Fallback:    det,
		Metrics:     nopMetricsPublisher{},
		Now:         now,
	}
}

// WithReasoner sets the primary reasoning provider (e.g. Bedrock/Ollama). The
// deterministic reasoner remains the fallback.
func (o *Optimizer) WithReasoner(r ReasoningProvider) *Optimizer {
	o.Reasoner = r
	return o
}

// Optimize runs the full optimization cycle for the given analytics input.
func (o *Optimizer) Optimize(ctx context.Context, input AnalyticsInput, runID string) (OptimizationReport, error) {
	if len(input.Records) == 0 {
		return OptimizationReport{}, ErrNoData
	}
	if runID == "" {
		runID = shortID("run", input.AsOf, itoaLen(input.Records))
	}
	// Idempotency: return an already-stored run unchanged.
	for _, r := range o.Repo.Reports() {
		if r.RunID == runID {
			return r, nil
		}
	}

	start := o.Now()
	winning, losing := o.Patterns.Detect(input)
	trends := o.Trends.Analyze(input)
	recs := o.Recommender.Recommend(input, winning, losing)
	reasoning := o.reasonWithFallback(ctx, input, winning, losing)
	analyses := o.analyzePrompts(input)

	report := OptimizationReport{
		SchemaVersion:   SchemaVersion,
		RunID:           runID,
		GeneratedAt:     o.Now(),
		AsOf:            input.AsOf,
		WinningPatterns: winning,
		LosingPatterns:  losing,
		Recommendations: recs,
		PromptAnalyses:  analyses,
		Trends:          trends,
		Reasoning:       reasoning,
		Platforms:       input.Platforms,
	}
	report.Confidence = overallConfidence(winning, losing, reasoning)
	report.DataQuality = dataQuality(input)
	report.ExecutiveSummary = executiveSummary(report, input)
	report.PriorityActions = priorityActions(recs, o.Config.topN())
	report.FutureOpps = futureOpportunities(trends)

	o.persist(report)
	o.publishMetrics(ctx, report, o.Now().Sub(start))
	return report, nil
}

// reasonWithFallback tries the primary reasoner up to MaxReasonRetries, then
// falls back to deterministic reasoning — never failing the run.
func (o *Optimizer) reasonWithFallback(ctx context.Context, input AnalyticsInput, winning, losing []Pattern) Reasoning {
	attempts := o.Config.MaxReasonRetries + 1
	for i := 0; i < attempts; i++ {
		rctx := ctx
		var cancel context.CancelFunc
		if o.Config.ReasonTimeout > 0 {
			rctx, cancel = context.WithTimeout(ctx, o.Config.ReasonTimeout)
		}
		reasoning, err := o.Reasoner.Reason(rctx, input, winning, losing)
		if cancel != nil {
			cancel()
		}
		if err == nil {
			return reasoning
		}
		if !isRecoverable(err) {
			break
		}
	}
	// Graceful degradation: deterministic reasoning is always available.
	r, _ := o.Fallback.Reason(ctx, input, winning, losing)
	return r
}

// analyzePrompts analyzes every registered prompt template.
func (o *Optimizer) analyzePrompts(input AnalyticsInput) []PromptAnalysis {
	var out []PromptAnalysis
	for _, id := range o.Repo.PromptTemplates() {
		out = append(out, o.Prompts.Analyze(id, o.Repo.PromptVersions(id), input))
	}
	return out
}

// persist writes the run append-only. A duplicate run is a no-op (idempotent).
func (o *Optimizer) persist(report OptimizationReport) {
	if err := o.Repo.SaveReport(report); err != nil {
		return // duplicate → already persisted
	}
	_ = o.Repo.SaveRecommendations(report.RunID, report.Recommendations)
	for _, p := range append(report.WinningPatterns, report.LosingPatterns...) {
		_ = o.Repo.SavePattern(p)
	}
	for _, t := range report.Trends {
		_ = o.Repo.SaveTrend(report.RunID, t)
	}
}

func (o *Optimizer) publishMetrics(ctx context.Context, report OptimizationReport, dur time.Duration) {
	if !o.Config.PublishToCloud || o.Metrics == nil {
		return
	}
	ts := o.Now()
	promptVer := 0.0
	for _, a := range report.PromptAnalyses {
		if float64(a.CurrentVersion) > promptVer {
			promptVer = float64(a.CurrentVersion)
		}
	}
	trendCount := 0
	for _, t := range report.Trends {
		trendCount += len(t.Emerging) + len(t.Declining)
	}
	_ = o.Metrics.Publish(ctx, o.Config.namespace(), []Metric{
		metric(MetricOptimizationRuns, 1, "Count", nil, ts),
		metric(MetricRecommendationCount, float64(len(report.Recommendations)), "Count", nil, ts),
		metric(MetricPromptVersion, promptVer, "None", nil, ts),
		metric(MetricOptimizationConfidence, report.Confidence, "None", nil, ts),
		metric(MetricTrendDetection, float64(trendCount), "Count", nil, ts),
		metric(MetricOptimizationDuration, float64(dur.Milliseconds()), "Milliseconds", nil, ts),
		metric(MetricRecommendationAccept, o.acceptanceRate(), "Percent", nil, ts),
	})
}

// acceptanceRate is accepted / decided recommendation approvals (%).
func (o *Optimizer) acceptanceRate() float64 {
	var decided, accepted int
	for _, a := range o.Repo.Approvals() {
		if a.Kind != "recommendation" {
			continue
		}
		decided++
		if a.Status == StatusApproved {
			accepted++
		}
	}
	if decided == 0 {
		return 0
	}
	return round2(float64(accepted) / float64(decided) * 100)
}

// itoaLen is a tiny helper so RunID varies with record count deterministically.
func itoaLen(rs []PerformanceRecord) string {
	n := len(rs)
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
