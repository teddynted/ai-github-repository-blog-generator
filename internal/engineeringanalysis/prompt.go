package engineeringanalysis

import (
	"fmt"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// jsonSchema documents the exact output contract for the model. It mirrors
// releasecontext.EngineeringContext so the response unmarshals cleanly.
const jsonSchema = `{
  "problem": "one paragraph: the engineering problem this release solves",
  "engineering_decisions": [{"decision": "what was done", "rationale": "why, concretely"}],
  "architectural_drivers": ["forces that shaped the design"],
  "tradeoffs": [{"choice": "what was chosen", "alternatives": ["rejected option"], "advantages": ["pro"], "disadvantages": ["con"]}],
  "aws_services": [{"name": "service", "purpose": "what it does here", "rationale": "why this over alternatives"}],
  "security": ["IAM / networking / encryption / least-privilege facts"],
  "scalability": {"current": "how it scales today", "future": "how it will scale"},
  "cost_optimizations": ["EC2 / Bedrock / Ollama / storage / networking cost choices"],
  "lessons_learned": ["engineering lessons this release surfaced"],
  "future_milestones": ["what this release makes possible next"],
  "implementation_notes": ["notable implementation details worth explaining"]
}`

func (a *Analyzer) buildPrompt(c *rc.ReleaseContext) string {
	max := a.MaxFactsBytes
	if max <= 0 {
		max = DefaultMaxFactsBytes
	}
	facts := safeTruncate(factsBlock(c), max)

	var b strings.Builder
	b.WriteString("You are a staff software engineer performing a structured engineering review of a software release.\n")
	b.WriteString("Extract the ENGINEERING REASONING behind the release: the problem solved, the decisions and why they were made, the trade-offs weighed, the AWS services chosen and why, security posture, scalability, cost optimisations, lessons, and what becomes possible next.\n\n")
	b.WriteString("STRICT RULES:\n")
	b.WriteString("- Ground every item ONLY in the facts below. Do not invent services, versions, or features not present.\n")
	b.WriteString("- If a section has no evidence, return an empty array (or empty string) for it. Never fabricate to fill it.\n")
	b.WriteString("- Do NOT write prose, an article, or release notes. Output the JSON object ONLY — no preamble, no code fence, no commentary.\n\n")
	b.WriteString("Output EXACTLY this JSON shape (fill the values, keep the keys):\n")
	b.WriteString(jsonSchema)
	b.WriteString("\n\n=== RELEASE FACTS ===\n")
	b.WriteString(facts)
	b.WriteString("\n=== END RELEASE FACTS ===\n\n")
	b.WriteString("Return the JSON object now.")
	return b.String()
}

// factsBlock renders the high-signal, factual parts of the ReleaseContext for
// the extraction prompt. It is intentionally distinct from the writer's
// grounding block: this feeds analysis, not prose.
func factsBlock(c *rc.ReleaseContext) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	line("Repository: %s", c.Repository.FullName)
	if c.Repository.Summary != "" {
		line("Summary: %s", c.Repository.Summary)
	}
	if c.Repository.Language != "" {
		line("Primary language: %s", c.Repository.Language)
	}
	line("Release: %s", c.Release.Tag)
	if c.Release.Name != "" {
		line("Release name: %s", c.Release.Name)
	}
	if body := strings.TrimSpace(c.Release.Body); body != "" {
		line("Release notes:\n%s", body)
	}

	appendList(&b, "Features", c.Changelog.Features)
	appendList(&b, "Improvements", c.Changelog.Improvements)
	appendList(&b, "Bug fixes", c.Changelog.BugFixes)
	appendList(&b, "Breaking changes", c.Changelog.BreakingChanges)

	if c.Implementation.WhyItMatters != "" {
		line("Why it matters: %s", c.Implementation.WhyItMatters)
	}
	appendList(&b, "What changed", c.Implementation.WhatChanged)
	appendList(&b, "Technical improvements", c.Implementation.TechnicalImprovements)
	appendList(&b, "Infrastructure improvements", c.Implementation.InfrastructureImprovements)

	if c.Architecture.Overview != "" {
		line("Architecture overview: %s", c.Architecture.Overview)
	}
	if len(c.Architecture.AWSServices) > 0 {
		line("AWS services detected: %s", strings.Join(c.Architecture.AWSServices, ", "))
	}
	appendList(&b, "Event-driven flows", c.Architecture.EventDrivenFlows)
	appendList(&b, "Architecture insights", c.Architecture.Insights)

	if names := technologyNames(c); len(names) > 0 {
		line("Technologies: %s", strings.Join(names, ", "))
	}
	if c.ContentIntelligence.Summary != "" {
		line("Content summary: %s", c.ContentIntelligence.Summary)
	}
	appendList(&b, "Technical highlights", c.ContentIntelligence.TechnicalHighlights)

	return strings.TrimSpace(b.String())
}

func appendList(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", heading)
	for _, it := range items {
		if s := strings.TrimSpace(it); s != "" {
			fmt.Fprintf(b, "  - %s\n", s)
		}
	}
}

func technologyNames(c *rc.ReleaseContext) []string {
	names := make([]string, 0, len(c.Technologies))
	for _, t := range c.Technologies {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	if len(names) > 20 {
		names = names[:20]
	}
	return names
}

func safeTruncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	const marker = "\n[facts truncated to fit the model budget]"
	if max <= len(marker) {
		return s[:max]
	}
	return s[:max-len(marker)] + marker
}
