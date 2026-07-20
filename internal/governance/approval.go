package governance

import "strings"

// QualityGate is one pre-approval verification.
type QualityGate struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// ApprovalEngine enforces role permissions and the pre-approval quality gates.
type ApprovalEngine struct {
	Config Config
}

// canApprove reports whether a reviewer's role permits approving.
func (ApprovalEngine) canApprove(r Reviewer) bool { return r.Role.can("approve") }

// QualityGates evaluates the pre-approval checklist against the latest report.
// Approval is blocked unless every gate passes.
func (e ApprovalEngine) QualityGates(state *WorkflowState) []QualityGate {
	rep := state.LatestReport
	c := state.Content
	gate := func(name string, ok bool, detail string) QualityGate {
		return QualityGate{Name: name, Passed: ok, Detail: detail}
	}
	if rep == nil {
		return []QualityGate{gate("AI review complete", false, "no review has been run")}
	}
	s := rep.Scores
	cfg := e.Config

	gates := []QualityGate{
		gate("Technical accuracy", s.TechnicalAccuracy >= cfg.MinDimensionScore, "score "+itoa(s.TechnicalAccuracy)),
		gate("Release alignment (grounding)", rep.Grounding.Verified, groundingDetail(rep.Grounding)),
		gate("Grammar", s.Grammar >= cfg.MinDimensionScore, "score "+itoa(s.Grammar)),
		gate("Formatting / markdown", rep.Validation.Passed, itoa(rep.Validation.Errors)+" validation error(s)"),
		gate("SEO", s.SEO >= cfg.MinDimensionScore, "score "+itoa(s.SEO)),
		gate("Metadata", metadataComplete(c), ""),
		gate("Architecture consistency", s.Architecture >= cfg.MinDimensionScore, "score "+itoa(s.Architecture)),
		gate("Diagrams", diagramsValid(rep), ""),
		gate("References & citations", noPlaceholderLinks(rep), ""),
		gate("CTA quality", hasCTA(c), ""),
		gate("Branding consistency", s.Consistency >= cfg.MinDimensionScore, "score "+itoa(s.Consistency)),
		gate("Overall quality", s.Overall >= cfg.MinOverallScore, "overall "+itoa(s.Overall)+" (min "+itoa(cfg.MinOverallScore)+")"),
	}
	return gates
}

// gatesPassed reports whether every gate passes.
func gatesPassed(gates []QualityGate) bool {
	for _, g := range gates {
		if !g.Passed {
			return false
		}
	}
	return true
}

func groundingDetail(g GroundingResult) string {
	if g.Verified {
		return itoa(g.GroundedClaims) + "/" + itoa(g.CheckedClaims) + " claims grounded"
	}
	return itoa(len(g.UnverifiedClaims)) + " ungrounded claim(s)"
}

func metadataComplete(c Content) bool {
	switch c.Type {
	case TypeSEO:
		return c.Metadata["metaDescription"] != "" && c.Metadata["slug"] != ""
	case TypeBlog:
		return c.Title != ""
	default:
		return true
	}
}

func diagramsValid(rep *QualityReport) bool {
	for _, i := range rep.Validation.Issues {
		if strings.HasPrefix(i.Code, "mermaid") {
			return false
		}
	}
	return true
}

func noPlaceholderLinks(rep *QualityReport) bool {
	for _, i := range rep.Validation.Issues {
		if i.Code == "placeholder-link" || i.Code == "empty-link" {
			return false
		}
	}
	return true
}

// hasCTA reports whether content that should drive action includes a CTA-like
// signal. Content types that don't need a CTA pass trivially.
func hasCTA(c Content) bool {
	switch c.Type {
	case TypeLinkedIn, TypeXThread, TypeYouTubeScript, TypeYouTubeShorts, TypeTikTok, TypeBlog:
		l := strings.ToLower(c.Body + " " + c.Metadata["cta"])
		for _, cue := range []string{"http", "read", "watch", "follow", "subscribe", "star", "explore", "link in", "repo", "github"} {
			if strings.Contains(l, cue) {
				return true
			}
		}
		return false
	default:
		return true
	}
}
