package contentanalytics

import (
	"fmt"
	"sort"
	"time"
)

// OptimizationEngine produces the structured OptimizationSignals bundle consumed
// by Milestone 18 (AI Content Optimization). It performs NO optimization — it
// only surfaces grounded signals derived from stored analytics. Every signal is
// traceable to a snapshot.
type OptimizationEngine struct {
	Analytics Analytics
	Repo      Repository
	Config    Config
	Now       Clock
}

// NewOptimizationEngine wires an optimization-signal engine.
func NewOptimizationEngine(repo Repository, cfg Config, now Clock) *OptimizationEngine {
	if now == nil {
		now = time.Now
	}
	return &OptimizationEngine{Analytics: NewAnalytics(repo, cfg), Repo: repo, Config: cfg, Now: now}
}

// Signals builds the optimization-signal bundle as of date.
func (e *OptimizationEngine) Signals(date Date) OptimizationSignals {
	n := e.Config.topN()
	perf := e.Analytics.Performance(date)

	byCTR := append([]ContentPerformance{}, perf...)
	sort.SliceStable(byCTR, func(i, j int) bool { return byCTR[i].CTR > byCTR[j].CTR })
	byRet := filterHasRetention(perf)
	sort.SliceStable(byRet, func(i, j int) bool { return byRet[i].RetentionPct > byRet[j].RetentionPct })

	return OptimizationSignals{
		SchemaVersion:           SchemaVersion,
		GeneratedAt:             e.Now(),
		BestPublishingTimes:     e.Analytics.TopPublishingTimes(date, n),
		BestTopics:              e.Analytics.TopKeywords(date, n),
		StrongestHashtags:       e.Analytics.TopTags(date, n),
		HighestCTR:              topN(byCTR, n),
		BestRetention:           topN(byRet, n),
		SuccessfulPatterns:      e.patterns(perf, date),
		PlatformRecommendations: e.platformRecommendations(date),
	}
}

// patterns extracts grounded, descriptive patterns from the top performers.
func (e *OptimizationEngine) patterns(perf []ContentPerformance, date Date) []string {
	var out []string
	if len(perf) == 0 {
		return out
	}
	top := perf[0]
	out = append(out, fmt.Sprintf("Top content %q (%s) scored %.0f/100 with %.2f%% engagement.", top.Title, top.Platform, top.Score, top.EngagementRate))
	if tags := e.Analytics.TopTags(date, 2); len(tags) > 0 {
		names := make([]string, 0, len(tags))
		for _, t := range tags {
			names = append(names, t.Tag)
		}
		out = append(out, "Highest-reach topics: "+join(names))
	}
	// Which content type dominates the top quartile.
	cut := maxInt(1, len(perf)/4)
	kinds := map[ContentType]int{}
	for _, c := range topN(perf, cut) {
		kinds[c.ContentType]++
	}
	if ct, count := dominantKind(kinds); count > 0 {
		out = append(out, fmt.Sprintf("%q dominates the top quartile of performers.", ct))
	}
	return out
}

func (e *OptimizationEngine) platformRecommendations(date Date) []PlatformRecommendation {
	var out []PlatformRecommendation
	for _, p := range e.Analytics.PlatformSummaries(date) {
		note := fmt.Sprintf("%s: %.2f%% engagement over %d publications.", p.Platform, p.EngagementRate, p.Publications)
		out = append(out, PlatformRecommendation{
			Platform: p.Platform,
			Note:     note,
			Evidence: []string{fmt.Sprintf("views=%d", p.Views), fmt.Sprintf("engagements=%d", p.Engagements)},
		})
	}
	return out
}

func filterHasRetention(in []ContentPerformance) []ContentPerformance {
	var out []ContentPerformance
	for _, c := range in {
		if c.RetentionPct > 0 {
			out = append(out, c)
		}
	}
	return out
}

func dominantKind(kinds map[ContentType]int) (ContentType, int) {
	var best ContentType
	max := 0
	// Iterate deterministically for stable output.
	types := make([]ContentType, 0, len(kinds))
	for k := range kinds {
		types = append(types, k)
	}
	sort.SliceStable(types, func(i, j int) bool { return string(types[i]) < string(types[j]) })
	for _, k := range types {
		if kinds[k] > max {
			best, max = k, kinds[k]
		}
	}
	return best, max
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
