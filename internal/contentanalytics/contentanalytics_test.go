package contentanalytics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

func fixedClock(d Date) Clock {
	t := parseDate(d)
	return func() time.Time { return t }
}

// seedPubs registers the standard demo publications and returns the repo.
func seedPubs(t *testing.T) *MemoryRepository {
	t.Helper()
	repo := NewMemoryRepository()
	base := parseDate("2026-07-01")
	pubs := []Publication{
		{ID: "d1", ContentID: "c1", Platform: PlatformDevTo, ContentType: TypeBlog, Title: "Go Clean Arch", PlatformID: "d1", Tags: []string{"golang", "aws"}, Keywords: []string{"solid"}, PublishedAt: base.Add(9 * time.Hour), Status: "published"},
		{ID: "d2", ContentID: "c2", Platform: PlatformDevTo, ContentType: TypeBlog, Title: "Serverless", PlatformID: "d2", Tags: []string{"aws"}, Keywords: []string{"lambda"}, PublishedAt: base.Add(10 * time.Hour), Status: "published"},
		{ID: "h1", ContentID: "c3", Platform: PlatformHashnode, ContentType: TypeBlog, Title: "EventBridge", PlatformID: "h1", Tags: []string{"aws"}, Keywords: []string{"events"}, PublishedAt: base.Add(11 * time.Hour), Status: "published"},
		{ID: "y1", ContentID: "c4", Platform: PlatformYouTube, ContentType: TypeYouTubeVideo, Title: "Release Bot", PlatformID: "y1", Tags: []string{"golang"}, Keywords: []string{"ci"}, PublishedAt: base.Add(12 * time.Hour), Status: "published"},
	}
	for _, p := range pubs {
		if err := repo.SavePublication(p); err != nil {
			t.Fatalf("save pub: %v", err)
		}
	}
	return repo
}

func syntheticProviders(start Date) []AnalyticsProvider {
	return []AnalyticsProvider{
		NewSyntheticProvider(PlatformDevTo, start, 1000, 100, false),
		NewSyntheticProvider(PlatformHashnode, start, 800, 60, false),
		NewSyntheticProvider(PlatformYouTube, start, 5000, 300, true),
	}
}

// seedHistory collects `days` of synthetic snapshots starting at start.
func seedHistory(t *testing.T, start Date, days int) (*MemoryRepository, *Collector) {
	t.Helper()
	repo := seedPubs(t)
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	col := NewCollector(cfg, repo, syntheticProviders(start), fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}
	for i := 0; i < days; i++ {
		col.Collect(context.Background(), addDays(start, i))
	}
	return repo, col
}

// mockDoer is a scripted HTTPDoer.
type mockDoer struct {
	status int
	body   string
	err    error
	calls  int
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

// bodyDoer returns a body string.
type bodyDoer struct {
	status int
	body   string
}

func (d bodyDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: d.status,
		Body:       io.NopCloser(strings.NewReader(d.body)),
		Header:     make(http.Header),
	}, nil
}

// --- store / immutability ---

func TestRepositoryImmutableSnapshots(t *testing.T) {
	repo := seedPubs(t)
	s := MetricsSnapshot{PublicationID: "d1", ContentID: "c1", Platform: PlatformDevTo, Period: PeriodDaily, Date: "2026-07-01", Reach: Reach{Views: 100}}
	if err := repo.SaveSnapshot(s); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := repo.SaveSnapshot(s); err != ErrDuplicateSnapshot {
		t.Fatalf("duplicate save should be rejected, got %v", err)
	}
	if !repo.SnapshotExists("d1", PeriodDaily, "2026-07-01") {
		t.Error("snapshot should exist")
	}
	if got := repo.Snapshots("d1", PeriodDaily); len(got) != 1 {
		t.Errorf("want 1 snapshot, got %d", len(got))
	}
	if _, ok := repo.LatestSnapshot("d1", PeriodDaily); !ok {
		t.Error("latest snapshot should be found")
	}
	if _, ok := repo.LatestSnapshot("nope", PeriodDaily); ok {
		t.Error("missing publication should have no latest snapshot")
	}
}

func TestRepositoryQueries(t *testing.T) {
	repo := seedPubs(t)
	if len(repo.Publications()) != 4 {
		t.Errorf("want 4 pubs, got %d", len(repo.Publications()))
	}
	if len(repo.PublicationsByPlatform(PlatformDevTo)) != 2 {
		t.Error("want 2 devto pubs")
	}
	plats := repo.Platforms()
	if len(plats) != 3 {
		t.Errorf("want 3 platforms, got %v", plats)
	}
	if err := repo.SavePublication(Publication{}); err == nil {
		t.Error("empty publication id should error")
	}
	if err := repo.SaveSnapshot(MetricsSnapshot{}); err == nil {
		t.Error("empty snapshot publication id should error")
	}
}

// --- collector ---

func TestCollectorHappyPathAndIdempotency(t *testing.T) {
	start := "2026-07-01"
	repo := seedPubs(t)
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	col := NewCollector(cfg, repo, syntheticProviders(start), fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}

	res := col.Collect(context.Background(), start)
	if res.Collected != 4 {
		t.Fatalf("want 4 collected, got %d (items=%+v)", res.Collected, res.Items)
	}
	if res.DailyViews == 0 {
		t.Error("daily views should be summed")
	}
	if res.SuccessRate() != 100 {
		t.Errorf("success rate should be 100, got %v", res.SuccessRate())
	}
	// Second run: everything deduped.
	res2 := col.Collect(context.Background(), start)
	if res2.Skipped != 4 || res2.Collected != 0 {
		t.Errorf("second run should skip all, got collected=%d skipped=%d", res2.Collected, res2.Skipped)
	}
}

func TestCollectorRetryThenSuccess(t *testing.T) {
	start := "2026-07-01"
	repo := NewMemoryRepository()
	_ = repo.SavePublication(Publication{ID: "d1", ContentID: "c1", Platform: PlatformDevTo, ContentType: TypeBlog, PlatformID: "d1"})
	prov := NewSyntheticProvider(PlatformDevTo, start, 1000, 100, false)
	prov.FailFirst = 2
	prov.Recoverable = true
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	col := NewCollector(cfg, repo, []AnalyticsProvider{prov}, fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}

	res := col.Collect(context.Background(), start)
	if res.Collected != 1 {
		t.Fatalf("want 1 collected after retries, got %d", res.Collected)
	}
	if res.Items[0].Retries != 2 {
		t.Errorf("want 2 retries, got %d", res.Items[0].Retries)
	}
}

func TestCollectorPermanentAuthFailure(t *testing.T) {
	start := "2026-07-01"
	repo := NewMemoryRepository()
	_ = repo.SavePublication(Publication{ID: "y1", ContentID: "c", Platform: PlatformYouTube, ContentType: TypeYouTubeVideo, PlatformID: "y1"})
	prov := NewSyntheticProvider(PlatformYouTube, start, 1, 1, true)
	prov.Auth = true
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	col := NewCollector(cfg, repo, []AnalyticsProvider{prov}, fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}

	res := col.Collect(context.Background(), start)
	if res.Failed != 1 {
		t.Fatalf("want 1 failed, got %d", res.Failed)
	}
	if res.Items[0].Code != "auth" {
		t.Errorf("want auth code, got %q", res.Items[0].Code)
	}
	if res.Items[0].Retries != 0 {
		t.Errorf("permanent errors must not retry, got %d", res.Items[0].Retries)
	}
}

func TestCollectorPartialSnapshot(t *testing.T) {
	// A non-video synthetic provider yields no watch metrics but still succeeds;
	// mark that the collector records a full (non-partial) daily snapshot because
	// composeMetrics got reach+engagement. Partial is exercised via YouTube adapter.
	start := "2026-07-01"
	repo, _ := seedHistory(t, start, 1)
	snaps := repo.SnapshotsOn(PeriodDaily, start)
	if len(snaps) != 4 {
		t.Fatalf("want 4 snapshots on date, got %d", len(snaps))
	}
}

func TestCollectorDisabledPlatform(t *testing.T) {
	start := "2026-07-01"
	repo := seedPubs(t)
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	cfg.Enabled = []Platform{PlatformYouTube}
	col := NewCollector(cfg, repo, syntheticProviders(start), fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}
	res := col.Collect(context.Background(), start)
	for _, it := range res.Items {
		if it.Platform != PlatformYouTube {
			t.Errorf("disabled platform %s should not be collected", it.Platform)
		}
	}
}

func TestCollectorCloudWatchEmission(t *testing.T) {
	start := "2026-07-01"
	repo := seedPubs(t)
	cfg := DefaultConfig()
	pub := &capturePublisher{}
	col := NewCollector(cfg, repo, syntheticProviders(start), fixedClock(start))
	col.Metrics = pub
	col.Sleep = func(context.Context, time.Duration) {}
	col.Collect(context.Background(), start)
	if !pub.sawMetric(MetricFailedCollections) || !pub.sawMetric(MetricPublishingSuccessRate) {
		t.Errorf("expected collection metrics to be published, saw %v", pub.names())
	}
}

func TestCollectToday(t *testing.T) {
	start := "2026-07-05"
	repo := seedPubs(t)
	cfg := DefaultConfig()
	cfg.PublishToCloud = false
	col := NewCollector(cfg, repo, syntheticProviders(start), fixedClock(start))
	col.Sleep = func(context.Context, time.Duration) {}
	res := col.CollectToday(context.Background())
	if res.Date != start || res.Collected != 4 {
		t.Errorf("CollectToday: date=%s collected=%d", res.Date, res.Collected)
	}
}

// --- snapshots / rollups ---

func TestRollUpWeeklyMonthly(t *testing.T) {
	start := "2026-07-01" // Wednesday
	repo, _ := seedHistory(t, start, 14)
	n := NewSnapshotter(repo, fixedClock(addDays(start, 13))).RollUpRange(start, addDays(start, 13))
	if n == 0 {
		t.Fatal("expected weekly/monthly rollups to be written")
	}
	weekly := repo.Snapshots("d1", PeriodWeekly)
	if len(weekly) < 2 {
		t.Errorf("want >=2 weekly rollups for d1, got %d", len(weekly))
	}
	monthly := repo.Snapshots("d1", PeriodMonthly)
	if len(monthly) < 1 {
		t.Errorf("want >=1 monthly rollup for d1, got %d", len(monthly))
	}
	// Idempotent: a second rollup writes nothing.
	if again := NewSnapshotter(repo, fixedClock(start)).RollUpRange(start, addDays(start, 13)); again != 0 {
		t.Errorf("rollups must be idempotent, wrote %d again", again)
	}
	// Weekly rollup holds the latest daily value in its bucket.
	wk := weekKey(start)
	var wkSnap MetricsSnapshot
	for _, s := range weekly {
		if s.Date == wk {
			wkSnap = s
		}
	}
	if wkSnap.Reach.Views == 0 {
		t.Error("weekly rollup should carry cumulative views")
	}
}

// --- analytics / trends ---

func TestGrowthTrends(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seedHistory(t, start, 30)
	a := NewAnalytics(repo, DefaultConfig())
	g := a.Growth("d1", "views")
	if g.Current == 0 {
		t.Fatal("current views should be non-zero")
	}
	if g.DayOverDay.Absolute <= 0 || g.WeekOverWeek.Absolute <= 0 || g.MonthOverMonth.Absolute <= 0 {
		t.Errorf("growth should be positive: %+v", g)
	}
	if g.Direction != "up" {
		t.Errorf("direction should be up, got %s", g.Direction)
	}
	if s := a.Series("d1", PeriodDaily, "views"); len(s) != 30 {
		t.Errorf("want 30-point series, got %d", len(s))
	}
	// Unknown metric yields zeros, no panic.
	if v := snapshotMetric(MetricsSnapshot{}, "bogus"); v != 0 {
		t.Error("unknown metric should be 0")
	}
}

func TestPerformanceRankingAndPlatformSummaries(t *testing.T) {
	date := "2026-07-10"
	repo, _ := seedHistory(t, "2026-07-01", 10)
	a := NewAnalytics(repo, DefaultConfig())
	perf := a.Performance(date)
	if len(perf) != 4 {
		t.Fatalf("want 4 ranked, got %d", len(perf))
	}
	if perf[0].Rank != 1 {
		t.Error("first item should be rank 1")
	}
	for i := 1; i < len(perf); i++ {
		if perf[i-1].Score < perf[i].Score {
			t.Error("performance should be sorted descending by score")
		}
	}
	plats := a.PlatformSummaries(date)
	if len(plats) != 3 {
		t.Fatalf("want 3 platform summaries, got %d", len(plats))
	}
	for _, p := range plats {
		if p.Views == 0 || p.EngagementRate == 0 {
			t.Errorf("platform %s should have views and engagement", p.Platform)
		}
	}
}

func TestTopTagsKeywordsTimes(t *testing.T) {
	date := "2026-07-10"
	repo, _ := seedHistory(t, "2026-07-01", 10)
	a := NewAnalytics(repo, DefaultConfig())
	tags := a.TopTags(date, 5)
	if len(tags) == 0 || tags[0].Views == 0 {
		t.Error("expected grounded top tags")
	}
	kws := a.TopKeywords(date, 5)
	if len(kws) == 0 {
		t.Error("expected grounded top keywords")
	}
	times := a.TopPublishingTimes(date, 5)
	if len(times) == 0 || times[0].AvgViews == 0 {
		t.Error("expected grounded publishing times")
	}
}

// --- report ---

func TestReportGeneration(t *testing.T) {
	date := "2026-07-20"
	repo, _ := seedHistory(t, "2026-07-01", 20)
	r := NewReportEngine(repo, DefaultConfig(), fixedClock(date)).Generate(date)
	if r.SchemaVersion != SchemaVersion {
		t.Error("schema version missing")
	}
	if r.ExecutiveSummary == "" {
		t.Error("executive summary missing")
	}
	if len(r.Platforms) != 3 || len(r.ContentPerformance) != 4 {
		t.Errorf("platforms=%d content=%d", len(r.Platforms), len(r.ContentPerformance))
	}
	if len(r.TopVideos) == 0 || len(r.TopArticles) == 0 {
		t.Error("expected top videos and articles")
	}
	if r.EngagementBreakdown.Total == 0 {
		t.Error("engagement breakdown should be populated")
	}
	if len(r.GrowthTrends) == 0 || len(r.Recommendations) == 0 {
		t.Error("expected growth trends and recommendations")
	}
	if r.DataQuality.Publications != 4 || r.DataQuality.CompletenessScore != 100 {
		t.Errorf("data quality: %+v", r.DataQuality)
	}
	md := r.Markdown()
	if !strings.Contains(md, "# Content Analytics Report") || !strings.Contains(md, "Recommendations") {
		t.Error("markdown missing expected sections")
	}
}

func TestReportEmptyRepo(t *testing.T) {
	r := NewReportEngine(NewMemoryRepository(), DefaultConfig(), fixedClock("2026-07-20")).Generate("2026-07-20")
	if r.DataQuality.CompletenessScore != 100 {
		t.Errorf("empty repo completeness should default to 100, got %v", r.DataQuality.CompletenessScore)
	}
	if len(r.Recommendations) == 0 {
		t.Error("empty repo should still yield a recommendation")
	}
}

// --- dashboard ---

func TestDashboardDatasetAndPublish(t *testing.T) {
	date := "2026-07-10"
	repo, _ := seedHistory(t, "2026-07-01", 10)
	pub := &capturePublisher{}
	eng := NewDashboardEngine(repo, DefaultConfig(), pub, fixedClock(date))
	ds := eng.Dataset(date)
	if len(ds.Series) != 4 || ds.Totals.Views == 0 {
		t.Errorf("dataset incomplete: series=%d views=%d", len(ds.Series), ds.Totals.Views)
	}
	if len(ds.Series[0].Points) != 10 {
		t.Errorf("want 10 daily points, got %d", len(ds.Series[0].Points))
	}
	if err := eng.Publish(context.Background(), date); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !pub.sawMetric(MetricTotalViews) || !pub.sawMetric(MetricEngagementRate) {
		t.Errorf("expected dashboard metrics, saw %v", pub.names())
	}
}

// --- optimization signals ---

func TestOptimizationSignals(t *testing.T) {
	date := "2026-07-15"
	repo, _ := seedHistory(t, "2026-07-01", 15)
	sig := NewOptimizationEngine(repo, DefaultConfig(), fixedClock(date)).Signals(date)
	if sig.SchemaVersion != SchemaVersion {
		t.Error("schema version missing")
	}
	if len(sig.BestPublishingTimes) == 0 || len(sig.BestTopics) == 0 || len(sig.StrongestHashtags) == 0 {
		t.Error("expected publishing-time, topic and hashtag signals")
	}
	if len(sig.HighestCTR) == 0 {
		t.Error("expected CTR ranking")
	}
	if len(sig.BestRetention) == 0 {
		t.Error("expected retention ranking (YouTube has watch metrics)")
	}
	if len(sig.SuccessfulPatterns) == 0 || len(sig.PlatformRecommendations) == 0 {
		t.Error("expected patterns and platform recommendations")
	}
}

// --- provider adapters (mock HTTP) ---

func TestDevToProviderParsing(t *testing.T) {
	doer := bodyDoer{status: 200, body: `{"id":42,"title":"Go","url":"https://dev.to/x","page_views_count":1234,"public_reactions_count":56,"comments_count":7,"published_at":"2026-07-01T09:00:00Z","edited_at":"2026-07-02T09:00:00Z"}`}
	p := NewDevToProvider(doer, mapCredentials(map[string]string{envDevToKey: "k"}))
	if !p.Supports(TypeBlog) || p.Name() != PlatformDevTo {
		t.Fatal("supports/name")
	}
	pub := Publication{ID: "d1", ContentID: "c1", PlatformID: "42"}
	got, err := p.FetchPublication(context.Background(), pub)
	if err != nil || got.URL != "https://dev.to/x" || got.PublishedAt.IsZero() {
		t.Fatalf("fetch publication: %v %+v", err, got)
	}
	s, err := p.FetchMetrics(context.Background(), pub, "2026-07-05")
	if err != nil || s.Reach.Views != 1234 || s.Engagement.Reactions != 56 || s.Engagement.Comments != 7 {
		t.Fatalf("fetch metrics: %v %+v", err, s)
	}
}

func TestDevToProviderAuthError(t *testing.T) {
	p := NewDevToProvider(bodyDoer{status: 200, body: "{}"}, mapCredentials(nil))
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "d1"}, "2026-07-05"); !isAuthError(err) {
		t.Errorf("missing key should be an auth error, got %v", err)
	}
}

func TestHashnodeProviderParsing(t *testing.T) {
	doer := bodyDoer{status: 200, body: `{"data":{"post":{"id":"h1","title":"EB","url":"https://hashnode/x","views":900,"reactionCount":40,"responseCount":5,"publishedAt":"2026-07-01T00:00:00Z"}}}`}
	p := NewHashnodeProvider(doer, mapCredentials(map[string]string{envHashnodeToken: "t"}))
	pub := Publication{ID: "h1", ContentID: "c3", PlatformID: "h1"}
	s, err := p.FetchMetrics(context.Background(), pub, "2026-07-05")
	if err != nil || s.Reach.Views != 900 || s.Engagement.Reactions != 40 || s.Engagement.Comments != 5 {
		t.Fatalf("hashnode metrics: %v %+v", err, s)
	}
	got, err := p.FetchPublication(context.Background(), pub)
	if err != nil || got.URL == "" {
		t.Fatalf("hashnode publication: %v %+v", err, got)
	}
}

func TestHashnodeProviderGraphQLError(t *testing.T) {
	doer := bodyDoer{status: 200, body: `{"errors":[{"message":"not found"}]}`}
	p := NewHashnodeProvider(doer, mapCredentials(map[string]string{envHashnodeToken: "t"}))
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "h1"}, "2026-07-05"); err == nil {
		t.Error("graphql error should surface")
	}
}

func TestMediumProviderParsing(t *testing.T) {
	doer := bodyDoer{status: 200, body: `{"posts":[{"postId":"m1","title":"AI","url":"https://medium/x","views":500,"reads":300,"claps":80,"comments":9}]}`}
	p := NewMediumProvider(doer, mapCredentials(map[string]string{envMediumStatsURL: "https://stats"}))
	pub := Publication{ID: "m1", ContentID: "c4", PlatformID: "m1"}
	s, err := p.FetchMetrics(context.Background(), pub, "2026-07-05")
	if err != nil || s.Reach.Views != 500 || s.Reach.UniqueViewers != 300 || s.Engagement.Reactions != 80 {
		t.Fatalf("medium metrics: %v %+v", err, s)
	}
	if s.Watch.CompletionRate == 0 {
		t.Error("medium read-ratio should map to completion rate")
	}
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "zzz"}, "2026-07-05"); err == nil {
		t.Error("missing post should error")
	}
}

func TestMediumProviderNoStatsURL(t *testing.T) {
	p := NewMediumProvider(bodyDoer{status: 200, body: "{}"}, mapCredentials(nil))
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "m1"}, "2026-07-05"); err == nil {
		t.Error("no stats url should error")
	}
}

func TestYouTubeProviderDataAndAnalytics(t *testing.T) {
	// Data API only (no OAuth) → partial snapshot.
	data := bodyDoer{status: 200, body: `{"items":[{"id":"y1","snippet":{"title":"Bot","publishedAt":"2026-07-01T00:00:00Z"},"statistics":{"viewCount":"10000","likeCount":"500","commentCount":"40"}}]}`}
	p := NewYouTubeProvider(data, mapCredentials(map[string]string{envYouTubeKey: "k"}))
	pub := Publication{ID: "y1", ContentID: "c5", PlatformID: "y1"}
	s, err := p.FetchMetrics(context.Background(), pub, "2026-07-05")
	if err != nil || s.Reach.Views != 10000 || s.Engagement.Likes != 500 {
		t.Fatalf("yt data metrics: %v %+v", err, s)
	}
	if !s.Partial {
		t.Error("snapshot should be partial without OAuth watch metrics")
	}
	got, err := p.FetchPublication(context.Background(), pub)
	if err != nil || got.Title != "Bot" || got.URL == "" {
		t.Fatalf("yt publication: %v %+v", err, got)
	}
}

func TestYouTubeProviderMissingVideo(t *testing.T) {
	p := NewYouTubeProvider(bodyDoer{status: 200, body: `{"items":[]}`}, mapCredentials(map[string]string{envYouTubeKey: "k"}))
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "y1"}, "2026-07-05"); err == nil {
		t.Error("missing video should error")
	}
}

// --- HTTP classification ---

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		code   string
		recov  bool
	}{
		{401, "auth", false},
		{403, "auth", false},
		{402, "quota", false},
		{429, "rate_limit", true},
		{404, "not_found", false},
		{500, "server", true},
		{503, "server", true},
		{418, "http", false},
	}
	for _, c := range cases {
		err := classifyStatus(c.status, "body")
		if err.Code != c.code {
			t.Errorf("status %d: want code %q, got %q", c.status, c.code, err.Code)
		}
		if err.Recoverable != c.recov {
			t.Errorf("status %d: want recoverable %v", c.status, c.recov)
		}
	}
}

func TestGetJSONTransportError(t *testing.T) {
	doer := &mockDoer{err: http.ErrHandlerTimeout}
	err := getJSON(context.Background(), doer, "http://x", nil, nil)
	if !isRecoverable(err) {
		t.Errorf("transport error should be recoverable, got %v", err)
	}
}

func TestGetJSONHTTPError(t *testing.T) {
	err := getJSON(context.Background(), bodyDoer{status: 429, body: "slow down"}, "http://x", nil, nil)
	if errCode(err) != "rate_limit" {
		t.Errorf("want rate_limit, got %q", errCode(err))
	}
}

// --- errors + util ---

func TestErrorClassification(t *testing.T) {
	if !isRecoverable(errServer("x")) || isRecoverable(errAuth("x")) {
		t.Error("recoverable classification wrong")
	}
	if !isAuthError(errAuth("x")) || isAuthError(errServer("x")) {
		t.Error("auth classification wrong")
	}
	if errCode(errRateLimit("x")) != "rate_limit" {
		t.Error("code extraction wrong")
	}
	ce := errNetwork("boom", http.ErrHandlerTimeout)
	if ce.Unwrap() == nil || !strings.Contains(ce.Error(), "boom") {
		t.Error("wrap/error string wrong")
	}
}

func TestDateAndMathHelpers(t *testing.T) {
	if addDays("2026-07-01", 6) != "2026-07-07" {
		t.Error("addDays")
	}
	if daysBetween("2026-07-01", "2026-07-08") != 7 {
		t.Error("daysBetween")
	}
	if weekKey("2026-07-01") != "2026-06-29" { // Wed → Monday
		t.Errorf("weekKey got %s", weekKey("2026-07-01"))
	}
	if monthKey("2026-07-20") != "2026-07-01" {
		t.Error("monthKey")
	}
	if pct(150, 100) != 50 || pct(1, 0) != 100 || pct(0, 0) != 0 {
		t.Error("pct")
	}
	if round2(1.235) != 1.24 || round2(-1.235) != -1.24 {
		t.Errorf("round2 got %v %v", round2(1.235), round2(-1.235))
	}
	if comma(1234567) != "1,234,567" || comma(-1000) != "-1,000" {
		t.Errorf("comma got %s", comma(1234567))
	}
	if direction(5) != "up" || direction(-5) != "down" || direction(0) != "flat" {
		t.Error("direction")
	}
	if safeDiv(1, 0) != 0 || clampFloat(5, 0, 3) != 3 {
		t.Error("safeDiv/clamp")
	}
}

func TestConfigHelpers(t *testing.T) {
	c := Config{}
	if c.topN() != 5 || c.pageSize() != 50 || c.namespace() != "ContentAnalytics" {
		t.Error("config defaults")
	}
	if !c.isEnabled(PlatformDevTo) {
		t.Error("empty enabled = all")
	}
	c.Enabled = []Platform{PlatformMedium}
	if c.isEnabled(PlatformDevTo) || !c.isEnabled(PlatformMedium) {
		t.Error("enabled filter")
	}
}

func TestCloudWatchLogPublisher(t *testing.T) {
	p := LogMetricsPublisher{}
	err := p.Publish(context.Background(), "ns", []Metric{
		metric("X", 1, "Count", map[string]string{"platform": "Dev.to"}, time.Now()),
	})
	if err != nil {
		t.Errorf("log publisher should not error: %v", err)
	}
	if err := (nopMetricsPublisher{}).Publish(context.Background(), "ns", nil); err != nil {
		t.Error("nop publisher should not error")
	}
}

func TestComposeMetricsUnsupported(t *testing.T) {
	// A provider implementing no capabilities returns ErrUnsupported.
	prov := &barebonesProvider{}
	if _, err := composeMetrics(context.Background(), prov, Publication{ID: "x"}, "2026-07-01"); err == nil {
		t.Error("provider with no capabilities should be unsupported")
	}
}

// scriptDoer returns responses in sequence (for multi-call providers).
type scriptDoer struct {
	bodies []string
	i      int
}

func (d *scriptDoer) Do(req *http.Request) (*http.Response, error) {
	body := "{}"
	if d.i < len(d.bodies) {
		body = d.bodies[d.i]
	}
	d.i++
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func TestYouTubeProviderWithWatchMetrics(t *testing.T) {
	doer := &scriptDoer{bodies: []string{
		`{"items":[{"id":"y1","snippet":{"title":"Bot"},"statistics":{"viewCount":"10000","likeCount":"500","commentCount":"40"}}]}`,
		`{"rows":[[820,215,58]]}`,
	}}
	p := NewYouTubeProvider(doer, mapCredentials(map[string]string{envYouTubeKey: "k", envYouTubeOAuth: "oauth"}))
	s, err := p.FetchMetrics(context.Background(), Publication{ID: "y1", PlatformID: "y1"}, "2026-07-05")
	if err != nil {
		t.Fatalf("fetch metrics: %v", err)
	}
	if s.Partial {
		t.Error("snapshot should not be partial when watch metrics are present")
	}
	if s.Watch.WatchTimeMinutes != 820 || s.Watch.RetentionPct != 58 {
		t.Errorf("watch metrics not parsed: %+v", s.Watch)
	}
}

func TestYouTubeProviderRecoverableAnalyticsFailure(t *testing.T) {
	// Data OK, analytics returns 500 → whole item should error recoverably.
	doer := &scriptDoer{bodies: []string{
		`{"items":[{"id":"y1","statistics":{"viewCount":"10","likeCount":"1","commentCount":"0"}}]}`,
	}}
	// Second call falls through to "{}" with status 200 → no rows → unsupported, so
	// force a 500 by using a status-based doer for the analytics call instead.
	p := NewYouTubeProvider(seqStatusDoer{statuses: []int{200, 500}, bodies: []string{doer.bodies[0], "err"}}, mapCredentials(map[string]string{envYouTubeKey: "k", envYouTubeOAuth: "o"}))
	if _, err := p.FetchMetrics(context.Background(), Publication{ID: "y1"}, "2026-07-05"); !isRecoverable(err) {
		t.Errorf("transient analytics failure should be recoverable, got %v", err)
	}
}

func TestProviderNamesAndSupports(t *testing.T) {
	if p := NewHashnodeProvider(bodyDoer{}, mapCredentials(nil)); p.Name() != PlatformHashnode || !p.Supports(TypeArticle) {
		t.Error("hashnode name/supports")
	}
	if p := NewMediumProvider(bodyDoer{}, mapCredentials(nil)); p.Name() != PlatformMedium || !p.Supports(TypeBlog) {
		t.Error("medium name/supports")
	}
	if p := NewYouTubeProvider(bodyDoer{}, mapCredentials(nil)); p.Name() != PlatformYouTube || p.Supports(TypeBlog) {
		t.Error("youtube name/supports")
	}
	sp := NewSyntheticProvider(PlatformDevTo, "2026-07-01", 1, 1, false)
	if !sp.Supports(TypeBlog) {
		t.Error("synthetic supports-all")
	}
	sp.Supported = []ContentType{TypeArticle}
	if sp.Supports(TypeBlog) || !sp.Supports(TypeArticle) {
		t.Error("synthetic supports-filter")
	}
	if _, err := sp.FetchPublication(context.Background(), Publication{ID: "x"}); err != nil {
		t.Errorf("synthetic fetch publication: %v", err)
	}
}

func TestMediumFetchPublicationAndEnvCreds(t *testing.T) {
	doer := bodyDoer{status: 200, body: `{"posts":[{"postId":"m1","title":"AI","url":"https://m/x","views":10,"reads":5,"claps":1,"comments":0}]}`}
	p := NewMediumProvider(doer, mapCredentials(map[string]string{envMediumStatsURL: "https://s", envMediumToken: "tok"}))
	got, err := p.FetchPublication(context.Background(), Publication{ID: "m1", PlatformID: "m1"})
	if err != nil || got.URL == "" || got.Status != "published" {
		t.Fatalf("medium fetch publication: %v %+v", err, got)
	}
	if c := EnvCredentials(); c.has("DEFINITELY_UNSET_VAR_XYZ") {
		t.Error("unset env var should not be present")
	}
}

// seqStatusDoer routes by host: the analytics host gets the second (status,body)
// pair, everything else the first — so a two-endpoint provider is scripted
// without relying on call order.
type seqStatusDoer struct {
	statuses []int
	bodies   []string
}

func (d seqStatusDoer) Do(req *http.Request) (*http.Response, error) {
	status, body := d.statuses[0], d.bodies[0]
	if strings.Contains(req.URL.Host, "youtubeanalytics") {
		status, body = d.statuses[1], d.bodies[1]
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

// --- test-only providers ---

type capturePublisher struct{ metrics []Metric }

func (p *capturePublisher) Publish(_ context.Context, _ string, m []Metric) error {
	p.metrics = append(p.metrics, m...)
	return nil
}
func (p *capturePublisher) sawMetric(name string) bool {
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

// barebonesProvider implements only AnalyticsProvider (no fetcher capabilities).
type barebonesProvider struct{}

func (b *barebonesProvider) Name() Platform            { return PlatformDevTo }
func (b *barebonesProvider) Supports(ContentType) bool { return true }
func (b *barebonesProvider) FetchPublication(_ context.Context, p Publication) (Publication, error) {
	return p, nil
}
func (b *barebonesProvider) FetchMetrics(_ context.Context, _ Publication, _ Date) (MetricsSnapshot, error) {
	return MetricsSnapshot{}, ErrUnsupported
}
