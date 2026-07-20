package governance

import "strings"

// ScoringEngine computes the deterministic, authoritative quality scores and the
// resulting decision from validation, grounding, and content heuristics.
type ScoringEngine struct {
	Config Config
}

// Score computes the eight quality dimensions and an overall score, then derives
// the decision. Deterministic — the same inputs always produce the same result.
func (e ScoringEngine) Score(c Content, v ValidationReport, g GroundingResult) (Scores, Decision) {
	s := Scores{
		Completeness:      completenessScore(c, v),
		Grammar:           grammarScore(v),
		TechnicalAccuracy: groundingScore(g),
		Architecture:      architectureScore(c, v),
		SEO:               seoScore(c),
		Readability:       readabilityScore(c),
		Consistency:       consistencyScore(c, v),
		EducationalValue:  educationalScore(c),
	}
	s.Overall = overall(s)
	return s, e.decide(s, v, g)
}

// decide maps scores + gates to Approve / Needs Revision / Reject.
func (e ScoringEngine) decide(s Scores, v ValidationReport, g GroundingResult) Decision {
	cfg := e.Config
	// Hard rejects: severe validation failure or ungrounded content.
	if !g.Verified {
		return DecisionNeedsRevision // ungrounded → revise (never approve hallucinations)
	}
	if s.Overall < cfg.RejectBelow {
		return DecisionReject
	}
	if cfg.BlockOnValidationErrors && v.Errors > 0 {
		return DecisionNeedsRevision
	}
	// Any single dimension below the floor forces a revision.
	if minDimension(s) < cfg.MinDimensionScore {
		return DecisionNeedsRevision
	}
	if s.Overall >= cfg.MinOverallScore {
		return DecisionApprove
	}
	return DecisionNeedsRevision
}

// --- dimension scores (0–100) ---

func completenessScore(c Content, v ValidationReport) int {
	score := 100
	for _, i := range v.Issues {
		switch i.Code {
		case "empty":
			return 0
		case "too-short":
			score -= 30
		case "no-title":
			score -= 10
		case "placeholder":
			score -= 40
		}
	}
	if wordCount(c.Body) >= 150 {
		score += 0
	} else if wordCount(c.Body) < 40 {
		score -= 15
	}
	return clampInt(score, 0, 100)
}

func grammarScore(v ValidationReport) int {
	score := 100
	for _, i := range v.Issues {
		switch i.Code {
		case "double-space", "repeated-word":
			score -= 10
		}
	}
	return clampInt(score, 0, 100)
}

func groundingScore(g GroundingResult) int {
	if !g.Verified {
		// Partial credit proportional to grounded ratio, but capped low.
		if g.CheckedClaims == 0 {
			return 40 // nothing to verify against
		}
		return clampInt(50*g.GroundedClaims/g.CheckedClaims, 0, 55)
	}
	if g.CheckedClaims == 0 {
		return 85
	}
	return 100
}

func architectureScore(c Content, v ValidationReport) int {
	score := 90
	for _, i := range v.Issues {
		if strings.HasPrefix(i.Code, "mermaid") || i.Code == "code-fence" {
			score -= 20
		}
	}
	if c.Type == TypeArchitectureDiagram && v.Errors == 0 {
		score = 100
	}
	return clampInt(score, 0, 100)
}

func seoScore(c Content) int {
	if c.Type != TypeSEO && c.Type != TypeBlog {
		return 80 // not primarily an SEO surface
	}
	score := 60
	if c.Metadata["metaDescription"] != "" {
		score += 20
	}
	if c.Metadata["slug"] != "" {
		score += 10
	}
	if c.Metadata["keywords"] != "" || c.Metadata["seoKeywords"] != "" {
		score += 10
	}
	return clampInt(score, 0, 100)
}

func readabilityScore(c Content) int {
	sents := sentences(c.Body)
	if len(sents) == 0 {
		return 50
	}
	words := wordCount(c.Body)
	avg := words / len(sents)
	switch {
	case avg <= 22:
		return 95
	case avg <= 30:
		return 80
	case avg <= 40:
		return 65
	default:
		return 50
	}
}

func consistencyScore(c Content, v ValidationReport) int {
	score := 95
	if hasDuplicateSentences(c.Body) {
		score -= 25
	}
	for _, i := range v.Issues {
		if i.Code == "repeated-word" {
			score -= 5
		}
	}
	return clampInt(score, 0, 100)
}

func educationalScore(c Content) int {
	l := strings.ToLower(c.Body)
	score := 60
	for _, cue := range []string{"because", "how", "why", "so that", "for example", "takeaway", "note that", "this means"} {
		if strings.Contains(l, cue) {
			score += 6
		}
	}
	return clampInt(score, 0, 100)
}

// overall is a weighted average favouring accuracy, grounding, and completeness.
func overall(s Scores) int {
	weighted := s.TechnicalAccuracy*25 +
		s.Completeness*15 +
		s.Consistency*10 +
		s.Readability*10 +
		s.Grammar*10 +
		s.Architecture*10 +
		s.EducationalValue*10 +
		s.SEO*10
	return clampInt(weighted/100, 0, 100)
}

func minDimension(s Scores) int {
	vals := []int{s.TechnicalAccuracy, s.Readability, s.SEO, s.Architecture, s.Completeness, s.Grammar, s.Consistency, s.EducationalValue}
	m := 100
	for _, v := range vals {
		if v < m {
			m = v
		}
	}
	return m
}

func hasDuplicateSentences(body string) bool {
	seen := map[string]bool{}
	for _, s := range sentences(body) {
		key := strings.ToLower(collapse(s))
		if len(key) < 25 {
			continue // ignore short lines (CTAs, labels)
		}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}
