package contentoptimizer

import (
	"fmt"
	"strings"
)

// executiveSummary renders a grounded one-paragraph summary of the run.
func executiveSummary(r OptimizationReport, input AnalyticsInput) string {
	topRec := "n/a"
	if len(r.Recommendations) > 0 {
		topRec = r.Recommendations[0].Title
	}
	return fmt.Sprintf(
		"Analyzed %d publications across %d platforms as of %s. Detected %d winning and %d losing patterns and generated %d grounded recommendations (overall confidence %.2f). Top priority: %q.",
		len(input.Records), len(input.Platforms), input.AsOf,
		len(r.WinningPatterns), len(r.LosingPatterns), len(r.Recommendations), r.Confidence, topRec)
}

// priorityActions surfaces the highest-confidence recommendations as actions.
func priorityActions(recs []Recommendation, n int) []string {
	var out []string
	for _, rec := range topN(recs, n) {
		out = append(out, fmt.Sprintf("[%s] %s (confidence %.2f, %s impact)", rec.Category, rec.Title, rec.Confidence, rec.ExpectedImpact))
	}
	return out
}

// futureOpportunities derives forward-looking opportunities from emerging topics.
func futureOpportunities(trends []TrendReport) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range trends {
		for _, e := range t.Emerging {
			if seen[e.Topic] {
				continue
			}
			seen[e.Topic] = true
			out = append(out, fmt.Sprintf("Emerging topic %q (%+.1f%% %s) — invest early.", e.Topic, e.ChangePct, t.Horizon))
		}
	}
	if len(out) == 0 {
		out = append(out, "No emerging topics detected yet; keep collecting analytics to surface early signals.")
	}
	return out
}

// overallConfidence blends pattern confidences with the reasoning confidence.
func overallConfidence(winning, losing []Pattern, reasoning Reasoning) float64 {
	var xs []float64
	for _, p := range append(append([]Pattern{}, winning...), losing...) {
		xs = append(xs, p.Confidence)
	}
	if reasoning.Confidence > 0 {
		xs = append(xs, reasoning.Confidence)
	}
	if len(xs) == 0 {
		return 0
	}
	return round2(mean(xs))
}

// dataQuality reports how much analytics grounded the run.
func dataQuality(input AnalyticsInput) DataQuality {
	q := DataQuality{
		Records:       len(input.Records),
		Platforms:     len(input.Platforms),
		SeriesMetrics: len(input.Series),
	}
	// Sufficiency: more records + platforms + series raise confidence in the run.
	recF := clampFloat(float64(q.Records)/10.0, 0, 1)
	platF := clampFloat(float64(q.Platforms)/3.0, 0, 1)
	serF := clampFloat(float64(q.SeriesMetrics)/2.0, 0, 1)
	q.Sufficiency = round2(clampFloat((recF*0.6+platF*0.25+serF*0.15)*100, 0, 100))
	switch {
	case q.Sufficiency >= 75:
		q.CompletenessNote = "Strong sample — recommendations are well grounded."
	case q.Sufficiency >= 40:
		q.CompletenessNote = "Moderate sample — treat lower-confidence items as directional."
	default:
		q.CompletenessNote = "Thin sample — collect more analytics before acting on low-confidence items."
	}
	return q
}

// Markdown renders an OptimizationReport as a production-ready document.
func (r OptimizationReport) Markdown() string {
	var b sb
	b.line("# Content Optimization Report — " + r.AsOf)
	b.line("")
	b.line(r.ExecutiveSummary)
	b.line("")
	b.linef("**Overall confidence:** %.2f · **Run:** `%s`", r.Confidence, r.RunID)
	b.line("")

	if len(r.WinningPatterns) > 0 {
		b.line("## Winning Patterns")
		b.line("")
		for _, p := range r.WinningPatterns {
			b.linef("- **%s** (support %d, confidence %.2f) — %s", p.Name, p.Support, p.Confidence, joinArrow(p.SignalChain))
		}
		b.line("")
	}
	if len(r.LosingPatterns) > 0 {
		b.line("## Losing Patterns")
		b.line("")
		for _, p := range r.LosingPatterns {
			b.linef("- **%s** (support %d, confidence %.2f) — %s", p.Name, p.Support, p.Confidence, joinArrow(p.SignalChain))
		}
		b.line("")
	}

	if len(r.Recommendations) > 0 {
		b.line("## Recommendations")
		b.line("")
		b.line("| # | Category | Recommendation | Confidence | Impact |")
		b.line("|---|---|---|---|---|")
		for _, rec := range r.Recommendations {
			b.linef("| %d | %s | %s | %.2f | %s |", rec.Priority, rec.Category, rec.Title, rec.Confidence, rec.ExpectedImpact)
		}
		b.line("")
	}

	if len(r.PromptAnalyses) > 0 {
		b.line("## Prompt Analysis")
		b.line("")
		for _, a := range r.PromptAnalyses {
			b.linef("- **%s** — v%d (best v%d, %s); %d proposal(s) pending approval", a.TemplateID, a.CurrentVersion, a.BestVersion, a.Trend, len(a.Recommendations))
		}
		b.line("")
	}

	b.line("## Reasoning")
	b.line("")
	for _, w := range r.Reasoning.WhyWell {
		b.linef("- 👍 %s", w)
	}
	for _, u := range r.Reasoning.WhyUnder {
		b.linef("- 👎 %s", u)
	}
	b.linef("- _Reviewer: %s · confidence %.2f · %s impact_", r.Reasoning.Reviewer, r.Reasoning.Confidence, r.Reasoning.ExpectedImpact)
	b.line("")

	if len(r.PriorityActions) > 0 {
		b.line("## Priority Improvements")
		b.line("")
		for _, a := range r.PriorityActions {
			b.linef("- %s", a)
		}
		b.line("")
	}
	if len(r.FutureOpps) > 0 {
		b.line("## Future Opportunities")
		b.line("")
		for _, f := range r.FutureOpps {
			b.linef("- %s", f)
		}
		b.line("")
	}

	b.line("## Data Quality")
	b.line("")
	b.linef("- Records: **%d** · Platforms: **%d** · Series metrics: **%d** · Sufficiency: **%.1f%%**", r.DataQuality.Records, r.DataQuality.Platforms, r.DataQuality.SeriesMetrics, r.DataQuality.Sufficiency)
	b.linef("- %s", r.DataQuality.CompletenessNote)
	return b.String()
}

func joinArrow(ss []string) string { return strings.Join(ss, " → ") }
