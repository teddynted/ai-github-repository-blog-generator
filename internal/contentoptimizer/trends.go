package contentoptimizer

import (
	"fmt"
	"sort"
)

// DefaultTrendAnalyzer detects multi-horizon trends grounded in the input series
// and the timestamped records. It classifies topics as emerging/declining by
// comparing a recent window to a prior window, and surfaces publishing windows,
// seasonal notes, and content-fatigue signals — never inventing a trend that the
// data does not show.
type DefaultTrendAnalyzer struct {
	Config Config
}

// NewTrendAnalyzer wires a trend analyzer.
func NewTrendAnalyzer(cfg Config) *DefaultTrendAnalyzer {
	return &DefaultTrendAnalyzer{Config: cfg}
}

// Analyze produces one TrendReport per horizon that the data can support.
func (a *DefaultTrendAnalyzer) Analyze(input AnalyticsInput) []TrendReport {
	var out []TrendReport
	horizons := []struct {
		name string
		days int
	}{
		{"daily", 1}, {"weekly", 7}, {"monthly", 30}, {"quarterly", 90}, {"long-term", 365},
	}
	for _, h := range horizons {
		rep, ok := a.horizon(input, h.name, h.days)
		if ok {
			out = append(out, rep)
		}
	}
	if len(out) == 0 {
		// Always emit at least a topic-based long-term view when records exist.
		if len(input.Records) > 0 {
			out = append(out, a.topicOnly(input, "long-term"))
		}
	}
	return out
}

// horizon builds a report for a window of `days`, using the views series to
// bound emerging/declining detection and the records for topic trajectories.
func (a *DefaultTrendAnalyzer) horizon(input AnalyticsInput, name string, days int) (TrendReport, bool) {
	series := input.Series["views"]
	// A horizon is reportable if the series covers at least 2×window, else skip
	// (we won't fabricate a trend we can't measure).
	if len(series) < days*2 && name != "long-term" {
		return TrendReport{}, false
	}
	rep := TrendReport{Horizon: name}
	emerging, declining := a.topicTrends(input, days)
	rep.Emerging = topN(emerging, a.Config.topN())
	rep.Declining = topN(declining, a.Config.topN())
	rep.PublishingWindows = a.windows(input)
	rep.SeasonalNotes = a.seasonal(series, days)
	rep.ContentFatigue = a.fatigue(rep.Declining)
	return rep, true
}

func (a *DefaultTrendAnalyzer) topicOnly(input AnalyticsInput, name string) TrendReport {
	emerging, declining := a.topicTrends(input, 0)
	return TrendReport{
		Horizon:           name,
		Emerging:          topN(emerging, a.Config.topN()),
		Declining:         topN(declining, a.Config.topN()),
		PublishingWindows: a.windows(input),
		ContentFatigue:    a.fatigue(declining),
	}
}

// topicTrends splits records into a prior and a recent cohort by published date
// and compares each tag's mean score to classify its trajectory.
func (a *DefaultTrendAnalyzer) topicTrends(input AnalyticsInput, windowDays int) (emerging, declining []TopicTrend) {
	dated := datedRecords(input.Records)
	if len(dated) < 2 {
		return nil, nil
	}
	split := splitPoint(dated, windowDays)
	prior := dated[:split]
	recent := dated[split:]
	if len(prior) == 0 || len(recent) == 0 {
		return nil, nil
	}

	priorByTag := tagScores(prior)
	recentByTag := tagScores(recent)
	tags := map[string]bool{}
	for t := range priorByTag {
		tags[t] = true
	}
	for t := range recentByTag {
		tags[t] = true
	}
	names := make([]string, 0, len(tags))
	for t := range tags {
		names = append(names, t)
	}
	sort.Strings(names)

	for _, t := range names {
		p := mean(priorByTag[t])
		r := mean(recentByTag[t])
		change := pct(r, p)
		support := len(priorByTag[t]) + len(recentByTag[t])
		conf := supportConfidence(support, clampFloat(abs(change)/100, 0, 1))
		tt := TopicTrend{Topic: t, ChangePct: change, Recent: round2(r), Prior: round2(p), Confidence: conf}
		switch {
		case change >= a.Config.trendChangePct():
			tt.Direction = TrendEmerging
			emerging = append(emerging, tt)
		case change <= -a.Config.trendChangePct():
			tt.Direction = TrendDeclining
			declining = append(declining, tt)
		default:
			tt.Direction = TrendStable
		}
	}
	sort.SliceStable(emerging, func(i, j int) bool { return emerging[i].ChangePct > emerging[j].ChangePct })
	sort.SliceStable(declining, func(i, j int) bool { return declining[i].ChangePct < declining[j].ChangePct })
	return emerging, declining
}

func (a *DefaultTrendAnalyzer) windows(input AnalyticsInput) []string {
	var out []string
	for _, t := range topN(input.BestTimes, 3) {
		out = append(out, fmt.Sprintf("%s ~%02d:00 UTC (avg %.0f views, %.2f%% engagement)", t.Weekday, t.Hour, t.AvgViews, t.EngagementRate))
	}
	return out
}

// seasonal notes only when the series is long enough to compare halves.
func (a *DefaultTrendAnalyzer) seasonal(series []SeriesPoint, days int) []string {
	if len(series) < days*2 || days == 0 {
		return nil
	}
	half := len(series) / 2
	prior := seriesMean(series[:half])
	recent := seriesMean(series[half:])
	change := pct(recent, prior)
	if abs(change) < a.Config.trendChangePct() {
		return nil
	}
	dir := "up"
	if change < 0 {
		dir = "down"
	}
	return []string{fmt.Sprintf("Overall views trending %s %.1f%% across the window.", dir, abs(change))}
}

// fatigue flags declining topics as potential content fatigue.
func (a *DefaultTrendAnalyzer) fatigue(declining []TopicTrend) []string {
	var out []string
	for _, t := range topN(declining, 3) {
		out = append(out, fmt.Sprintf("%q may be fatiguing (%.1f%% decline).", t.Topic, t.ChangePct))
	}
	return out
}

// --- helpers ---

func datedRecords(records []PerformanceRecord) []PerformanceRecord {
	var out []PerformanceRecord
	for _, r := range records {
		if !r.PublishedAt.IsZero() {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublishedAt.Before(out[j].PublishedAt) })
	return out
}

// splitPoint chooses the prior/recent boundary. With a window, recent = records
// published within `windowDays` of the latest; else split in half.
func splitPoint(dated []PerformanceRecord, windowDays int) int {
	if windowDays <= 0 {
		return len(dated) / 2
	}
	latest := dated[len(dated)-1].PublishedAt
	cutoff := latest.AddDate(0, 0, -windowDays)
	for i, r := range dated {
		if r.PublishedAt.After(cutoff) {
			if i == 0 {
				return len(dated) / 2 // everything is recent → fall back to halves
			}
			return i
		}
	}
	return len(dated) / 2
}

func tagScores(records []PerformanceRecord) map[string][]float64 {
	m := map[string][]float64{}
	for _, r := range records {
		for _, t := range r.Tags {
			m[t] = append(m[t], r.Score)
		}
	}
	return m
}

func seriesMean(pts []SeriesPoint) float64 {
	var xs []float64
	for _, p := range pts {
		xs = append(xs, p.Value)
	}
	return mean(xs)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
