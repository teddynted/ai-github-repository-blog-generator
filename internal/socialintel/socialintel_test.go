package socialintel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

// seed collects `days` daily snapshots for the given providers into a fresh repo.
func seed(t *testing.T, start Date, days int, providers ...Provider) (*MemoryRepository, *Collector) {
	t.Helper()
	repo := NewMemoryRepository()
	cfg := DefaultConfig()
	col := NewCollector(cfg, repo, providers, func() time.Time { return parseDate(start) })
	col.Sleep = func(context.Context, time.Duration) {}
	for i := 0; i < days; i++ {
		col.Collect(context.Background(), addDays(start, i))
	}
	return repo, col
}

func syntheticSet(start Date) []Provider {
	return []Provider{
		NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 3),
		NewSyntheticProvider(PlatformTikTok, start, 5000, 80, 3),
		NewSyntheticProvider(PlatformX, start, 2000, 5, 2),
	}
}

// --- collection ---

func TestCollectionPersistsImmutableSnapshots(t *testing.T) {
	start := "2026-07-01"
	repo, col := seed(t, start, 10, syntheticSet(start)...)

	yt := repo.AccountSeries(PlatformYouTube)
	if len(yt) != 10 {
		t.Fatalf("expected 10 daily snapshots, got %d", len(yt))
	}
	// Growing followers.
	if yt[0].Followers >= yt[9].Followers {
		t.Errorf("followers should grow: %d → %d", yt[0].Followers, yt[9].Followers)
	}
	// Re-collecting a day is skipped (immutable, dedupe).
	r := col.Collect(context.Background(), start)
	if !r.Platforms[PlatformYouTube].Skipped {
		t.Errorf("re-collecting an existing day should be skipped")
	}
	// A direct duplicate save is rejected.
	if err := repo.SaveAccountSnapshot(yt[0]); !errors.Is(err, ErrDuplicateSnapshot) {
		t.Errorf("duplicate save should be rejected, got %v", err)
	}
}

func TestCollectionRetriesRecoverable(t *testing.T) {
	start := "2026-07-01"
	yt := NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2)
	yt.FailFirst = 2
	yt.Recoverable = true
	repo := NewMemoryRepository()
	col := NewCollector(DefaultConfig(), repo, []Provider{yt}, func() time.Time { return parseDate(start) })
	col.Sleep = func(context.Context, time.Duration) {}

	r := col.Collect(context.Background(), start)
	pr := r.Platforms[PlatformYouTube]
	if !pr.Collected || pr.Retries != 2 {
		t.Fatalf("expected success after 2 retries, got %+v", pr)
	}
}

func TestCollectionPermanentErrorNotRetried(t *testing.T) {
	start := "2026-07-01"
	yt := NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2)
	yt.FailFirst = 5
	yt.Recoverable = false
	repo := NewMemoryRepository()
	col := NewCollector(DefaultConfig(), repo, []Provider{yt}, func() time.Time { return parseDate(start) })
	col.Sleep = func(context.Context, time.Duration) {}

	r := col.Collect(context.Background(), start)
	pr := r.Platforms[PlatformYouTube]
	if pr.Collected || pr.Retries != 0 || pr.Error == "" {
		t.Fatalf("permanent error should fail without retry, got %+v", pr)
	}
}

func TestBackfill(t *testing.T) {
	start := "2026-07-01"
	repo := NewMemoryRepository()
	col := NewCollector(DefaultConfig(), repo, []Provider{NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2)}, func() time.Time { return parseDate("2026-07-05") })
	col.Sleep = func(context.Context, time.Duration) {}
	results := col.Backfill(context.Background(), start, "2026-07-05")
	if len(results) != 5 {
		t.Fatalf("backfill should cover 5 days, got %d", len(results))
	}
	if len(repo.AccountSeries(PlatformYouTube)) != 5 {
		t.Errorf("expected 5 stored snapshots")
	}
}

// --- analytics ---

func TestGrowthAndTrends(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 30, NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2))
	a := Analytics{Repo: repo, Config: DefaultConfig()}

	g := a.Growth(PlatformYouTube, "followers")
	if g.Current != 10000+50*29 {
		t.Errorf("current followers = %v", g.Current)
	}
	if g.DayOverDay.Absolute != 50 {
		t.Errorf("DoD absolute = %v, want 50", g.DayOverDay.Absolute)
	}
	if g.WeekOverWeek.Absolute != 350 {
		t.Errorf("WoW absolute = %v, want 350", g.WeekOverWeek.Absolute)
	}
	if g.Velocity != 50 {
		t.Errorf("velocity = %v, want 50", g.Velocity)
	}
	tr := a.Trend(PlatformYouTube, "followers")
	if tr.Direction != "up" {
		t.Errorf("trend direction = %q, want up", tr.Direction)
	}
}

func TestEffectivenessRanking(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 5, NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 4))
	a := Analytics{Repo: repo, Config: DefaultConfig()}
	eff := a.Effectiveness(PlatformYouTube, addDays(start, 4))
	if len(eff) != 4 {
		t.Fatalf("expected 4 ranked items, got %d", len(eff))
	}
	// Ranks are assigned in descending score order.
	for i := 1; i < len(eff); i++ {
		if eff[i-1].Score < eff[i].Score {
			t.Errorf("not sorted by score: %v < %v", eff[i-1].Score, eff[i].Score)
		}
		if eff[i].Rank != i+1 {
			t.Errorf("rank %d = %d", i, eff[i].Rank)
		}
	}
	// Scores are grounded and bounded.
	for _, e := range eff {
		if e.Score < 0 || e.Score > 100 || e.EngagementRate < 0 {
			t.Errorf("bad score: %+v", e)
		}
	}
}

func TestSubscriberConversion(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 3, NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 3))
	a := Analytics{Repo: repo, Config: DefaultConfig()}
	conv := a.SubscriberConversion(PlatformYouTube, addDays(start, 2))
	if len(conv) != 3 {
		t.Fatalf("expected 3 videos, got %d", len(conv))
	}
	// Ranked by net subscribers descending; net = gained - lost.
	if conv[0].NetSubscribers < conv[len(conv)-1].NetSubscribers {
		t.Errorf("not ranked by net subscribers")
	}
	for _, c := range conv {
		if c.NetSubscribers != c.SubscribersGained-c.SubscribersLost {
			t.Errorf("net mismatch: %+v", c)
		}
		if c.Views > 0 && c.ConversionRate == 0 && c.NetSubscribers != 0 {
			t.Errorf("conversion rate not computed: %+v", c)
		}
	}
	best := BestSubscriberVideos(conv, 1)
	worst := WorstSubscriberVideos(conv, 1)
	if best[0].NetSubscribers < worst[0].NetSubscribers {
		t.Errorf("best should have >= net than worst")
	}
}

func TestCrossPlatform(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 14, syntheticSet(start)...)
	a := Analytics{Repo: repo, Config: DefaultConfig()}
	cp := a.CrossPlatform(addDays(start, 13))
	if len(cp.Platforms) != 3 {
		t.Fatalf("expected 3 platforms, got %d", len(cp.Platforms))
	}
	if cp.FastestGrowing == "" || cp.HighestEngagement == "" || cp.BestPlatform == "" {
		t.Errorf("cross-platform leaders not identified: %+v", cp)
	}
	// TikTok grows fastest in the synthetic set (80/day on 5000 base).
	if cp.FastestGrowing != PlatformTikTok {
		t.Errorf("fastest growing = %q, want TikTok", cp.FastestGrowing)
	}
}

// --- briefing + advisory ---

func TestMorningBriefing(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 14, syntheticSet(start)...)
	be := NewBriefingEngine(DefaultConfig(), repo, nil, func() time.Time { return parseDate("2026-07-14") })
	b := be.Generate(context.Background(), addDays(start, 13))

	if b.SchemaVersion != SchemaVersion || len(b.Platforms) != 3 {
		t.Fatalf("briefing envelope wrong: %+v", b.Platforms)
	}
	if b.NewFollowers <= 0 {
		t.Errorf("expected positive new followers, got %d", b.NewFollowers)
	}
	if len(b.BestContent) == 0 || len(b.Insights) == 0 || len(b.Recommendations) == 0 {
		t.Errorf("briefing under-populated")
	}
	if b.CrossPlatform.FastestGrowing == "" {
		t.Errorf("cross-platform missing")
	}
	// Markdown renders.
	md := b.Markdown()
	for _, want := range []string{"# Morning Briefing", "## Platform Performance", "## Cross-Platform", "## Top Content"} {
		if !strings.Contains(md, want) {
			t.Errorf("briefing markdown missing %q", want)
		}
	}
	// JSON round-trips (the Advisory Council receives JSON).
	if _, err := json.Marshal(b); err != nil {
		t.Errorf("briefing marshal: %v", err)
	}
}

func TestAdvisoryPackage(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 14, syntheticSet(start)...)
	be := NewBriefingEngine(DefaultConfig(), repo, nil, func() time.Time { return parseDate("2026-07-14") })
	adv := be.Advisory(context.Background(), addDays(start, 13))
	if len(adv.YesterdayPerformance) != 3 || len(adv.GrowthTrends) != 3 || len(adv.Recommendations) == 0 {
		t.Fatalf("advisory package under-populated: %+v", adv)
	}
	if _, err := json.Marshal(adv); err != nil {
		t.Errorf("advisory marshal: %v", err)
	}
}

// --- insights (deterministic + AI) ---

func TestDeterministicInsightsGrounded(t *testing.T) {
	g := InsightGrounding{
		Growth:        []GrowthMetrics{{Platform: PlatformYouTube, Metric: "followers", WeekOverWeek: Delta{Percent: 8}, Momentum: "accelerating"}},
		TopContent:    []ContentEffectiveness{{Platform: PlatformTikTok, Score: 82, EngagementRate: 6.1, ViralityScore: 40}},
		CrossPlatform: CrossPlatform{HighestEngagement: PlatformTikTok},
	}
	insights, recs := deterministicInsights(g)
	if len(insights) == 0 || len(recs) == 0 {
		t.Fatalf("expected grounded insights + recs")
	}
	var sawStrength bool
	for _, i := range insights {
		if i.Kind == "strength" && strings.Contains(i.Summary, "8") {
			sawStrength = true
		}
	}
	if !sawStrength {
		t.Errorf("expected a grounded strength citing the 8%% growth")
	}
}

type fakeInsighter struct{ calls int }

func (f *fakeInsighter) Insights(context.Context, InsightGrounding) (AINotes, error) {
	f.calls++
	return AINotes{Reviewer: "fake", Summary: "AI view.", Insights: []BusinessInsight{{Kind: "prediction", Summary: "Growth continues."}}}, nil
}

func TestBriefingMergesAIInsights(t *testing.T) {
	start := "2026-07-01"
	repo, _ := seed(t, start, 14, syntheticSet(start)...)
	ai := &fakeInsighter{}
	be := NewBriefingEngine(DefaultConfig(), repo, ai, func() time.Time { return parseDate("2026-07-14") })
	b := be.Generate(context.Background(), addDays(start, 13))
	if ai.calls == 0 {
		t.Error("AI insighter should be called")
	}
	var sawAI bool
	for _, i := range b.Insights {
		if i.Kind == "prediction" && i.Summary == "Growth continues." {
			sawAI = true
		}
	}
	if !sawAI {
		t.Error("AI insight not merged into the briefing")
	}
}

func TestModelInsighterParsing(t *testing.T) {
	m := NewModelInsighter(fakeModel(`ok {"summary":"s","insights":[{"kind":"strength","summary":"x"}],"recommendations":[]}`), "claude")
	notes, err := m.Insights(context.Background(), InsightGrounding{})
	if err != nil || notes.Summary != "s" || notes.Reviewer != "claude" || len(notes.Insights) != 1 {
		t.Fatalf("parse: %v, %+v", err, notes)
	}
	if _, err := NewModelInsighter(fakeModel("no json"), "claude").Insights(context.Background(), InsightGrounding{}); err == nil {
		t.Error("expected parse error on non-JSON")
	}
}

// --- providers (mock HTTP) ---

type mockDoer struct {
	status int
	body   string
	gotReq *http.Request
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	m.gotReq = req
	return &http.Response{StatusCode: m.status, Body: io.NopCloser(strings.NewReader(m.body)), Header: make(http.Header)}, nil
}

func TestYouTubeProviderParsing(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"items":[{"statistics":{"subscriberCount":"12000","viewCount":"3400000","videoCount":"42"}}]}`}
	p := NewYouTubeProvider(doer, mapCredentials(map[string]string{envYouTubeToken: "t"}), nil)
	acc, err := p.CollectAccount(context.Background(), "2026-07-14")
	if err != nil || acc.Followers != 12000 || acc.Views != 3400000 {
		t.Fatalf("youtube account: %v, %+v", err, acc)
	}
	if doer.gotReq.Header.Get("Authorization") != "Bearer t" {
		t.Error("youtube auth header not set")
	}
	// Missing token → auth error (not recoverable).
	nope := NewYouTubeProvider(doer, mapCredentials(nil), nil)
	if _, err := nope.CollectAccount(context.Background(), "2026-07-14"); errCode(err) != "auth" {
		t.Errorf("expected auth error, got %v", err)
	}
}

func TestXProviderParsing(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"data":{"public_metrics":{"followers_count":8000,"following_count":300,"tweet_count":1200}}}`}
	p := NewXProvider(doer, mapCredentials(map[string]string{envXBearer: "b", envXUserID: "u"}), nil)
	acc, err := p.CollectAccount(context.Background(), "2026-07-14")
	if err != nil || acc.Followers != 8000 {
		t.Fatalf("x account: %v, %+v", err, acc)
	}
}

func TestProviderHTTPErrorClassification(t *testing.T) {
	for status, code := range map[int]string{401: "auth", 429: "rate-limit", 402: "quota", 503: "platform-down"} {
		doer := &mockDoer{status: status, body: `{"error":"x"}`}
		p := NewYouTubeProvider(doer, mapCredentials(map[string]string{envYouTubeToken: "t"}), nil)
		_, err := p.CollectAccount(context.Background(), "2026-07-14")
		if errCode(err) != code {
			t.Errorf("status %d → %q, want %q", status, errCode(err), code)
		}
	}
}

func TestTikTokProviderParsing(t *testing.T) {
	doer := &mockDoer{status: 200, body: `{"data":{"user":{"follower_count":15000,"following_count":100,"likes_count":900000,"video_count":80}}}`}
	p := NewTikTokProvider(doer, mapCredentials(map[string]string{envTikTokToken: "t"}))
	acc, err := p.CollectAccount(context.Background(), "2026-07-14")
	if err != nil || acc.Followers != 15000 || acc.Extra["totalLikes"] != 900000 {
		t.Fatalf("tiktok account: %v, %+v", err, acc)
	}
}

func TestInstagramProviderParsing(t *testing.T) {
	acct := &mockDoer{status: 200, body: `{"followers_count":9000,"follows_count":500,"media_count":120}`}
	p := NewInstagramProvider(acct, mapCredentials(map[string]string{envInstagramToken: "t", envInstagramUser: "u"}), []string{"m1"})
	a, err := p.CollectAccount(context.Background(), "2026-07-14")
	if err != nil || a.Followers != 9000 || a.Following != 500 {
		t.Fatalf("ig account: %v, %+v", err, a)
	}
	media := &mockDoer{status: 200, body: `{"id":"m1","caption":"hello world","timestamp":"2026-07-10T00:00:00+0000","like_count":300,"comments_count":20}`}
	p.HTTP = media
	cs, err := p.CollectContent(context.Background(), "2026-07-14")
	if err != nil || len(cs) != 1 || cs[0].Likes != 300 {
		t.Fatalf("ig content: %v, %+v", err, cs)
	}
}

func TestLogMonitorAndNop(t *testing.T) {
	m := LogMonitor{}
	m.Count("c", 1, map[string]string{"p": "YouTube"})
	m.Duration("d", time.Second, nil)
	m.Error("e", errAuth("x"), nil)
	n := nopMonitor{}
	n.Count("c", 1, nil)
	n.Duration("d", 0, nil)
	n.Error("e", nil, nil)
}

func TestCollectTodayAndRegister(t *testing.T) {
	start := "2026-07-01"
	repo := NewMemoryRepository()
	col := NewCollector(DefaultConfig(), repo, nil, func() time.Time { return parseDate(start) })
	col.Sleep = func(context.Context, time.Duration) {}
	col.Register(NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2))
	r := col.CollectToday(context.Background())
	if !r.Platforms[PlatformYouTube].Collected {
		t.Errorf("CollectToday should collect the registered provider")
	}
}

func TestMomentumAndTrendingTags(t *testing.T) {
	// Accelerating series: growth speeds up in the second half.
	pts := []Point{
		{"2026-07-01", 100}, {"2026-07-02", 105}, {"2026-07-03", 110}, {"2026-07-04", 115},
		{"2026-07-05", 130}, {"2026-07-06", 150}, {"2026-07-07", 175}, {"2026-07-08", 205},
	}
	if m := momentum(pts); m != "accelerating" {
		t.Errorf("momentum = %q, want accelerating", m)
	}
	start := "2026-07-01"
	repo, _ := seed(t, start, 3, NewSyntheticProvider(PlatformYouTube, start, 10000, 50, 2))
	a := Analytics{Repo: repo, Config: DefaultConfig()}
	tags := a.TrendingTags(PlatformYouTube, addDays(start, 2), 5)
	if len(tags) == 0 {
		t.Error("expected grounded trending tags")
	}
}

func TestDisabledPlatformNotCollected(t *testing.T) {
	start := "2026-07-01"
	cfg := DefaultConfig()
	cfg.Enabled = []Platform{PlatformTikTok}
	repo := NewMemoryRepository()
	col := NewCollector(cfg, repo, []Provider{NewSyntheticProvider(PlatformYouTube, start, 1, 1, 1)}, func() time.Time { return parseDate(start) })
	col.Sleep = func(context.Context, time.Duration) {}
	r := col.Collect(context.Background(), start)
	if _, ok := r.Platforms[PlatformYouTube]; ok {
		t.Error("disabled platform should not be collected")
	}
}

func TestUnknownMetricAndEmptyGrowth(t *testing.T) {
	a := Analytics{Repo: NewMemoryRepository(), Config: DefaultConfig()}
	if s := a.Series(PlatformYouTube, "nonsense"); len(s.Points) != 0 {
		t.Error("unknown metric should yield an empty series")
	}
	if g := a.Growth(PlatformYouTube, "followers"); g.Current != 0 {
		t.Error("empty repo growth should be zero-valued")
	}
}

func TestWorstSubscriberReversal(t *testing.T) {
	conv := []SubscriberConversion{
		{ContentID: "a", NetSubscribers: 100}, {ContentID: "b", NetSubscribers: 50},
		{ContentID: "c", NetSubscribers: 10}, {ContentID: "d", NetSubscribers: 1},
	}
	worst := WorstSubscriberVideos(conv, 2)
	if len(worst) != 2 || worst[0].ContentID != "d" {
		t.Errorf("worst should start with the lowest, got %+v", worst)
	}
}

func TestBriefingAlertsOnDecline(t *testing.T) {
	// A platform whose followers shrink triggers an alert.
	start := "2026-07-01"
	repo := NewMemoryRepository()
	_ = repo.SaveAccountSnapshot(AccountSnapshot{Platform: PlatformX, Date: start, Followers: 1000})
	_ = repo.SaveAccountSnapshot(AccountSnapshot{Platform: PlatformX, Date: addDays(start, 1), Followers: 900})
	be := NewBriefingEngine(DefaultConfig(), repo, nil, func() time.Time { return parseDate(addDays(start, 1)) })
	b := be.Generate(context.Background(), addDays(start, 1))
	if len(b.Alerts) == 0 {
		t.Error("expected an alert for the follower decline")
	}
	if !strings.Contains(b.Markdown(), "lost") {
		t.Error("markdown should surface the alert")
	}
}

// --- helpers ---

type fakeModel string

func (f fakeModel) Generate(context.Context, string) (string, error) { return string(f), nil }
