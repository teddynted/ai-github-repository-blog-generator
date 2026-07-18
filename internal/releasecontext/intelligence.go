package releasecontext

import (
	"fmt"
	"sort"
	"strings"
)

// buildContentIntelligence produces the AI-ready metadata layer from the fully
// assembled context. It is deterministic and heuristic — a strong, structured
// prompt substrate that a downstream LLM milestone refines, not a replacement
// for one.
func buildContentIntelligence(rc *ReleaseContext) ContentIntelligence {
	var ci ContentIntelligence
	feats := rc.CommitStats.ByCategory["Features"]
	fixes := rc.CommitStats.ByCategory["Bug Fixes"]

	ci.Summary = fmt.Sprintf(
		"%s release %s delivers %d analyzed changes (%d features, %d fixes) across %d files. %s",
		rc.Repository.Name, rc.Release.Tag, rc.CommitStats.Analyzed, feats, fixes, rc.FileStats.Total,
		firstSentence(rc.Implementation.WhyItMatters),
	)

	ci.TechnicalHighlights = implementationHighlights(rc.Implementation)
	if len(ci.TechnicalHighlights) == 0 {
		ci.TechnicalHighlights = topN(commitSubjects(rc, "Features"), 6)
	}
	ci.InfrastructureHighlights = infraHighlights(rc)
	ci.ArchitectureHighlights = dedupeStrings(append(rc.Architecture.Insights, docPatternsSentence(rc)))
	ci.ImplementationComplexity = complexity(rc)
	ci.DeveloperValue = developerValue(rc)
	ci.BusinessValue = businessValue(rc)
	ci.TargetAudience = targetAudience(rc)
	ci.SEOKeywords = seoKeywords(rc)
	ci.BlogTitles = blogTitles(rc)
	ci.ArticleOutline = articleOutline(rc)
	ci.LinkedInPost = linkedInPost(rc)
	ci.YouTubeShortsTopic = fmt.Sprintf("How we shipped %s: %s in 60 seconds", rc.Release.Tag, shortSubject(rc))
	ci.TikTokTopic = fmt.Sprintf("Dev log: what changed in %s %s", rc.Repository.Name, rc.Release.Tag)
	ci.DocumentationUpdates = documentationUpdates(rc)
	ci.FutureEnhancements = futureEnhancements(rc)
	return ci
}

func infraHighlights(rc *ReleaseContext) []string {
	var hs []string
	if rc.CloudFormation.Counts.Resources > 0 {
		hs = append(hs, rc.CloudFormation.Summary)
	}
	if len(rc.Architecture.AWSServices) > 0 {
		hs = append(hs, "AWS services in use: "+strings.Join(topN(rc.Architecture.AWSServices, 8), ", ")+".")
	}
	if rc.Architecture.DeploymentTopology != "" {
		hs = append(hs, rc.Architecture.DeploymentTopology)
	}
	return dedupeStrings(hs)
}

func docPatternsSentence(rc *ReleaseContext) string {
	if len(rc.Documentation.ArchitecturePatterns) == 0 {
		return ""
	}
	return "Architectural patterns: " + strings.Join(rc.Documentation.ArchitecturePatterns, ", ") + "."
}

func complexity(rc *ReleaseContext) string {
	score := rc.CommitStats.Analyzed + rc.FileStats.Total/2
	if rc.CommitStats.Breaking > 0 {
		score += 20
	}
	switch {
	case score >= 40:
		return "high"
	case score >= 15:
		return "medium"
	default:
		return "low"
	}
}

func developerValue(rc *ReleaseContext) string {
	var parts []string
	if len(rc.Implementation.DeveloperExperience) > 0 {
		parts = append(parts, "improved developer experience (CI, tests, tooling)")
	}
	if rc.CommitStats.ByCategory["Bug Fixes"] > 0 {
		parts = append(parts, "fewer defects")
	}
	if rc.CommitStats.ByCategory["Documentation"] > 0 {
		parts = append(parts, "better documentation")
	}
	if len(parts) == 0 {
		return "Incremental hardening of the codebase."
	}
	return "Developers benefit from " + strings.Join(parts, ", ") + "."
}

func businessValue(rc *ReleaseContext) string {
	if rc.CommitStats.ByCategory["Features"] > 0 {
		return "New capabilities expand what the platform can deliver, supporting more content formats and use cases with predictable, bounded cost."
	}
	return "A more reliable, maintainable platform reduces operational risk and total cost of ownership."
}

func targetAudience(rc *ReleaseContext) string {
	if len(rc.Architecture.AWSServices) > 0 {
		return "Software engineers, cloud/DevOps engineers, and technical leaders building AWS-native, event-driven systems."
	}
	return "Software engineers and technical leaders."
}

func seoKeywords(rc *ReleaseContext) []string {
	var kw []string
	kw = append(kw, splitName(rc.Repository.Name)...)
	kw = append(kw, rc.Repository.Topics...)
	for _, t := range rc.Technologies {
		kw = append(kw, t.Name)
	}
	kw = append(kw, rc.Documentation.ArchitecturePatterns...)
	kw = append(kw, "release notes", "changelog", "software architecture")
	lowered := make([]string, 0, len(kw))
	for _, k := range kw {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			lowered = append(lowered, k)
		}
	}
	sort.Strings(lowered)
	return topN(dedupeStrings(lowered), 20)
}

func blogTitles(rc *ReleaseContext) []string {
	name := rc.Repository.Name
	tag := rc.Release.Tag
	titles := []string{
		fmt.Sprintf("Inside %s %s: What Changed and Why It Matters", name, tag),
		fmt.Sprintf("Shipping %s: An Event-Driven, AWS-Native Release Walkthrough", tag),
		fmt.Sprintf("From Commits to Content: How %s Turns a Release into Insight", name),
	}
	if rc.CommitStats.Breaking > 0 {
		titles = append(titles, fmt.Sprintf("Migrating to %s: Breaking Changes, Explained", tag))
	}
	return titles
}

func articleOutline(rc *ReleaseContext) []string {
	outline := []string{
		"Introduction — what this release is about",
		"Context — the problem and the architecture it fits into",
		"What changed — features, fixes, and improvements",
	}
	if rc.CloudFormation.Counts.Resources > 0 {
		outline = append(outline, "Infrastructure — the CloudFormation topology and AWS services")
	}
	if len(rc.Mermaid) > 0 {
		outline = append(outline, "How it works — architecture diagrams and data flow")
	}
	outline = append(outline,
		"Implementation highlights — notable engineering decisions",
		"Impact — developer and business value",
		"What's next — future enhancements",
	)
	return outline
}

func linkedInPost(rc *ReleaseContext) string {
	feats := rc.CommitStats.ByCategory["Features"]
	return fmt.Sprintf(
		"🚀 %s %s is out.\n\n%s\n\n%d features, %d fixes, %d files touched. Built event-driven on AWS with Infrastructure as Code.\n\n#golang #aws #serverless #devops #opensource",
		rc.Repository.Name, rc.Release.Tag, firstSentence(rc.Implementation.WhyItMatters),
		feats, rc.CommitStats.ByCategory["Bug Fixes"], rc.FileStats.Total,
	)
}

func documentationUpdates(rc *ReleaseContext) []string {
	var out []string
	if rc.CommitStats.Breaking > 0 {
		out = append(out, "Add a migration guide for the breaking changes in this release.")
	}
	if rc.CommitStats.ByCategory["Features"] > 0 && rc.CommitStats.ByCategory["Documentation"] == 0 {
		out = append(out, "Document the new features shipped this release (no docs commits detected).")
	}
	if len(rc.Mermaid) == 0 && rc.CloudFormation.Counts.Resources > 0 {
		out = append(out, "Add an architecture diagram — infrastructure exists but no Mermaid diagrams were found.")
	}
	return out
}

func futureEnhancements(rc *ReleaseContext) []string {
	return []string{
		"Generate the technical blog post from this Release Context (next milestone).",
		"Integrate Amazon Bedrock to refine the heuristic content intelligence.",
		"Persist each Release Context and diff successive releases for trend analysis.",
	}
}

func shortSubject(rc *ReleaseContext) string {
	if len(rc.Implementation.WhatChanged) > 0 {
		return firstSentence(rc.Implementation.WhatChanged[0])
	}
	return rc.Repository.Name
}

func splitName(name string) []string {
	fields := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	return fields
}
