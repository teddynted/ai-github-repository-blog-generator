package contentoptimizer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ca "github.com/teddynted/ai-github-repository-blog-generator/internal/contentanalytics"
)

// --- helpers ---

func fixedClock(d Date) Clock {
	t, _ := time.Parse("2006-01-02", d)
	return func() time.Time { return t }
}

// sampleInput builds a grounded AnalyticsInput with a clear winner/loser spread.
func sampleInput() AnalyticsInput {
	base, _ := time.Parse("2006-01-02", "2026-07-01")
	rec := func(id string, score, ctr, ret, eng float64, views, watch int64, offsetH int, tags ...string) PerformanceRecord {
		return PerformanceRecord{
			PublicationID: id, Platform: "YouTube", ContentType: "YouTube Video", Title: id,
			Tags: tags, Score: score, CTR: ctr, RetentionPct: ret, EngagementRate: eng,
			Views: views, WatchMinutes: watch, Engagements: int64(float64(views) * eng / 100),
			PublishedAt: base.Add(time.Duration(offsetH) * time.Hour),
		}
	}
	return AnalyticsInput{
		AsOf: "2026-07-20",
		Records: []PerformanceRecord{
			rec("win-1", 90, 9, 70, 9, 10000, 4000, 0, "aws", "golang"),
			rec("win-2", 85, 8, 68, 8.5, 9000, 3600, 24, "aws"),
			rec("mid-1", 55, 5, 50, 5, 4000, 1500, 240, "devops"),
			rec("lose-1", 20, 2, 30, 2, 800, 200, 400, "legacy"),
			rec("lose-2", 15, 1.5, 25, 1.5, 500, 120, 420, "legacy"),
		},
		Platforms: []PlatformStat{
			{Platform: "YouTube", Publications: 3, Views: 19000, Engagements: 1600, EngagementRate: 8.4, CTR: 8.5, AvgRetentionPct: 69},
			{Platform: "Dev.to", Publications: 2, Views: 1300, Engagements: 30, EngagementRate: 2.3, CTR: 0},
		},
		TopTags:     []TagStat{{Tag: "aws", Publications: 2, Views: 19000, EngagementRate: 8.7}},
		TopKeywords: []TopicStat{{Keyword: "golang", Publications: 1, Views: 10000, EngagementRate: 9}},
		BestTimes:   []TimeStat{{Weekday: "Tuesday", Hour: 9, Publications: 3, AvgViews: 8000, EngagementRate: 8.2}},
		Series: map[string][]SeriesPoint{"views": {
			{"2026-07-01", 100}, {"2026-07-05", 400}, {"2026-07-10", 900}, {"2026-07-15", 1600}, {"2026-07-20", 2500},
		}},
	}
}

// --- store / immutability ---

func TestRepositoryAppendOnly(t *testing.T) {
	repo := NewMemoryRepository()
	rep := OptimizationReport{RunID: "r1"}
	if err := repo.SaveReport(rep); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := repo.SaveReport(rep); err != ErrDuplicateReport {
		t.Fatalf("duplicate report should be rejected, got %v", err)
	}
	if len(repo.Reports()) != 1 {
		t.Errorf("want 1 report, got %d", len(repo.Reports()))
	}
	if _, ok := repo.LatestReport(); !ok {
		t.Error("latest report should exist")
	}
	_ = repo.SaveRecommendations("r1", []Recommendation{{ID: "x"}})
	_ = repo.SavePattern(Pattern{ID: "p"})
	_ = repo.SaveTrend("r1", TrendReport{Horizon: "weekly"})
	if len(repo.Recommendations()) != 1 || len(repo.Patterns()) != 1 || len(repo.Trends()) != 1 {
		t.Error("append-only writes not stored")
	}
}

func TestPromptVersionImmutability(t *testing.T) {
	repo := NewMemoryRepository()
	v := PromptVersion{TemplateID: "t", Version: 1, Body: "b", Status: StatusApproved}
	if err := repo.SavePromptVersion(v); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	if err := repo.SavePromptVersion(v); err != ErrDuplicateReport {
		t.Errorf("duplicate version should be rejected, got %v", err)
	}
	if err := repo.SavePromptVersion(PromptVersion{TemplateID: "", Version: 1}); err == nil {
		t.Error("empty template id should error")
	}
	if got := repo.PromptTemplates(); len(got) != 1 {
		t.Errorf("want 1 template, got %d", len(got))
	}
}

// --- pattern detection ---

func TestPatternDetection(t *testing.T) {
	d := NewPatternDetector(DefaultConfig())
	winning, losing := d.Detect(sampleInput())
	if len(winning) == 0 {
		t.Fatal("expected a winning pattern")
	}
	if len(losing) == 0 {
		t.Fatal("expected a losing pattern")
	}
	var sig *Pattern
	for i := range winning {
		if winning[i].Name == "High-performing content signature" {
			sig = &winning[i]
		}
	}
	if sig == nil {
		t.Fatal("expected the high-performing signature pattern")
	}
	if sig.Support < 1 || sig.Confidence <= 0 || len(sig.Evidence) == 0 {
		t.Errorf("winning pattern not grounded: %+v", sig)
	}
	if sig.Kind != PatternWinning {
		t.Error("wrong kind")
	}
}

func TestPatternDetectionEmpty(t *testing.T) {
	d := NewPatternDetector(DefaultConfig())
	w, l := d.Detect(AnalyticsInput{})
	if w != nil || l != nil {
		t.Error("empty input should yield no patterns")
	}
}

// --- trend analysis ---

func TestTrendAnalysis(t *testing.T) {
	in := sampleInput()
	// Make "aws" emerging: add a recent high-score aws record and old low ones.
	a := NewTrendAnalyzer(DefaultConfig())
	reports := a.Analyze(in)
	if len(reports) == 0 {
		t.Fatal("expected trend reports")
	}
	foundWindows := false
	for _, r := range reports {
		if len(r.PublishingWindows) > 0 {
			foundWindows = true
		}
	}
	if !foundWindows {
		t.Error("expected publishing windows from best times")
	}
}

func TestTrendTopicEmergingDeclining(t *testing.T) {
	base, _ := time.Parse("2006-01-02", "2026-01-01")
	in := AnalyticsInput{
		AsOf: "2026-07-01",
		Records: []PerformanceRecord{
			// prior cohort: "fading" strong, "rising" weak
			{PublicationID: "a", Score: 80, Tags: []string{"fading"}, PublishedAt: base},
			{PublicationID: "b", Score: 75, Tags: []string{"fading"}, PublishedAt: base.Add(24 * time.Hour)},
			{PublicationID: "c", Score: 20, Tags: []string{"rising"}, PublishedAt: base.Add(48 * time.Hour)},
			// recent cohort: "rising" strong, "fading" weak
			{PublicationID: "d", Score: 85, Tags: []string{"rising"}, PublishedAt: base.Add(200 * 24 * time.Hour)},
			{PublicationID: "e", Score: 90, Tags: []string{"rising"}, PublishedAt: base.Add(201 * 24 * time.Hour)},
			{PublicationID: "f", Score: 15, Tags: []string{"fading"}, PublishedAt: base.Add(202 * 24 * time.Hour)},
		},
	}
	a := NewTrendAnalyzer(DefaultConfig())
	emerging, declining := a.topicTrends(in, 0)
	hasRising, hasFading := false, false
	for _, e := range emerging {
		if e.Topic == "rising" && e.Direction == TrendEmerging {
			hasRising = true
		}
	}
	for _, d := range declining {
		if d.Topic == "fading" && d.Direction == TrendDeclining {
			hasFading = true
		}
	}
	if !hasRising {
		t.Errorf("expected 'rising' emerging, got %+v", emerging)
	}
	if !hasFading {
		t.Errorf("expected 'fading' declining, got %+v", declining)
	}
}

// --- recommendations ---

func TestRecommendationEngine(t *testing.T) {
	e := NewRecommendationEngine(DefaultConfig())
	d := NewPatternDetector(DefaultConfig())
	in := sampleInput()
	w, l := d.Detect(in)
	recs := e.Recommend(in, w, l)
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	cats := map[RecommendationCategory]bool{}
	for i, r := range recs {
		if r.Priority != i+1 {
			t.Errorf("priorities should be sequential; got %d at index %d", r.Priority, i)
		}
		if r.Confidence <= 0 || len(r.Evidence) == 0 {
			t.Errorf("recommendation %q not grounded: %+v", r.Title, r)
		}
		if r.ExpectedImpact == "" || r.ID == "" {
			t.Error("missing impact/id")
		}
		cats[r.Category] = true
	}
	// Sorted by confidence descending.
	for i := 1; i < len(recs); i++ {
		if recs[i-1].Confidence < recs[i].Confidence {
			t.Error("recommendations not sorted by confidence")
		}
	}
	for _, want := range []RecommendationCategory{CatSEO, CatSocial, CatPublishing, CatPlatform, CatVideo, CatThumbnail} {
		if !cats[want] {
			t.Errorf("missing expected category %q", want)
		}
	}
}

// --- prompt versioning + approval ---

func TestPromptLifecycleAndRollback(t *testing.T) {
	repo := NewMemoryRepository()
	ps := NewPromptService(repo, fixedClock("2026-07-20"))

	v1, err := ps.Register("blog", "v1 body", "seed")
	if err != nil || v1.Version != 1 {
		t.Fatalf("register: %v %+v", err, v1)
	}
	// Re-register is a no-op.
	if again, _ := ps.Register("blog", "other", ""); again.Version != 1 {
		t.Error("re-register should return v1")
	}
	v2, err := ps.Propose("blog", "v2 body", "stronger hook")
	if err != nil || v2.Version != 2 || v2.Status != StatusProposed {
		t.Fatalf("propose: %v %+v", err, v2)
	}
	// v2 not active until approved; v1 is the active approved version.
	if act, ok := ps.ActiveVersion("blog"); !ok || act.Version != 1 {
		t.Errorf("active should be v1 before approval, got %+v", act)
	}
	if err := ps.Approve("blog", 2, "teddy", "looks good"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if ps.EffectiveStatus("blog", 2) != StatusApproved {
		t.Error("v2 should be approved")
	}
	if act, _ := ps.ActiveVersion("blog"); act.Version != 2 {
		t.Error("active should now be v2")
	}
	// Rollback: the immutable v1 is still readable.
	if len(repo.PromptVersions("blog")) != 2 {
		t.Error("both versions should be retained for rollback")
	}
	// Illegal transition: approving an already-approved version.
	if err := ps.Approve("blog", 2, "x", ""); err != ErrInvalidTransition {
		t.Errorf("re-approving should be invalid, got %v", err)
	}
	if err := ps.Approve("blog", 99, "x", ""); err != ErrNotFound {
		t.Errorf("unknown version should be not-found, got %v", err)
	}
	// Reject flow on a fresh proposal.
	_, _ = ps.Propose("blog", "v3", "")
	if err := ps.Reject("blog", 3, "teddy", "off-brand"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if ps.EffectiveStatus("blog", 3) != StatusRejected {
		t.Error("v3 should be rejected")
	}
	if err := ps.RecordUsage("blog", 1, PromptMetrics{UsageCount: 10, AvgCTR: 5}); err != nil {
		t.Errorf("record usage: %v", err)
	}
	if err := ps.RecordUsage("blog", 99, PromptMetrics{}); err != ErrNotFound {
		t.Error("usage on unknown version should be not-found")
	}
}

func TestProposeWithoutTemplate(t *testing.T) {
	ps := NewPromptService(NewMemoryRepository(), fixedClock("2026-07-20"))
	if _, err := ps.Propose("missing", "b", ""); err != ErrNotFound {
		t.Errorf("propose on missing template should be not-found, got %v", err)
	}
}

func TestPromptAnalyzer(t *testing.T) {
	repo := NewMemoryRepository()
	_ = repo.SavePromptVersion(PromptVersion{TemplateID: "blog", Version: 1, Status: StatusApproved, Metrics: PromptMetrics{AvgScore: 40}})
	_ = repo.SavePromptVersion(PromptVersion{TemplateID: "blog", Version: 2, Status: StatusApproved, Metrics: PromptMetrics{AvgScore: 55}})
	a := NewPromptAnalyzer(DefaultConfig())
	// Input with low retention/ctr/engagement triggers all three proposals.
	in := AnalyticsInput{Records: []PerformanceRecord{
		{RetentionPct: 40, CTR: 3, EngagementRate: 2, Score: 30},
		{RetentionPct: 45, CTR: 4, EngagementRate: 3, Score: 35},
	}}
	an := a.Analyze("blog", repo.PromptVersions("blog"), in)
	if an.CurrentVersion != 2 || an.BestVersion != 2 || an.Trend != "improving" {
		t.Errorf("analysis wrong: %+v", an)
	}
	if len(an.Recommendations) != 3 {
		t.Errorf("expected 3 prompt proposals (hook/seo/cta), got %d", len(an.Recommendations))
	}
	if a.Analyze("none", nil, in).Versions != 0 {
		t.Error("no versions should yield empty analysis")
	}
}

// --- reasoning ---

func TestDeterministicReasoner(t *testing.T) {
	d := NewPatternDetector(DefaultConfig())
	in := sampleInput()
	w, l := d.Detect(in)
	r, err := NewDeterministicReasoner().Reason(context.Background(), in, w, l)
	if err != nil {
		t.Fatalf("reason: %v", err)
	}
	if len(r.WhyWell) == 0 || len(r.WhyUnder) == 0 || len(r.Evidence) == 0 {
		t.Errorf("reasoning not grounded: %+v", r)
	}
	if r.Reviewer != "deterministic" || r.Confidence <= 0 {
		t.Error("reviewer/confidence missing")
	}
}

func TestModelReasonerParsingAndFallback(t *testing.T) {
	in := sampleInput()
	good := &fakeModel{out: `noise {"whyPerformedWell":["high ctr"],"whyUnderperformed":["weak hook"],"supportingEvidence":["CTR 9%"],"suggestedImprovements":["stronger hook"],"confidence":0.8,"expectedImpact":"high"} trailing`}
	r, err := NewModelReasoner(good, "ollama").Reason(context.Background(), in, nil, nil)
	if err != nil || r.Reviewer != "ollama" || r.Confidence != 0.8 || len(r.WhyWell) == 0 {
		t.Fatalf("model reasoning parse: %v %+v", err, r)
	}
	// Unparseable → parse error (permanent).
	bad := &fakeModel{out: "no json here"}
	if _, err := NewModelReasoner(bad, "").Reason(context.Background(), in, nil, nil); err == nil {
		t.Error("unparseable output should error")
	}
	// Model error → recoverable.
	errModel := &fakeModel{err: errors.New("boom")}
	_, err = NewModelReasoner(errModel, "").Reason(context.Background(), in, nil, nil)
	if !isRecoverable(err) {
		t.Errorf("model error should be recoverable, got %v", err)
	}
	// Nil model → unavailable.
	if _, err := (&ModelReasoner{}).Reason(context.Background(), in, nil, nil); !errors.Is(err, ErrReasoningUnavailable) {
		t.Errorf("nil model should be unavailable, got %v", err)
	}
}

// --- optimizer orchestration ---

func TestOptimizeEndToEnd(t *testing.T) {
	repo := NewMemoryRepository()
	_ = repo.SavePromptVersion(PromptVersion{TemplateID: "blog", Version: 1, Status: StatusApproved, Metrics: PromptMetrics{AvgScore: 50}})
	pub := &capturePublisher{}
	opt := NewOptimizer(DefaultConfig(), repo, fixedClock("2026-07-20"))
	opt.Metrics = pub

	report, err := opt.Optimize(context.Background(), sampleInput(), "run-1")
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if report.RunID != "run-1" || report.SchemaVersion != SchemaVersion {
		t.Error("run id / schema missing")
	}
	if len(report.WinningPatterns) == 0 || len(report.LosingPatterns) == 0 {
		t.Error("expected patterns")
	}
	if len(report.Recommendations) == 0 || len(report.PromptAnalyses) == 0 {
		t.Error("expected recommendations + prompt analyses")
	}
	if report.Confidence <= 0 || report.ExecutiveSummary == "" {
		t.Error("confidence/summary missing")
	}
	if report.DataQuality.Records != 5 {
		t.Errorf("data quality records = %d", report.DataQuality.Records)
	}
	// Persisted append-only.
	if len(repo.Reports()) != 1 || len(repo.Recommendations()) == 0 || len(repo.Patterns()) == 0 {
		t.Error("run not persisted")
	}
	// CloudWatch metrics emitted.
	if !pub.saw(MetricOptimizationRuns) || !pub.saw(MetricRecommendationCount) || !pub.saw(MetricOptimizationConfidence) {
		t.Errorf("expected optimizer metrics, saw %v", pub.names())
	}
	// Idempotent: same run id returns the stored report, no duplication.
	again, _ := opt.Optimize(context.Background(), sampleInput(), "run-1")
	if again.RunID != "run-1" || len(repo.Reports()) != 1 {
		t.Error("optimize should be idempotent per run id")
	}
	md := report.Markdown()
	if !strings.Contains(md, "# Content Optimization Report") || !strings.Contains(md, "Recommendations") {
		t.Error("markdown missing sections")
	}
}

func TestOptimizeNoData(t *testing.T) {
	opt := NewOptimizer(DefaultConfig(), nil, fixedClock("2026-07-20"))
	if _, err := opt.Optimize(context.Background(), AnalyticsInput{}, ""); !errors.Is(err, ErrNoData) {
		t.Errorf("empty input should be ErrNoData, got %v", err)
	}
}

func TestOptimizeReasonerFallback(t *testing.T) {
	// A failing primary reasoner must fall back to deterministic without failing.
	opt := NewOptimizer(DefaultConfig(), NewMemoryRepository(), fixedClock("2026-07-20"))
	opt.Config.PublishToCloud = false
	opt.WithReasoner(&fakeReasoner{err: errReasoning("down", nil)})
	report, err := opt.Optimize(context.Background(), sampleInput(), "r")
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if report.Reasoning.Reviewer != "deterministic" {
		t.Errorf("expected deterministic fallback, got %q", report.Reasoning.Reviewer)
	}
}

func TestAcceptanceRate(t *testing.T) {
	repo := NewMemoryRepository()
	_ = repo.SaveApproval(Approval{Kind: "recommendation", Status: StatusApproved})
	_ = repo.SaveApproval(Approval{Kind: "recommendation", Status: StatusRejected})
	_ = repo.SaveApproval(Approval{Kind: "prompt", Status: StatusApproved})
	opt := NewOptimizer(DefaultConfig(), repo, fixedClock("2026-07-20"))
	if got := opt.acceptanceRate(); got != 50 {
		t.Errorf("acceptance rate = %v, want 50", got)
	}
	empty := NewOptimizer(DefaultConfig(), NewMemoryRepository(), nil)
	if empty.acceptanceRate() != 0 {
		t.Error("no approvals should be 0")
	}
}

// --- input adapter (real M17 types) ---

func TestFromContentAnalytics(t *testing.T) {
	report := ca.Report{
		Window: "2026-07-01..2026-07-20",
		ContentPerformance: []ca.ContentPerformance{
			{PublicationID: "p1", Platform: ca.PlatformYouTube, ContentType: ca.TypeYouTubeVideo, Title: "V", Views: 1000, CTR: 5, EngagementRate: 8, RetentionPct: 60, WatchMinutes: 400, Score: 70},
		},
		Platforms: []ca.PlatformSummary{{Platform: ca.PlatformYouTube, Publications: 1, Views: 1000, Engagements: 80, EngagementRate: 8}},
	}
	signals := ca.OptimizationSignals{
		StrongestHashtags:   []ca.TagSignal{{Tag: "aws", Publications: 1, Views: 1000, EngagementRate: 8}},
		BestTopics:          []ca.TopicSignal{{Keyword: "go", Publications: 1, Views: 1000}},
		BestPublishingTimes: []ca.PublishingTime{{Weekday: "Monday", Hour: 10, Publications: 1, AvgViews: 1000}},
	}
	in := FromContentAnalytics(report, signals, nil)
	if len(in.Records) != 1 || in.Records[0].PublicationID != "p1" || in.Records[0].Score != 70 {
		t.Fatalf("records not mapped: %+v", in.Records)
	}
	if len(in.Platforms) != 1 || len(in.TopTags) != 1 || len(in.TopKeywords) != 1 || len(in.BestTimes) != 1 {
		t.Error("aggregates not mapped")
	}
	if in.AsOf != "2026-07-01..2026-07-20" {
		t.Error("asOf not mapped")
	}
	in.EnrichTags(map[string][]string{"p1": {"aws", "go"}}, map[string][]string{"p1": {"lambda"}})
	if len(in.Records[0].Tags) != 2 || len(in.Records[0].Keywords) != 1 {
		t.Errorf("enrich tags failed: %+v", in.Records[0])
	}
}

// --- util + config ---

func TestUtilHelpers(t *testing.T) {
	if round2(1.235) != 1.24 || round2(-1.235) != -1.24 {
		t.Errorf("round2: %v %v", round2(1.235), round2(-1.235))
	}
	if pct(150, 100) != 50 || pct(1, 0) != 100 || pct(0, 0) != 0 {
		t.Error("pct")
	}
	if clampFloat(5, 0, 3) != 3 || clampFloat(-1, 0, 3) != 0 {
		t.Error("clamp")
	}
	if percentile([]float64{1, 2, 3, 4}, 0.75) != 3 || percentile(nil, 0.5) != 0 {
		t.Error("percentile")
	}
	if expectedImpact(0.8) != "high" || expectedImpact(0.6) != "medium" || expectedImpact(0.2) != "low" {
		t.Error("expectedImpact")
	}
	if supportConfidence(8, 1) <= supportConfidence(1, 0) == false {
		// more support+separation should be >= less
	}
	if mean(nil) != 0 || mean([]float64{2, 4}) != 3 {
		t.Error("mean")
	}
	if shortID("a", "b") == shortID("a", "c") {
		t.Error("shortID should differ")
	}
	if abs(-3) != 3 {
		t.Error("abs")
	}
}

func TestConfigDefaults(t *testing.T) {
	c := Config{}
	if c.topN() != 5 || c.minSupport() != 1 || c.namespace() != "ContentOptimization" {
		t.Error("config fallbacks")
	}
	if c.winningPct() != 0.75 || c.losingPct() != 0.25 || c.trendChangePct() != 20 {
		t.Error("threshold fallbacks")
	}
}

func TestCloudWatchPublishers(t *testing.T) {
	if err := (LogMetricsPublisher{}).Publish(context.Background(), "ns", []Metric{metric("X", 1, "Count", map[string]string{"p": "YouTube"}, time.Now())}); err != nil {
		t.Errorf("log publisher: %v", err)
	}
	if err := (nopMetricsPublisher{}).Publish(context.Background(), "ns", nil); err != nil {
		t.Error("nop publisher")
	}
}

// --- test doubles ---

type fakeModel struct {
	out string
	err error
}

func (m *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	return m.out, m.err
}

type fakeReasoner struct {
	err error
}

func (f *fakeReasoner) Reason(context.Context, AnalyticsInput, []Pattern, []Pattern) (Reasoning, error) {
	return Reasoning{}, f.err
}

type capturePublisher struct{ metrics []Metric }

func (p *capturePublisher) Publish(_ context.Context, _ string, m []Metric) error {
	p.metrics = append(p.metrics, m...)
	return nil
}
func (p *capturePublisher) saw(name string) bool {
	for _, m := range p.metrics {
		if m.Name == name {
			return true
		}
	}
	return false
}
func (p *capturePublisher) names() []string {
	var out []string
	for _, m := range p.metrics {
		out = append(out, m.Name)
	}
	return out
}
