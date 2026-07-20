package contentoptimizer

import (
	ca "github.com/teddynted/ai-github-repository-blog-generator/internal/contentanalytics"
)

// FromContentAnalytics adapts Milestone 17 output into the optimizer's grounded
// AnalyticsInput. It is the single seam between the analytics and optimization
// packages: the optimizer depends on its own AnalyticsInput, not on M17 types,
// so either side can evolve independently. All values are copied verbatim — the
// optimizer never invents data the analytics did not report.
func FromContentAnalytics(report ca.Report, signals ca.OptimizationSignals, series map[string][]SeriesPoint) AnalyticsInput {
	in := AnalyticsInput{AsOf: report.Window, Series: series}

	for _, c := range report.ContentPerformance {
		in.Records = append(in.Records, PerformanceRecord{
			PublicationID:  c.PublicationID,
			Platform:       Platform(c.Platform),
			ContentType:    ContentType(c.ContentType),
			Title:          c.Title,
			Views:          c.Views,
			CTR:            c.CTR,
			EngagementRate: c.EngagementRate,
			RetentionPct:   c.RetentionPct,
			WatchMinutes:   c.WatchMinutes,
			Engagements:    c.Engagements,
			Score:          c.Score,
		})
	}
	for _, p := range report.Platforms {
		in.Platforms = append(in.Platforms, PlatformStat{
			Platform:        Platform(p.Platform),
			Publications:    p.Publications,
			Views:           p.Views,
			Engagements:     p.Engagements,
			EngagementRate:  p.EngagementRate,
			CTR:             p.CTR,
			AvgRetentionPct: p.AvgRetentionPct,
			WatchMinutes:    p.WatchTimeMinutes,
		})
	}
	for _, t := range signals.StrongestHashtags {
		in.TopTags = append(in.TopTags, TagStat{Tag: t.Tag, Publications: t.Publications, Views: t.Views, EngagementRate: t.EngagementRate})
	}
	for _, k := range signals.BestTopics {
		in.TopKeywords = append(in.TopKeywords, TopicStat{Keyword: k.Keyword, Publications: k.Publications, Views: k.Views, EngagementRate: k.EngagementRate})
	}
	for _, ti := range signals.BestPublishingTimes {
		in.BestTimes = append(in.BestTimes, TimeStat{Weekday: ti.Weekday, Hour: ti.Hour, Publications: ti.Publications, AvgViews: ti.AvgViews, EngagementRate: ti.EngagementRate})
	}
	return in
}

// EnrichTags attaches per-record tags/keywords from a lookup keyed by publication
// id (M17 ContentPerformance omits them). It lets callers ground topic patterns
// in real publication metadata without changing the adapter contract.
func (in *AnalyticsInput) EnrichTags(tags, keywords map[string][]string) {
	for i := range in.Records {
		id := in.Records[i].PublicationID
		if t, ok := tags[id]; ok {
			in.Records[i].Tags = dedupeStr(t)
		}
		if k, ok := keywords[id]; ok {
			in.Records[i].Keywords = dedupeStr(k)
		}
	}
}
