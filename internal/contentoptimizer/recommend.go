package contentoptimizer

import (
	"fmt"
	"sort"
)

// DefaultRecommendationEngine turns detected patterns and analytics aggregates
// into grounded, confidence-scored recommendations. Every recommendation cites
// the evidence that produced it; none is emitted without supporting analytics.
type DefaultRecommendationEngine struct {
	Config Config
}

// NewRecommendationEngine wires a recommendation engine.
func NewRecommendationEngine(cfg Config) *DefaultRecommendationEngine {
	return &DefaultRecommendationEngine{Config: cfg}
}

// Recommend produces the grounded recommendation set.
func (e *DefaultRecommendationEngine) Recommend(input AnalyticsInput, winning, losing []Pattern) []Recommendation {
	var recs []Recommendation

	// Content strategy from winning patterns.
	for _, p := range winning {
		recs = append(recs, Recommendation{
			Category:   CatContentStrategy,
			Title:      "Replicate: " + p.Name,
			Detail:     "Produce more content matching this winning signature: " + join(p.SignalChain),
			Rationale:  p.Description,
			Evidence:   p.Evidence,
			Confidence: p.Confidence,
			PatternID:  p.ID,
		})
	}
	// Corrective strategy from losing patterns.
	for _, p := range losing {
		recs = append(recs, Recommendation{
			Category:   CatContentStrategy,
			Title:      "Avoid: " + p.Name,
			Detail:     "Rework content matching this underperforming signature: " + join(p.SignalChain),
			Rationale:  p.Description,
			Evidence:   p.Evidence,
			Confidence: p.Confidence,
			PatternID:  p.ID,
		})
	}

	// Publishing strategy from best times.
	if len(input.BestTimes) > 0 {
		t := input.BestTimes[0]
		recs = append(recs, Recommendation{
			Category:   CatPublishing,
			Title:      "Publish in the strongest window",
			Detail:     fmt.Sprintf("Schedule releases around %s ~%02d:00 UTC.", t.Weekday, t.Hour),
			Rationale:  "This window has the highest historical average views.",
			Evidence:   []string{fmt.Sprintf("%s %02d:00 avg %.0f views, %.2f%% engagement", t.Weekday, t.Hour, t.AvgViews, t.EngagementRate)},
			Confidence: supportConfidence(t.Publications, 0.5),
		})
	}

	// SEO + social from top tags/keywords.
	if len(input.TopKeywords) > 0 {
		k := input.TopKeywords[0]
		recs = append(recs, Recommendation{
			Category:   CatSEO,
			Title:      "Lead with the highest-reach keyword",
			Detail:     fmt.Sprintf("Feature %q in titles, headings and metadata.", k.Keyword),
			Rationale:  "It has the highest reach among tracked keywords.",
			Evidence:   []string{fmt.Sprintf("%q: %d publications, %d views, %.2f%% engagement", k.Keyword, k.Publications, k.Views, k.EngagementRate)},
			Confidence: supportConfidence(k.Publications, 0.5),
		})
	}
	if len(input.TopTags) > 0 {
		tg := input.TopTags[0]
		recs = append(recs, Recommendation{
			Category:   CatSocial,
			Title:      "Prioritize the strongest hashtag/topic",
			Detail:     fmt.Sprintf("Use #%s prominently in social posts.", tg.Tag),
			Rationale:  "It leads on reach among tracked tags.",
			Evidence:   []string{fmt.Sprintf("#%s: %d publications, %d views", tg.Tag, tg.Publications, tg.Views)},
			Confidence: supportConfidence(tg.Publications, 0.5),
		})
	}

	// Video / thumbnail from retention + CTR of the video cohort.
	recs = append(recs, e.videoRecs(input)...)

	// Platform recommendations from platform comparison.
	recs = append(recs, e.platformRecs(input)...)

	// Prompt improvement pointer (details come from PromptAnalyzer).
	if len(losing) > 0 {
		recs = append(recs, Recommendation{
			Category:   CatPrompt,
			Title:      "Refine generation prompts for weak hooks",
			Detail:     "Propose prompt revisions strengthening the opening hook and early retention.",
			Rationale:  "Underperforming content consistently shows weak hooks.",
			Evidence:   losing[0].Evidence,
			Confidence: clampFloat(losing[0].Confidence, 0, 0.9),
		})
	}

	finalize(recs)
	return recs
}

// videoRecs grounds video/thumbnail/narration guidance in the video cohort.
func (e *DefaultRecommendationEngine) videoRecs(input AnalyticsInput) []Recommendation {
	var video []PerformanceRecord
	for _, r := range input.Records {
		if r.WatchMinutes > 0 || r.RetentionPct > 0 {
			video = append(video, r)
		}
	}
	if len(video) < e.Config.minSupport() {
		return nil
	}
	ret := meanMetric(video, "retentionPct")
	ctr := meanMetric(video, "ctr")
	var out []Recommendation
	out = append(out, Recommendation{
		Category:   CatVideo,
		Title:      "Tighten early retention",
		Detail:     "Front-load value in the first 15 seconds to lift average retention.",
		Rationale:  "Average retention across the video cohort has headroom.",
		Evidence:   []string{fmt.Sprintf("mean retention %.1f%% over %d videos", ret, len(video))},
		Confidence: supportConfidence(len(video), 0.4),
	})
	out = append(out, Recommendation{
		Category:   CatThumbnail,
		Title:      "A/B test thumbnails to raise CTR",
		Detail:     "Test high-contrast thumbnails with a clear focal subject.",
		Rationale:  "Thumbnail CTR is the strongest lever on impressions→views.",
		Evidence:   []string{fmt.Sprintf("mean CTR %.2f%% over %d videos", ctr, len(video))},
		Confidence: supportConfidence(len(video), 0.35),
	})
	out = append(out, Recommendation{
		Category:   CatNarration,
		Title:      "Improve narration pacing",
		Detail:     "Vary pace and add pattern interrupts to sustain attention.",
		Rationale:  "Sustained attention correlates with watch time in the cohort.",
		Evidence:   []string{fmt.Sprintf("%d videos analyzed", len(video))},
		Confidence: supportConfidence(len(video), 0.3),
	})
	return out
}

// platformRecs grounds platform guidance in the platform comparison.
func (e *DefaultRecommendationEngine) platformRecs(input AnalyticsInput) []Recommendation {
	if len(input.Platforms) == 0 {
		return nil
	}
	plats := append([]PlatformStat(nil), input.Platforms...)
	sort.SliceStable(plats, func(i, j int) bool { return plats[i].EngagementRate > plats[j].EngagementRate })
	best := plats[0]
	out := []Recommendation{{
		Category:   CatPlatform,
		Title:      "Invest more in " + string(best.Platform),
		Detail:     fmt.Sprintf("%s has the highest engagement rate; allocate more cross-posts there.", best.Platform),
		Rationale:  "Highest engagement rate among tracked platforms.",
		Evidence:   []string{fmt.Sprintf("%s: %.2f%% engagement over %d publications", best.Platform, best.EngagementRate, best.Publications)},
		Confidence: supportConfidence(best.Publications, 0.5),
	}}
	if len(plats) > 1 {
		worst := plats[len(plats)-1]
		if worst.Views > 0 && worst.EngagementRate < best.EngagementRate {
			out = append(out, Recommendation{
				Category:   CatPlatform,
				Title:      "Reassess " + string(worst.Platform) + " strategy",
				Detail:     fmt.Sprintf("%s engagement lags; adapt format or cadence.", worst.Platform),
				Rationale:  "Lowest engagement rate among tracked platforms.",
				Evidence:   []string{fmt.Sprintf("%s: %.2f%% engagement over %d publications", worst.Platform, worst.EngagementRate, worst.Publications)},
				Confidence: supportConfidence(worst.Publications, 0.3),
			})
		}
	}
	return out
}

// finalize sorts by confidence, stamps ids, priorities, and impact labels.
func finalize(recs []Recommendation) {
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Confidence > recs[j].Confidence })
	for i := range recs {
		recs[i].Confidence = round2(recs[i].Confidence)
		recs[i].ExpectedImpact = expectedImpact(recs[i].Confidence)
		recs[i].Priority = i + 1
		recs[i].ID = shortID(string(recs[i].Category), recs[i].Title)
	}
}
