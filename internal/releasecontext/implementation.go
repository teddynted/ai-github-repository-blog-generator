package releasecontext

import (
	"strings"
)

// buildImplementation derives the "what shipped and why it matters" summary from
// the changelog, commits, and architecture already on rc.
func buildImplementation(rc *ReleaseContext) ImplementationSummary {
	var impl ImplementationSummary

	// What changed: prefer the curated CHANGELOG; fall back to feature commits.
	if rc.Changelog.Found {
		impl.WhatChanged = topN(append(append([]string{}, rc.Changelog.Features...), rc.Changelog.Improvements...), 10)
	}
	if len(impl.WhatChanged) == 0 {
		impl.WhatChanged = topN(commitSubjects(rc, "Features"), 8)
	}

	impl.HowItWorks = rc.Architecture.Overview
	impl.TechnicalImprovements = topN(dedupeStrings(append(
		commitSubjects(rc, "Refactoring"), commitSubjects(rc, "Performance")...)), 6)
	if len(rc.Changelog.Improvements) > 0 {
		impl.TechnicalImprovements = topN(dedupeStrings(append(impl.TechnicalImprovements, rc.Changelog.Improvements...)), 6)
	}

	infra := commitSubjects(rc, "Infrastructure")
	if rc.CloudFormation.Counts.Resources > 0 {
		infra = append([]string{rc.CloudFormation.Summary}, infra...)
	}
	impl.InfrastructureImprovements = topN(dedupeStrings(infra), 6)

	impl.DeveloperExperience = topN(dedupeStrings(append(append(
		commitSubjects(rc, "CI/CD"), commitSubjects(rc, "Build")...), commitSubjects(rc, "Testing")...)), 6)
	impl.DocumentationImprovements = topN(dedupeStrings(commitSubjects(rc, "Documentation")), 6)

	impl.WhyItMatters = whyItMatters(rc)
	return impl
}

// commitSubjects returns the subjects of commits in a given category.
func commitSubjects(rc *ReleaseContext, category string) []string {
	var out []string
	for _, c := range rc.Commits {
		if c.Category == category {
			out = append(out, c.Subject)
		}
	}
	return out
}

// whyItMatters is EVERGREEN: it speaks to what the change does, never "Release
// <tag> … N features and M fixes" (the counts live in structured CommitStats).
func whyItMatters(rc *ReleaseContext) string {
	switch {
	case rc.CommitStats.Breaking > 0:
		return "This introduces breaking changes that reshape the platform's contract and warrant a migration guide."
	case rc.CommitStats.ByCategory["Features"] > 0:
		return "This advances the platform with new capability while keeping the existing contract stable."
	default:
		return "This hardens the platform with fixes and maintenance changes, improving reliability and the developer experience."
	}
}

// implementationHighlights flattens the summary into short bullet strings used
// by the content-intelligence layer.
func implementationHighlights(impl ImplementationSummary) []string {
	var hs []string
	hs = append(hs, impl.WhatChanged...)
	hs = append(hs, impl.TechnicalImprovements...)
	return topN(dedupeStrings(trimBullets(hs)), 8)
}

func trimBullets(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(stripMarkdown(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
