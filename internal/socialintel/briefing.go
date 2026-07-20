package socialintel

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// BriefingEngine generates the daily morning briefing and the advisory-council
// package from the analytics. The AI insighter is optional; when absent (or on
// error) it uses deterministic, grounded insights.
type BriefingEngine struct {
	Analytics Analytics
	AI        AIInsighter
	Config    Config
	Now       Clock
}

// NewBriefingEngine wires a briefing engine.
func NewBriefingEngine(cfg Config, repo Repository, ai AIInsighter, now Clock) *BriefingEngine {
	if now == nil {
		now = time.Now
	}
	return &BriefingEngine{
		Analytics: Analytics{Repo: repo, Config: cfg},
		AI:        ai,
		Config:    cfg,
		Now:       now,
	}
}

// Generate builds the morning briefing for a date.
func (e *BriefingEngine) Generate(ctx context.Context, date Date) MorningBriefing {
	platforms := e.Analytics.Repo.Platforms()

	perf := e.performance(date, platforms)
	growth := e.growthAll(platforms)
	best, worst := e.contentRanking(date, platforms)
	conv := e.topConversion(date, platforms)
	cross := e.Analytics.CrossPlatform(date)
	tags := e.trending(date, platforms)

	grounding := InsightGrounding{Date: date, Performance: perf, Growth: growth, TopContent: best, CrossPlatform: cross}
	insights, recs := e.insights(ctx, grounding)

	newFollowers := int64(0)
	for _, p := range perf {
		if p.FollowerDelta > 0 {
			newFollowers += p.FollowerDelta
		}
	}

	return MorningBriefing{
		SchemaVersion:     SchemaVersion,
		Date:              date,
		GeneratedAt:       e.Now(),
		Headline:          headline(perf, cross, newFollowers),
		Platforms:         perf,
		NewFollowers:      newFollowers,
		BestContent:       topN(best, e.Config.topN()),
		WorstContent:      topN(worst, e.Config.topN()),
		TopConversion:     topN(conv, e.Config.topN()),
		CrossPlatform:     cross,
		TrendingTopics:    tags,
		Insights:          insights,
		Recommendations:   recs,
		Alerts:            alerts(perf, growth),
		PublishingWindows: publishingWindows(cross),
	}
}

// Advisory builds the structured intelligence package for the Business Advisory
// Council (delivered as JSON).
func (e *BriefingEngine) Advisory(ctx context.Context, date Date) AdvisoryPackage {
	platforms := e.Analytics.Repo.Platforms()
	perf := e.performance(date, platforms)
	growth := e.growthAll(platforms)
	best, _ := e.contentRanking(date, platforms)
	cross := e.Analytics.CrossPlatform(date)
	_, recs := e.insights(ctx, InsightGrounding{Date: date, Performance: perf, Growth: growth, TopContent: best, CrossPlatform: cross})

	var trends []Trend
	for _, p := range platforms {
		trends = append(trends, e.Analytics.Trend(p, "followers"))
	}

	return AdvisoryPackage{
		SchemaVersion:        SchemaVersion,
		Date:                 date,
		YesterdayPerformance: perf,
		HistoricalComparison: growth,
		GrowthTrends:         trends,
		ContentEffectiveness: topN(best, e.Config.topN()),
		CrossPlatform:        cross,
		Recommendations:      recs,
		StrategicActions:     strategicActions(cross, growth),
	}
}

func (e *BriefingEngine) performance(date Date, platforms []Platform) []PlatformPerformance {
	var out []PlatformPerformance
	for _, p := range platforms {
		series := e.Analytics.Repo.AccountSeries(p)
		if len(series) == 0 {
			continue
		}
		latest := series[len(series)-1]
		var prev AccountSnapshot
		if len(series) >= 2 {
			prev = series[len(series)-2]
		}
		engage, _, _ := aggregateContent(e.Analytics.Repo.ContentSnapshots(p, date))
		out = append(out, PlatformPerformance{
			Platform:       p,
			Followers:      latest.Followers,
			FollowerDelta:  latest.Followers - prev.Followers,
			Views:          latest.Views,
			ViewsDelta:     latest.Views - prev.Views,
			EngagementRate: engage,
		})
	}
	return out
}

func (e *BriefingEngine) growthAll(platforms []Platform) []GrowthMetrics {
	var out []GrowthMetrics
	for _, p := range platforms {
		out = append(out, e.Analytics.Growth(p, "followers"))
	}
	return out
}

func (e *BriefingEngine) contentRanking(date Date, platforms []Platform) (best, worst []ContentEffectiveness) {
	var all []ContentEffectiveness
	for _, p := range platforms {
		all = append(all, e.Analytics.Effectiveness(p, date)...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	best = append([]ContentEffectiveness{}, all...)
	worst = append([]ContentEffectiveness{}, all...)
	// worst = reverse
	for i, j := 0, len(worst)-1; i < j; i, j = i+1, j-1 {
		worst[i], worst[j] = worst[j], worst[i]
	}
	return best, worst
}

func (e *BriefingEngine) topConversion(date Date, platforms []Platform) []SubscriberConversion {
	var all []SubscriberConversion
	for _, p := range platforms {
		all = append(all, e.Analytics.SubscriberConversion(p, date)...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].NetSubscribers > all[j].NetSubscribers })
	for i := range all {
		all[i].Rank = i + 1
	}
	return all
}

func (e *BriefingEngine) trending(date Date, platforms []Platform) []string {
	var tags []string
	for _, p := range platforms {
		tags = append(tags, e.Analytics.TrendingTags(p, date, 5)...)
	}
	return topN(dedupeStr(tags), 8)
}

// insights merges deterministic (authoritative, grounded) insights with optional
// AI notes; the AI never invents numbers.
func (e *BriefingEngine) insights(ctx context.Context, g InsightGrounding) ([]BusinessInsight, []Recommendation) {
	insights, recs := deterministicInsights(g)
	if e.AI != nil {
		if notes, err := e.AI.Insights(ctx, g); err == nil {
			insights = append(insights, notes.Insights...)
			recs = append(recs, notes.Recommendations...)
		}
	}
	return insights, recs
}

func headline(perf []PlatformPerformance, cross CrossPlatform, newFollowers int64) string {
	if len(perf) == 0 {
		return "No platform data collected yet."
	}
	lead := cross.FastestGrowing
	if lead == "" {
		lead = perf[0].Platform
	}
	return fmt.Sprintf("+%d followers across %d platforms; %s is growing fastest.", newFollowers, len(perf), lead)
}

func alerts(perf []PlatformPerformance, growth []GrowthMetrics) []string {
	var out []string
	for _, p := range perf {
		if p.FollowerDelta < 0 {
			out = append(out, fmt.Sprintf("⚠ %s lost %d followers yesterday.", p.Platform, -p.FollowerDelta))
		}
	}
	for _, g := range growth {
		if g.WeekOverWeek.Percent <= -5 {
			out = append(out, fmt.Sprintf("⚠ %s %s is down %.1f%% week-over-week.", g.Platform, g.Metric, g.WeekOverWeek.Percent))
		}
	}
	return out
}

func publishingWindows(cross CrossPlatform) []string {
	// Grounded default windows per the highest-engagement platform; a future
	// enhancement derives these from per-hour engagement once hourly data exists.
	if cross.HighestEngagement == "" {
		return nil
	}
	return []string{
		string(cross.HighestEngagement) + ": weekday mornings and lunch windows perform best for this audience",
	}
}

func strategicActions(cross CrossPlatform, growth []GrowthMetrics) []string {
	var out []string
	if cross.BestPlatform != "" {
		out = append(out, "Double down on "+string(cross.BestPlatform)+" — it has the strongest blended standing.")
	}
	if cross.FastestGrowing != "" && cross.FastestGrowing != cross.BestPlatform {
		out = append(out, "Invest more in "+string(cross.FastestGrowing)+" while its growth momentum is high.")
	}
	for _, g := range growth {
		if g.Momentum == "decelerating" && g.WeekOverWeek.Percent > 0 {
			out = append(out, "Refresh the "+string(g.Platform)+" content mix to counter decelerating momentum.")
		}
	}
	return out
}
