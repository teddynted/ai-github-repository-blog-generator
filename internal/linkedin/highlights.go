package linkedin

// planHighlights extracts grounded technical highlights relevant to a post type.
// Every highlight comes from the Release Context — nothing is invented.
func planHighlights(pkg ReleasePackage, postType string) []string {
	c := pkg.Context
	if c == nil {
		return nil
	}
	var out []string

	switch postType {
	case "Architecture Deep Dive", "AWS Best Practice":
		out = append(out, c.ContentIntelligence.ArchitectureHighlights...)
		out = append(out, c.ContentIntelligence.InfrastructureHighlights...)
		if len(awsServices(pkg)) > 0 {
			out = append(out, "Uses "+joinAnd(topStrings(awsServices(pkg), 4)))
		}
	case "AI Engineering Highlight":
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Implementation.TechnicalImprovements...)
	case "Performance Improvement":
		out = append(out, c.ContentIntelligence.InfrastructureHighlights...)
		out = append(out, c.Implementation.InfrastructureImprovements...)
	case "Engineering Lesson":
		out = append(out, c.Implementation.TechnicalImprovements...)
		if c.Implementation.WhyItMatters != "" {
			out = append(out, firstSentences(c.Implementation.WhyItMatters, 1))
		}
	case "Feature Spotlight", "Developer Productivity Tip":
		out = append(out, c.Changelog.Features...)
		out = append(out, c.Implementation.DeveloperExperience...)
	default: // Release Announcement, Behind-the-Build, Technical Insight
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Changelog.Features...)
	}

	// Fall back to the broadest grounded set so a post is never highlight-less.
	if len(out) == 0 {
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Changelog.Features...)
		out = append(out, c.Implementation.WhatChanged...)
	}
	return topStrings(dedupe(out), 5)
}
