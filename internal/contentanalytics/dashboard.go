package contentanalytics

import (
	"context"
	"sort"
	"time"
)

// DashboardEngine builds dashboard-ready datasets (CloudWatch today; Grafana /
// web / advisory-council in future) and can push the headline metrics to a
// MetricsPublisher.
type DashboardEngine struct {
	Analytics Analytics
	Repo      Repository
	Config    Config
	Metrics   MetricsPublisher
	Now       Clock
}

// NewDashboardEngine wires a dashboard engine.
func NewDashboardEngine(repo Repository, cfg Config, metrics MetricsPublisher, now Clock) *DashboardEngine {
	if now == nil {
		now = time.Now
	}
	if metrics == nil {
		metrics = nopMetricsPublisher{}
	}
	return &DashboardEngine{Analytics: NewAnalytics(repo, cfg), Repo: repo, Config: cfg, Metrics: metrics, Now: now}
}

// Dataset builds a dashboard dataset as of date: per-metric daily series plus
// platform summaries and headline totals.
func (e *DashboardEngine) Dataset(date Date) DashboardDataset {
	plats := e.Analytics.PlatformSummaries(date)
	ds := DashboardDataset{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   e.Now(),
		Platforms:     plats,
		Series: []DashboardSeries{
			{Name: "TotalViews", Unit: "Count", Points: e.totalSeries("views", date)},
			{Name: "TotalEngagements", Unit: "Count", Points: e.totalSeries("engagements", date)},
			{Name: "EngagementRate", Unit: "Percent", Points: e.rateSeries(date)},
			{Name: "WatchTimeMinutes", Unit: "Count", Points: e.totalSeries("watchTime", date)},
		},
	}
	for _, p := range plats {
		ds.Totals.Views += p.Views
		ds.Totals.Engagements += p.Engagements
		ds.Totals.WatchMinutes += p.WatchTimeMinutes
		ds.Totals.Publications += p.Publications
	}
	ds.Totals.EngagementRate = round2(safeDiv(float64(ds.Totals.Engagements), float64(ds.Totals.Views)) * 100)
	return ds
}

// totalSeries sums a metric across all publications per date up to date.
func (e *DashboardEngine) totalSeries(metric string, date Date) []Point {
	totals := map[Date]float64{}
	for _, pub := range e.Repo.Publications() {
		for _, s := range e.Repo.Snapshots(pub.ID, PeriodDaily) {
			if s.Date <= date {
				totals[s.Date] += snapshotMetric(s, metric)
			}
		}
	}
	return sortedPoints(totals)
}

// rateSeries is total engagements / total views per date (as a percentage).
func (e *DashboardEngine) rateSeries(date Date) []Point {
	views := map[Date]float64{}
	eng := map[Date]float64{}
	for _, pub := range e.Repo.Publications() {
		for _, s := range e.Repo.Snapshots(pub.ID, PeriodDaily) {
			if s.Date <= date {
				views[s.Date] += float64(s.Reach.Views)
				eng[s.Date] += float64(s.Engagement.Total())
			}
		}
	}
	rate := map[Date]float64{}
	for d, v := range views {
		rate[d] = round2(safeDiv(eng[d], v) * 100)
	}
	return sortedPoints(rate)
}

func sortedPoints(m map[Date]float64) []Point {
	dates := make([]Date, 0, len(m))
	for d := range m {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	pts := make([]Point, 0, len(dates))
	for _, d := range dates {
		pts = append(pts, Point{Date: d, Value: m[d]})
	}
	return pts
}

// Publish pushes the dataset's headline metrics + per-platform breakdown to the
// MetricsPublisher (CloudWatch). It publishes the cumulative TotalViews, overall
// EngagementRate, per-platform views/engagement/CTR/watch-time.
func (e *DashboardEngine) Publish(ctx context.Context, date Date) error {
	ds := e.Dataset(date)
	ts := e.Now()
	metrics := []Metric{
		metric(MetricTotalViews, float64(ds.Totals.Views), "Count", nil, ts),
		metric(MetricEngagementRate, ds.Totals.EngagementRate, "Percent", nil, ts),
		metric(MetricWatchTime, float64(ds.Totals.WatchMinutes), "Count", nil, ts),
	}
	for _, p := range ds.Platforms {
		dim := map[string]string{"platform": string(p.Platform)}
		metrics = append(metrics,
			metric(MetricTotalViews, float64(p.Views), "Count", dim, ts),
			metric(MetricEngagementRate, p.EngagementRate, "Percent", dim, ts),
			metric(MetricCTR, p.CTR, "Percent", dim, ts),
			metric(MetricWatchTime, float64(p.WatchTimeMinutes), "Count", dim, ts),
		)
	}
	return e.Metrics.Publish(ctx, e.Config.namespace(), metrics)
}
