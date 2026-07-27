package releasegen

import (
	"fmt"
	"sort"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// writeArchitectureGraph renders the repository's most informative Mermaid
// diagrams as a plain component graph (named nodes + directed edges). It is the
// grounding that lets the writer describe the real architecture — the actual
// components and how they connect — rather than a generic "two components"
// summary. It selects the diagrams with the most edges (the richest ones) and
// caps volume so the grounding block stays compact.
func writeArchitectureGraph(b *strings.Builder, diagrams []rc.MermaidDiagram) {
	const maxGraphDiagrams, maxGraphEdges = 2, 24
	ranked := make([]rc.MermaidDiagram, 0, len(diagrams))
	for _, d := range diagrams {
		if len(d.Edges) > 0 {
			ranked = append(ranked, d)
		}
	}
	if len(ranked) == 0 {
		return
	}
	sort.SliceStable(ranked, func(i, j int) bool { return len(ranked[i].Edges) > len(ranked[j].Edges) })

	b.WriteString("\n## Architecture graph (components and relationships, from the repository's diagrams)\n")
	for i, d := range ranked {
		if i >= maxGraphDiagrams {
			break
		}
		src := d.Source
		if src == "" {
			src = "diagram"
		}
		if s := strings.TrimSpace(d.Summary); s != "" {
			fmt.Fprintf(b, "%s — %s\n", src, s)
		} else {
			fmt.Fprintf(b, "%s\n", src)
		}
		if len(d.Nodes) > 0 {
			fmt.Fprintf(b, "Components: %s\n", strings.Join(d.Nodes, ", "))
		}
		edges := d.Edges
		if len(edges) > maxGraphEdges {
			edges = edges[:maxGraphEdges]
		}
		for _, e := range edges {
			if e.Label != "" {
				fmt.Fprintf(b, "  %s -> %s [%s]\n", e.From, e.To, e.Label)
			} else {
				fmt.Fprintf(b, "  %s -> %s\n", e.From, e.To)
			}
		}
	}
}

// contextBlock renders the parts of a ReleaseContext most useful for grounding
// a content prompt into a compact, readable block. It deliberately favours the
// curated, high-signal fields (summaries, changelog, implementation,
// architecture, content intelligence) over raw data dumps.
func contextBlock(c *rc.ReleaseContext) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	// Lead with the structured engineering analysis (Stage 2) when present: it is
	// the reasoning distilled from the raw facts — decisions, trade-offs, service
	// choices — and is what turns a summary into an engineering narrative. The
	// factual context below remains, but the writer is steered by this first.
	if eb := c.Engineering.GroundingBlock(); eb != "" {
		b.WriteString(eb)
		b.WriteString("\n")
	}

	line("# Repository: %s", c.Repository.FullName)
	if c.Repository.Summary != "" {
		line("%s", c.Repository.Summary)
	}
	if c.Repository.Language != "" {
		line("Primary language: %s", c.Repository.Language)
	}
	if len(c.Repository.Topics) > 0 {
		line("Topics: %s", strings.Join(c.Repository.Topics, ", "))
	}

	line("\n## Release: %s", c.Release.Tag)
	if c.Release.Summary != "" {
		line("%s", c.Release.Summary)
	}
	if c.Release.PreviousTag != "" {
		line("Previous release: %s", c.Release.PreviousTag)
	}
	if body := strings.TrimSpace(c.Release.Body); body != "" {
		line("Release notes:\n%s", body)
	}

	writeList(&b, "## Features", c.Changelog.Features)
	writeList(&b, "## Improvements", c.Changelog.Improvements)
	writeList(&b, "## Bug Fixes", c.Changelog.BugFixes)
	writeList(&b, "## Breaking Changes", c.Changelog.BreakingChanges)

	line("\n## Change statistics")
	line("Analyzed commits: %d (%d features, %d fixes). Files changed: %d.",
		c.CommitStats.Analyzed, c.CommitStats.ByCategory["Features"],
		c.CommitStats.ByCategory["Bug Fixes"], c.FileStats.Total)
	if len(c.CommitStats.ByCategory) > 0 {
		line("By category: %s", renderCounts(c.CommitStats.ByCategory))
	}

	if c.Implementation.WhyItMatters != "" || len(c.Implementation.WhatChanged) > 0 {
		line("\n## Implementation")
		if c.Implementation.WhyItMatters != "" {
			line("Why it matters: %s", c.Implementation.WhyItMatters)
		}
		writeList(&b, "What changed:", c.Implementation.WhatChanged)
		writeList(&b, "Technical improvements:", c.Implementation.TechnicalImprovements)
		writeList(&b, "Infrastructure improvements:", c.Implementation.InfrastructureImprovements)
	}

	if c.Architecture.Overview != "" || len(c.Architecture.AWSServices) > 0 {
		line("\n## Architecture")
		if c.Architecture.Overview != "" {
			line("%s", c.Architecture.Overview)
		}
		if len(c.Architecture.AWSServices) > 0 {
			line("AWS services: %s", strings.Join(c.Architecture.AWSServices, ", "))
		}
		writeList(&b, "Event-driven flows:", c.Architecture.EventDrivenFlows)
		writeList(&b, "Insights:", c.Architecture.Insights)
	}

	// The concrete component graph from the repository's own diagrams — so the
	// writer names the ACTUAL components and relationships (routers, queues,
	// providers, services) instead of abstracting to "two components".
	writeArchitectureGraph(&b, c.Mermaid)

	if names := technologyNames(c); len(names) > 0 {
		line("\n## Technologies")
		line("%s", strings.Join(names, ", "))
	}

	ci := c.ContentIntelligence
	line("\n## Content intelligence")
	if ci.Summary != "" {
		line("Summary: %s", ci.Summary)
	}
	if ci.TargetAudience != "" {
		line("Target audience: %s", ci.TargetAudience)
	}
	if ci.ImplementationComplexity != "" {
		line("Complexity: %s", ci.ImplementationComplexity)
	}
	writeList(&b, "Technical highlights:", ci.TechnicalHighlights)
	writeList(&b, "Suggested article outline:", ci.ArticleOutline)
	writeList(&b, "Suggested blog titles:", ci.BlogTitles)
	if len(ci.SEOKeywords) > 0 {
		line("SEO keywords: %s", strings.Join(ci.SEOKeywords, ", "))
	}

	return strings.TrimSpace(b.String())
}

func writeList(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s\n", heading)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", strings.TrimSpace(it))
	}
}

func renderCounts(m map[string]int) string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%s %d", p.k, p.v))
	}
	return strings.Join(parts, ", ")
}

func technologyNames(c *rc.ReleaseContext) []string {
	names := make([]string, 0, len(c.Technologies))
	for _, t := range c.Technologies {
		names = append(names, t.Name)
	}
	if len(names) > 16 {
		names = names[:16]
	}
	return names
}
