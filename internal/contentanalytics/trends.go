package contentanalytics

import "sort"

// Analytics computes trends, rankings, and aggregates from the immutable
// snapshot history. It is deterministic — the same snapshots always produce the
// same result — and grounded: every number comes from a stored snapshot.
type Analytics struct {
	Repo   Repository
	Config Config
}

// NewAnalytics wires an analytics engine.
func NewAnalytics(repo Repository, cfg Config) Analytics {
	return Analytics{Repo: repo, Config: cfg}
}

// snapshotMetric extracts a scalar metric from a snapshot.
func snapshotMetric(s MetricsSnapshot, metric string) float64 {
	switch metric {
	case "views":
		return float64(s.Reach.Views)
	case "impressions":
		return float64(s.Reach.Impressions)
	case "engagements":
		return float64(s.Engagement.Total())
	case "engagementRate":
		return s.EngagementRate()
	case "ctr":
		return s.Click.CTR
	case "watchTime":
		return float64(s.Watch.WatchTimeMinutes)
	case "subscribers":
		return float64(s.Growth.SubscribersGained - s.Growth.SubscribersLost)
	default:
		return 0
	}
}

// latestOnOrBefore returns a publication's newest snapshot at/on-or-before date.
func (a Analytics) latestOnOrBefore(pubID string, period Period, date Date) (MetricsSnapshot, bool) {
	var best MetricsSnapshot
	found := false
	for _, s := range a.Repo.Snapshots(pubID, period) {
		if s.Date <= date {
			best, found = s, true
		}
	}
	return best, found
}

// Series returns a metric's daily historical series for a publication.
func (a Analytics) Series(pubID string, period Period, metric string) []Point {
	snaps := a.Repo.Snapshots(pubID, period)
	pts := make([]Point, 0, len(snaps))
	for _, s := range snaps {
		pts = append(pts, Point{Date: s.Date, Value: snapshotMetric(s, metric)})
	}
	return pts
}

// Growth computes DoD/WoW/MoM growth of a publication metric from daily snapshots.
// Deltas are looked up by date (not array index) so gaps don't distort them.
func (a Analytics) Growth(pubID, metric string) GrowthTrend {
	pts := a.Series(pubID, PeriodDaily, metric)
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

// deltaBack compares the value at curDate to the value `back` days earlier.
func deltaBack(pts []Point, curDate Date, back int) Delta {
	cur := valueOnOrBefore(pts, curDate)
	prev := valueOnOrBefore(pts, addDays(curDate, -back))
	return Delta{Absolute: round2(cur - prev), Percent: pct(cur, prev)}
}

// valueOnOrBefore returns the value of the latest point on-or-before target.
func valueOnOrBefore(pts []Point, target Date) float64 {
	val := 0.0
	found := false
	for _, p := range pts {
		if p.Date <= target {
			val, found = p.Value, true
		}
	}
	if !found && len(pts) > 0 {
		return pts[0].Value
	}
	return val
}

// Performance ranks every publication by a blended effectiveness score using its
// latest daily snapshot on-or-before date.
func (a Analytics) Performance(date Date) []ContentPerformance {
	var out []ContentPerformance
	var maxViews int64 = 1
	for _, pub := range a.Repo.Publications() {
		s, ok := a.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok {
			continue
		}
		if s.Reach.Views > maxViews {
			maxViews = s.Reach.Views
		}
		out = append(out, ContentPerformance{
			PublicationID:  pub.ID,
			ContentID:      pub.ContentID,
			Platform:       pub.Platform,
			ContentType:    pub.ContentType,
			Title:          pub.Title,
			URL:            pub.URL,
			Views:          s.Reach.Views,
			Engagements:    s.Engagement.Total(),
			EngagementRate: s.EngagementRate(),
			CTR:            s.Click.CTR,
			RetentionPct:   s.Watch.RetentionPct,
			WatchMinutes:   s.Watch.WatchTimeMinutes,
		})
	}
	for i := range out {
		out[i].Score = performanceScore(out[i], maxViews)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// performanceScore blends engagement, reach, click-through, and retention (0–100).
func performanceScore(c ContentPerformance, maxViews int64) float64 {
	engage := clampFloat(c.EngagementRate*2, 0, 40)
	reach := clampFloat(safeDiv(float64(c.Views), float64(maxViews))*100*0.25, 0, 25)
	ctr := clampFloat(c.CTR, 0, 20)
	ret := clampFloat(c.RetentionPct*0.15, 0, 15)
	return round2(clampFloat(engage+reach+ctr+ret, 0, 100))
}

// PlatformSummaries aggregates each platform's performance for date.
func (a Analytics) PlatformSummaries(date Date) []PlatformSummary {
	byPlat := map[Platform]*PlatformSummary{}
	var retSum = map[Platform]float64{}
	var retN = map[Platform]int{}
	for _, pub := range a.Repo.Publications() {
		s, ok := a.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok {
			continue
		}
		sum := byPlat[pub.Platform]
		if sum == nil {
			sum = &PlatformSummary{Platform: pub.Platform}
			byPlat[pub.Platform] = sum
		}
		sum.Publications++
		sum.Views += s.Reach.Views
		sum.Impressions += s.Reach.Impressions
		sum.Engagements += s.Engagement.Total()
		sum.WatchTimeMinutes += s.Watch.WatchTimeMinutes
		sum.SubscribersGained += s.Growth.SubscribersGained
		if s.Watch.RetentionPct > 0 {
			retSum[pub.Platform] += s.Watch.RetentionPct
			retN[pub.Platform]++
		}
	}
	var out []PlatformSummary
	for plat, sum := range byPlat {
		sum.EngagementRate = round2(safeDiv(float64(sum.Engagements), float64(sum.Views)) * 100)
		if retN[plat] > 0 {
			sum.AvgRetentionPct = round2(retSum[plat] / float64(retN[plat]))
		}
		out = append(out, *sum)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Views > out[j].Views })
	return out
}

// TopTags aggregates performance by publication tag.
func (a Analytics) TopTags(date Date, n int) []TagSignal {
	type acc struct {
		pubs   int
		views  int64
		engage int64
	}
	m := map[string]*acc{}
	for _, pub := range a.Repo.Publications() {
		s, ok := a.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok {
			continue
		}
		for _, t := range dedupeStr(pub.Tags) {
			e := m[t]
			if e == nil {
				e = &acc{}
				m[t] = e
			}
			e.pubs++
			e.views += s.Reach.Views
			e.engage += s.Engagement.Total()
		}
	}
	var out []TagSignal
	for tag, e := range m {
		out = append(out, TagSignal{Tag: tag, Publications: e.pubs, Views: e.views,
			EngagementRate: round2(safeDiv(float64(e.engage), float64(e.views)) * 100)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Views > out[j].Views })
	return topN(out, n)
}

// TopKeywords aggregates performance by publication keyword.
func (a Analytics) TopKeywords(date Date, n int) []TopicSignal {
	type acc struct {
		pubs   int
		views  int64
		engage int64
	}
	m := map[string]*acc{}
	for _, pub := range a.Repo.Publications() {
		s, ok := a.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok {
			continue
		}
		for _, k := range dedupeStr(pub.Keywords) {
			e := m[k]
			if e == nil {
				e = &acc{}
				m[k] = e
			}
			e.pubs++
			e.views += s.Reach.Views
			e.engage += s.Engagement.Total()
		}
	}
	var out []TopicSignal
	for kw, e := range m {
		out = append(out, TopicSignal{Keyword: kw, Publications: e.pubs, Views: e.views,
			EngagementRate: round2(safeDiv(float64(e.engage), float64(e.views)) * 100)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Views > out[j].Views })
	return topN(out, n)
}

// TopPublishingTimes aggregates performance by publish weekday+hour.
func (a Analytics) TopPublishingTimes(date Date, n int) []PublishingTime {
	type acc struct {
		pubs    int
		views   int64
		engage  int64
		weekday string
		hour    int
	}
	m := map[string]*acc{}
	for _, pub := range a.Repo.Publications() {
		s, ok := a.latestOnOrBefore(pub.ID, PeriodDaily, date)
		if !ok || pub.PublishedAt.IsZero() {
			continue
		}
		wd := pub.PublishedAt.UTC().Weekday().String()
		hr := pub.PublishedAt.UTC().Hour()
		key := wd + "|" + itoa(int64(hr))
		e := m[key]
		if e == nil {
			e = &acc{weekday: wd, hour: hr}
			m[key] = e
		}
		e.pubs++
		e.views += s.Reach.Views
		e.engage += s.Engagement.Total()
	}
	var out []PublishingTime
	for _, e := range m {
		out = append(out, PublishingTime{
			Weekday: e.weekday, Hour: e.hour, Publications: e.pubs,
			AvgViews:       round2(safeDiv(float64(e.views), float64(e.pubs))),
			EngagementRate: round2(safeDiv(float64(e.engage), float64(e.views)) * 100),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AvgViews > out[j].AvgViews })
	return topN(out, n)
}
