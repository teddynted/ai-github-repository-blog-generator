package socialintel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// deterministicInsights derives grounded strengths/weaknesses/opportunities/
// risks straight from the metrics — no model, no fabrication.
func deterministicInsights(g InsightGrounding) ([]BusinessInsight, []Recommendation) {
	var insights []BusinessInsight
	var recs []Recommendation

	for _, gm := range g.Growth {
		if gm.Metric != "followers" {
			continue
		}
		switch {
		case gm.WeekOverWeek.Percent >= 5:
			insights = append(insights, BusinessInsight{Kind: "strength", Metric: string(gm.Platform) + " followers",
				Summary: fmt.Sprintf("%s follower growth is strong: %+.1f%% week-over-week (%s).", gm.Platform, gm.WeekOverWeek.Percent, gm.Momentum)})
		case gm.WeekOverWeek.Percent <= -2:
			insights = append(insights, BusinessInsight{Kind: "weakness", Metric: string(gm.Platform) + " followers",
				Summary: fmt.Sprintf("%s followers declined %.1f%% week-over-week; investigate content mix.", gm.Platform, gm.WeekOverWeek.Percent)})
			recs = append(recs, Recommendation{Category: "growth", Action: "Review recent " + string(gm.Platform) + " content for a drop-off cause",
				Rationale: "week-over-week follower change is negative", Evidence: []string{fmt.Sprintf("%s WoW %.1f%%", gm.Platform, gm.WeekOverWeek.Percent)}})
		}
		if gm.Momentum == "decelerating" && gm.WeekOverWeek.Percent > 0 {
			insights = append(insights, BusinessInsight{Kind: "risk", Metric: string(gm.Platform) + " momentum",
				Summary: fmt.Sprintf("%s is still growing but momentum is decelerating.", gm.Platform)})
		}
	}

	// Best content → replicate.
	if len(g.TopContent) > 0 {
		top := g.TopContent[0]
		insights = append(insights, BusinessInsight{Kind: "opportunity", Metric: "top content",
			Summary: fmt.Sprintf("Top content on %s scored %.0f/100 (engagement %.1f%%, virality %.0f).", top.Platform, top.Score, top.EngagementRate, top.ViralityScore)})
		recs = append(recs, Recommendation{Category: "content", Action: "Produce more content in the style of the top performer",
			Rationale: "it has the highest effectiveness score", Evidence: []string{fmt.Sprintf("%s score %.0f", top.Platform, top.Score)}})
	}

	// Cross-platform steer.
	if cp := g.CrossPlatform; cp.HighestEngagement != "" {
		recs = append(recs, Recommendation{Category: "posting", Action: "Prioritize " + string(cp.HighestEngagement) + " for engagement-driven content",
			Rationale: "it has the highest engagement rate", Evidence: []string{"highest engagement: " + string(cp.HighestEngagement)}})
	}
	if len(insights) == 0 {
		insights = append(insights, BusinessInsight{Kind: "strength", Summary: "Metrics are stable across platforms."})
	}
	return insights, recs
}

// ModelInsighter is an AIInsighter backed by releasegen.Model (Bedrock/Ollama).
// It is grounded: the prompt supplies the collected metrics and forbids inventing
// statistics.
type ModelInsighter struct {
	Model releasegen.Model
	Name  string
}

// NewModelInsighter wraps a Model as an AIInsighter.
func NewModelInsighter(m releasegen.Model, name string) *ModelInsighter {
	if name == "" {
		name = "model"
	}
	return &ModelInsighter{Model: m, Name: name}
}

// Insights asks the model for grounded qualitative notes and parses them. On any
// error or unparseable output it returns an error so the engine falls back to the
// deterministic insights — it never fabricates.
func (r *ModelInsighter) Insights(ctx context.Context, g InsightGrounding) (AINotes, error) {
	if r.Model == nil {
		return AINotes{}, ErrNoData
	}
	out, err := r.Model.Generate(ctx, insightPrompt(g))
	if err != nil {
		return AINotes{}, err
	}
	notes, err := parseNotes(out)
	if err != nil {
		return AINotes{}, err
	}
	notes.Reviewer = r.Name
	return notes, nil
}

func insightPrompt(g InsightGrounding) string {
	metrics, _ := json.Marshal(struct {
		Date          Date                   `json:"date"`
		Performance   []PlatformPerformance  `json:"performance"`
		Growth        []GrowthMetrics        `json:"growth"`
		TopContent    []ContentEffectiveness `json:"topContent"`
		CrossPlatform CrossPlatform          `json:"crossPlatform"`
	}{g.Date, g.Performance, g.Growth, topN(g.TopContent, 5), g.CrossPlatform})

	return fmt.Sprintf(
		"You are a social-media analytics advisor. Analyze ONLY the metrics JSON below (do NOT invent any numbers "+
			"or statistics that are not present) and return grounded insights.\n\n"+
			"Respond ONLY with minified JSON: {\"summary\":\"...\",\"insights\":[{\"kind\":\"strength|weakness|opportunity|risk|prediction\",\"summary\":\"...\",\"metric\":\"...\"}],"+
			"\"recommendations\":[{\"category\":\"content|posting|audience|growth|risk\",\"action\":\"...\",\"rationale\":\"...\",\"evidence\":[\"...\"]}]}. "+
			"Ground every statement in the metrics.\n\nMETRICS:\n%s", string(metrics))
}

func parseNotes(out string) (AINotes, error) {
	s := strings.TrimSpace(out)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return AINotes{}, ErrNoData
	}
	var n AINotes
	if err := json.Unmarshal([]byte(s[start:end+1]), &n); err != nil {
		return AINotes{}, ErrNoData
	}
	return n, nil
}
