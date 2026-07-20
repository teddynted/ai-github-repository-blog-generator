package contentanalytics

import (
	"fmt"
	"sort"
	"time"
)

// ReportEngine assembles the structured analytics report from stored snapshots.
// It is deterministic and grounded — no AI, no fabrication. (AI consumption
// happens in Milestone 18 via the OptimizationSignals bundle.)
type ReportEngine struct {
	Analytics Analytics
	Repo      Repository
	Config    Config
	Now       Clock
}

// NewReportEngine wires a report engine.
func NewReportEngine(repo Repository, cfg Config, now Clock) *ReportEngine {
	if now == nil {
		now = time.Now
	}
	return &ReportEngine{Analytics: NewAnalytics(repo, cfg), Repo: repo, Config: cfg, Now: now}
}

// Generate builds the report as of date.
func (e *ReportEngine) Generate(date Date) Report {
	n := e.Config.topN()
	perf := e.Analytics.Performance(date)
	plats := e.Analytics.PlatformSummaries(date)

	articles := filterByKind(perf, false)
	videos := filterByKind(perf, true)

	worst := append([]ContentPerformance{}, perf...)
	sort.SliceStable(worst, func(i, j int) bool { return worst[i].Score < worst[j].Score })

	r := Report{
		SchemaVersion:       SchemaVersion,
		GeneratedAt:         e.Now(),
		Window:              e.window(date),
		Platforms:           plats,
		ContentPerformance:  topN(perf, maxInt(n, 10)),
		EngagementBreakdown: e.engagementBreakdown(date),
		GrowthTrends:        e.growthTrends(date),
		TopArticles:         topN(articles, n),
		TopVideos:           topN(videos, n),
		WorstPerforming:     topN(worst, n),
		Historical:          e.historical(date),
		DataQuality:         e.dataQuality(date),
	}
	r.ExecutiveSummary = e.executiveSummary(r, perf, plats)
	r.Recommendations = e.recommendations(r, date)
	return r
}

func (e *ReportEngine) window(date Date) string {
	from := date
	for _, pub := range e.Repo.Publications() {
		for _, s := range e.Repo.Snapshots(pub.ID, PeriodDaily) {
			if s.Date < from {
				from = s.Date
			}
		}
	}
	return from + ".." + date
}

func (e *ReportEngine) engagementBreakdown(date Date) EngagementBreakdown {
	var b EngagementBreakdown
	for _, pub := range e.Repo.Publications() {
		s, ok := e.Analytics.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok {
			continue
		}
		b.Likes += s.Engagement.Likes + s.Engagement.Reactions
		b.Comments += s.Engagement.Comments + s.Engagement.Replies
		b.Shares += s.Engagement.Shares + s.Engagement.Reposts
		b.Saves += s.Engagement.Saves + s.Engagement.Bookmarks
	}
	b.Total = b.Likes + b.Comments + b.Shares + b.Saves + b.Other
	return b
}

// growthTrends returns per-platform aggregate views growth.
func (e *ReportEngine) growthTrends(date Date) []GrowthTrend {
	var out []GrowthTrend
	for _, plat := range e.Repo.Platforms() {
		g := e.aggregateGrowth(plat, "views", date)
		g.Platform = plat
		g.Metric = "views"
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Current > out[j].Current })
	return out
}

// historical returns per-platform aggregate engagement growth for the historical
// comparisons section.
func (e *ReportEngine) historical(date Date) []GrowthTrend {
	var out []GrowthTrend
	for _, plat := range e.Repo.Platforms() {
		g := e.aggregateGrowth(plat, "engagements", date)
		g.Platform = plat
		g.Metric = "engagements"
		out = append(out, g)
	}
	return out
}

// aggregateGrowth sums a metric across a platform's publications per date, then
// computes DoD/WoW/MoM.
func (e *ReportEngine) aggregateGrowth(plat Platform, metric string, date Date) GrowthTrend {
	totals := map[Date]float64{}
	for _, pub := range e.Repo.PublicationsByPlatform(plat) {
		for _, s := range e.Repo.Snapshots(pub.ID, PeriodDaily) {
			if s.Date <= date {
				totals[s.Date] += snapshotMetric(s, metric)
			}
		}
	}
	dates := make([]Date, 0, len(totals))
	for d := range totals {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	pts := make([]Point, 0, len(dates))
	for _, d := range dates {
		pts = append(pts, Point{Date: d, Value: totals[d]})
	}
	g := GrowthTrend{Metric: metric}
	if len(pts) == 0 {
		return g
	}
	cur := pts[len(pts)-1]
	g.Current = cur.Value
	g.DayOverDay = deltaBack(pts, cur.Date, 1)
	g.WeekOverWeek = deltaBack(pts, cur.Date, 7)
	g.MonthOverMonth = deltaBack(pts, cur.Date, 30)
	g.Direction = direction(g.WeekOverWeek.Percent)
	return g
}

func (e *ReportEngine) dataQuality(date Date) DataQuality {
	pubs := e.Repo.Publications()
	q := DataQuality{Publications: len(pubs)}
	withData := 0
	for _, pub := range pubs {
		snaps := e.Repo.Snapshots(pub.ID, PeriodDaily)
		q.Snapshots += len(snaps)
		has := false
		for _, s := range snaps {
			if s.Partial {
				q.PartialSnapshots++
			}
			if s.Date <= date {
				has = true
			}
		}
		if has {
			withData++
		}
	}
	if len(pubs) > 0 {
		q.CompletenessScore = round2(float64(withData) / float64(len(pubs)) * 100)
	} else {
		q.CompletenessScore = 100
	}
	for _, plat := range e.Repo.Platforms() {
		if len(e.Repo.PublicationsByPlatform(plat)) == 0 {
			q.MissingPlatforms = append(q.MissingPlatforms, string(plat))
		}
	}
	return q
}

func (e *ReportEngine) executiveSummary(r Report, perf []ContentPerformance, plats []PlatformSummary) string {
	var views, engagements int64
	for _, p := range plats {
		views += p.Views
		engagements += p.Engagements
	}
	rate := round2(safeDiv(float64(engagements), float64(views)) * 100)
	lead := "n/a"
	if len(plats) > 0 {
		lead = string(plats[0].Platform)
	}
	top := "n/a"
	if len(perf) > 0 {
		top = perf[0].Title
	}
	return fmt.Sprintf("%s views and %s engagements (%.2f%% engagement) across %d publications on %d platforms. %s leads by reach; top content: %q.",
		comma(views), comma(engagements), rate, r.DataQuality.Publications, len(plats), lead, top)
}

func (e *ReportEngine) recommendations(r Report, date Date) []string {
	var recs []string
	if len(r.Platforms) > 0 {
		recs = append(recs, fmt.Sprintf("Prioritize %s — it drives the most reach (%s views).", r.Platforms[0].Platform, comma(r.Platforms[0].Views)))
	}
	if tags := e.Analytics.TopTags(date, 1); len(tags) > 0 {
		recs = append(recs, fmt.Sprintf("Lean into the %q topic — its content leads on views (%.2f%% engagement).", tags[0].Tag, tags[0].EngagementRate))
	}
	if times := e.Analytics.TopPublishingTimes(date, 1); len(times) > 0 {
		recs = append(recs, fmt.Sprintf("Publish around %s ~%02d:00 UTC — historically the strongest window (avg %.0f views).", times[0].Weekday, times[0].Hour, times[0].AvgViews))
	}
	for _, p := range r.Platforms {
		if p.Views > 0 && p.CTR == 0 && p.EngagementRate < 1 {
			recs = append(recs, fmt.Sprintf("Investigate low %s engagement (%.2f%%) — review hooks and CTAs.", p.Platform, p.EngagementRate))
		}
	}
	if len(recs) == 0 {
		recs = append(recs, "Collect more history to unlock trend-based recommendations.")
	}
	return recs
}

// filterByKind splits video vs article content.
func filterByKind(in []ContentPerformance, video bool) []ContentPerformance {
	var out []ContentPerformance
	for _, c := range in {
		isVideo := c.ContentType == TypeYouTubeVideo || c.ContentType == TypeYouTubeShorts || c.ContentType == TypeTikTokVideo
		if isVideo == video {
			out = append(out, c)
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
