package youtube

import rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"

// planCallouts returns the educational asides for a chapter, chosen by scene
// type and grounded in the Release Context where possible (e.g. an Architecture
// Decision cites a real design decision from the docs). Deterministic.
func planCallouts(chapterType string, c *rc.ReleaseContext) []Callout {
	var out []Callout
	switch chapterType {
	case "architecture", "diagram":
		out = append(out, Callout{Kind: "Architecture Decision", Text: architectureDecision(c)})
		out = append(out, Callout{Kind: "Best Practice", Text: "Keep the diagram the single source of truth — the storyboard and this script both reference it rather than redrawing it."})
	case "cloudformation":
		out = append(out, Callout{Kind: "Best Practice", Text: "Define every resource as code so the environment is reproducible and reviewable."})
		out = append(out, Callout{Kind: "Warning", Text: "Watch IAM scope here — grant least privilege, not blanket access."})
	case "repository":
		out = append(out, Callout{Kind: "Tip", Text: "Follow the dependency direction: handlers depend on interfaces, never the other way around."})
		out = append(out, Callout{Kind: "Common Mistake", Text: "Don't leak infrastructure types into the domain — that's what the ports are for."})
	case "implementation":
		out = append(out, Callout{Kind: "Tip", Text: "Notice how the logic sits behind a small interface, which is what keeps it unit-testable."})
		if c != nil && len(c.Implementation.TechnicalImprovements) > 0 {
			out = append(out, Callout{Kind: "Lesson Learned", Text: c.Implementation.TechnicalImprovements[0]})
		}
	case "results":
		out = append(out, Callout{Kind: "Performance Note", Text: performanceNote(c)})
	case "lessons":
		out = append(out, Callout{Kind: "Lesson Learned", Text: lessonLearned(c)})
		out = append(out, Callout{Kind: "Best Practice", Text: "Ground every generated artifact in real analysis so the output stays accurate."})
	}
	// Drop any callout that ended up empty.
	filtered := out[:0]
	for _, co := range out {
		if collapse(co.Text) != "" {
			filtered = append(filtered, co)
		}
	}
	return filtered
}

func architectureDecision(c *rc.ReleaseContext) string {
	if c != nil {
		if d := c.Documentation.DesignDecisions; len(d) > 0 {
			return d[0]
		}
		if len(c.ContentIntelligence.ArchitectureHighlights) > 0 {
			return c.ContentIntelligence.ArchitectureHighlights[0]
		}
		if c.Architecture.Overview != "" {
			return firstSentences(c.Architecture.Overview, 1)
		}
	}
	return "The system is decomposed so each component has a single responsibility."
}

func performanceNote(c *rc.ReleaseContext) string {
	if c != nil && len(c.ContentIntelligence.InfrastructureHighlights) > 0 {
		return c.ContentIntelligence.InfrastructureHighlights[0]
	}
	return "Measure before and after — the win here is in reproducibility and correctness, not raw speed."
}

func lessonLearned(c *rc.ReleaseContext) string {
	if c != nil {
		if c.Implementation.WhyItMatters != "" {
			return firstSentences(c.Implementation.WhyItMatters, 1)
		}
		if c.ContentIntelligence.DeveloperValue != "" {
			return firstSentences(c.ContentIntelligence.DeveloperValue, 1)
		}
	}
	return "Small, well-bounded packages make each milestone easy to reason about and test."
}
