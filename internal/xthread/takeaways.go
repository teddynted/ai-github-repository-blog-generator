package xthread

// planTakeaways extracts grounded key engineering takeaways for a thread type.
// Every takeaway comes from the Release Context — nothing is invented.
func planTakeaways(pkg ReleasePackage, threadType string) []string {
	c := pkg.Context
	if c == nil {
		return nil
	}
	var out []string
	switch threadType {
	case "Architecture Walkthrough", "AWS Best Practices":
		out = append(out, c.ContentIntelligence.ArchitectureHighlights...)
		out = append(out, c.ContentIntelligence.InfrastructureHighlights...)
	case "AI Engineering Insights":
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Implementation.TechnicalImprovements...)
	case "Performance Improvements":
		out = append(out, c.ContentIntelligence.InfrastructureHighlights...)
		out = append(out, c.Implementation.InfrastructureImprovements...)
	case "Engineering Lessons Learned":
		out = append(out, c.Implementation.TechnicalImprovements...)
		if c.Implementation.WhyItMatters != "" {
			out = append(out, firstSentences(c.Implementation.WhyItMatters, 1))
		}
	case "Implementation Deep Dive", "Feature Breakdown", "Developer Tips":
		out = append(out, c.Changelog.Features...)
		out = append(out, c.Implementation.DeveloperExperience...)
	default: // Release Announcement, Open Source Update, Technical Summary, Repository Highlights
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Changelog.Features...)
	}
	if len(out) == 0 {
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
		out = append(out, c.Changelog.Features...)
		out = append(out, c.Implementation.WhatChanged...)
	}
	return topStrings(dedupe(out), 5)
}
