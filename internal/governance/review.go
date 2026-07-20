package governance

import (
	"context"
	"time"
)

// ReviewEngine produces a QualityReport by combining the deterministic engines
// (validation, grounding, scoring — authoritative) with optional AI qualitative
// notes. The AI never changes the scores or decision; it only enriches the
// report. This keeps the decision reproducible and auditable.
type ReviewEngine struct {
	Validation ValidationEngine
	Grounding  GroundingEngine
	Scoring    ScoringEngine
	AI         AIReviewer // optional; nil → deterministic notes only
	Now        Clock
}

// NewReviewEngine builds a review engine with the given config and (optional) AI
// reviewer.
func NewReviewEngine(cfg Config, ai AIReviewer, now Clock) ReviewEngine {
	if now == nil {
		now = time.Now
	}
	return ReviewEngine{
		Scoring: ScoringEngine{Config: cfg},
		AI:      ai,
		Now:     now,
	}
}

// Review evaluates content and returns a complete QualityReport.
func (e ReviewEngine) Review(ctx context.Context, c Content) QualityReport {
	v := e.Validation.Validate(c)
	g := e.Grounding.Verify(c)
	scores, decision := e.Scoring.Score(c, v, g)

	report := QualityReport{
		Reviewer:   "deterministic",
		Scores:     scores,
		Decision:   decision,
		Confidence: confidenceFrom(v, g),
		Validation: v,
		Grounding:  g,
		CreatedAt:  e.Now(),
	}

	// Deterministic summary/strengths/weaknesses/suggestions.
	report.Summary = deterministicSummary(c, scores, decision)
	report.Strengths, report.Weaknesses = deterministicNotes(v, g, scores)
	report.Suggestions = deterministicSuggestions(v, g)

	// Optional AI enrichment — grounded, and never overriding the decision.
	if e.AI != nil {
		if notes, err := e.AI.Review(ctx, c); err == nil {
			if notes.Reviewer != "" {
				report.Reviewer = "deterministic + " + notes.Reviewer
			}
			if notes.Summary != "" {
				report.Summary = notes.Summary
			}
			report.Strengths = mergeNotes(report.Strengths, notes.Strengths)
			report.Weaknesses = mergeNotes(report.Weaknesses, notes.Weaknesses)
			report.Suggestions = mergeNotes(report.Suggestions, notes.Suggestions)
			if notes.Confidence > 0 {
				report.Confidence = (report.Confidence + clampInt(notes.Confidence, 0, 100)) / 2
			}
		}
	}
	return report
}

func confidenceFrom(v ValidationReport, g GroundingResult) int {
	c := 90
	c -= v.Errors * 15
	c -= v.Warnings * 3
	if !g.Verified {
		c -= 30
	}
	return clampInt(c, 0, 100)
}

func deterministicSummary(c Content, s Scores, d Decision) string {
	return "Automated review of " + string(c.Type) + ": overall quality " + itoa(s.Overall) +
		"/100, decision \"" + string(d) + "\"."
}

func deterministicNotes(v ValidationReport, g GroundingResult, s Scores) (strengths, weaknesses []string) {
	if v.Passed {
		strengths = append(strengths, "Passes all deterministic validation checks.")
	}
	if g.Verified {
		strengths = append(strengths, "Every factual claim is grounded in the Release Context.")
	}
	if s.EducationalValue >= 80 {
		strengths = append(strengths, "Strong educational framing.")
	}
	if v.Errors > 0 {
		weaknesses = append(weaknesses, itoa(v.Errors)+" validation error(s) must be fixed.")
	}
	if !g.Verified {
		weaknesses = append(weaknesses, itoa(len(g.UnverifiedClaims))+" claim(s) could not be grounded in the Release Context.")
	}
	if s.Readability < 70 {
		weaknesses = append(weaknesses, "Readability is low; sentences may be too long.")
	}
	return strengths, weaknesses
}

func deterministicSuggestions(v ValidationReport, g GroundingResult) []string {
	var out []string
	for _, i := range v.Issues {
		out = append(out, "["+string(i.Severity)+"] "+i.Message)
	}
	for _, claim := range g.UnverifiedClaims {
		out = append(out, "Ground or remove this claim: \""+truncate(claim, 100)+"\"")
	}
	return out
}

func mergeNotes(a, b []string) []string { return dedupe(append(append([]string{}, a...), b...)) }

func truncate(s string, max int) string {
	s = collapse(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
