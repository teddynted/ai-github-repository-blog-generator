package architecture

import (
	"fmt"
	"strings"
)

// repoLevelNotes is the version-independence statement rendered under the title.
const repoLevelNotes = "diagrams are generated from the repository README, documented AWS " +
	"integrations, automation workflows, and the current project structure. The architecture " +
	"document is intentionally **version-independent** so it can be reused across releases, " +
	"branches, and generated documentation workflows."

// repoLevelGenerationContext is the closing statement.
const repoLevelGenerationContext = "This document is generated from the **current repository state**, " +
	"including the README, documentation, workflows, and project structure available at generation " +
	"time. No release-specific version information is embedded in the architecture document."

// RepoLevelMarkdown renders the collection as a repository-level, version-independent
// architecture document: a platform overview, up to three curated diagram sections
// (logical data flow, deployment topology, repository component structure), and an
// architecture-intelligence table — with no release tag, date, or changelog. Every
// section is grounded in the same diagrams as Markdown(); missing diagrams are
// skipped rather than fabricated.
func (col ArchitectureCollection) RepoLevelMarkdown() string {
	var b strings.Builder
	ci := col.ContentIntelligence

	style := firstNonEmpty(ci.ArchitectureStyle, "Application")
	// Repository-level title uses the bare repo name (no owner prefix), so it reads
	// the same whether the context came from a fixture (owner/name) or the tree.
	repo := col.Metadata.Repository
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repo = repo[i+1:]
	}
	fmt.Fprintf(&b, "# Architecture Diagrams: %s\n\n", repo)
	fmt.Fprintf(&b, "_Repository-level architecture overview · %s_\n\n", style)
	fmt.Fprintf(&b, "> **Notes:** %s\n\n", repoLevelNotes)

	if col.PlatformOverview != "" {
		fmt.Fprintf(&b, "---\n\n## Platform Overview\n\n%s\n\n", col.PlatformOverview)
	}

	used := map[int]bool{}
	deployment := col.pick(used, "High-Level Architecture", "Event-Driven Architecture")
	logical := col.pick(used, "Data Flow Diagram", "Event-Driven Architecture", "Sequence Diagram")
	component := col.pick(used, "Component Diagram")
	cicd := col.pick(used, "CI/CD Pipeline")

	logicalTitle := "Logical Architecture — Data Flow"
	if len(ci.LocalInference) > 0 && len(ci.CloudInference) > 0 {
		logicalTitle = "Logical Architecture — Hybrid AI Data Flow"
	}
	writeRepoSection(&b, deployment, "Deployment Architecture — AWS Integration", "Deployment architecture", "Deployment Diagram", "AWS Services Used")
	writeRepoSection(&b, logical, logicalTitle, "Logical architecture", "Data Flow Diagram", "Key Components")
	writeRepoSection(&b, component, "Repository Structure View", "Top-level package layout", "Component Diagram", "Repository Responsibilities")
	writeRepoSection(&b, cicd, "CI/CD & Infrastructure Automation", "Delivery and infrastructure automation", "Deployment Diagram", "Deployment Characteristics")

	writeRepoIntelligence(&b, ci)

	fmt.Fprintf(&b, "---\n\n## Generation Context\n\n%s\n", repoLevelGenerationContext)
	return b.String()
}

// pick returns the first unused diagram whose Type matches one of prefs (in order
// of preference), marking it used. Returns nil when none match.
func (col ArchitectureCollection) pick(used map[int]bool, prefs ...string) *Diagram {
	for _, want := range prefs {
		for i := range col.Diagrams {
			if used[i] {
				continue
			}
			if col.Diagrams[i].Type == want {
				used[i] = true
				return &col.Diagrams[i]
			}
		}
	}
	return nil
}

// writeRepoSection renders one curated diagram section with a fixed title, its
// Mermaid, and a component bullet list. It is a no-op when the diagram is absent.
func writeRepoSection(b *strings.Builder, d *Diagram, title, subtitle, typeLabel, componentsHeading string) {
	if d == nil {
		return
	}
	fmt.Fprintf(b, "---\n\n## %s\n\n_%s_\n\n", title, subtitle)
	fmt.Fprintf(b, "**Type:** %s · **Complexity:** %s\n\n", typeLabel, d.Metadata.Complexity)
	fmt.Fprintf(b, "```mermaid\n%s\n```\n\n", d.Mermaid)

	components := d.Nodes
	if len(components) == 0 {
		components = d.AWSServices
	}
	if len(components) > 0 {
		fmt.Fprintf(b, "### %s\n\n", componentsHeading)
		for _, c := range components {
			fmt.Fprintf(b, "- %s\n", c)
		}
		b.WriteString("\n")
	}
}

// writeRepoIntelligence renders the Architecture Intelligence table. Rows appear
// only when grounded — inference rows are omitted for non-AI repositories.
func writeRepoIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Architecture Intelligence\n\n")
	b.WriteString("| Attribute | Value |\n| --- | --- |\n")
	row := func(attr, val string) {
		if strings.TrimSpace(val) != "" {
			fmt.Fprintf(b, "| **%s** | %s |\n", attr, val)
		}
	}
	row("Architecture style", ci.ArchitectureStyle)
	row("Primary workflow", ci.PrimaryWorkflow)
	row("Infrastructure complexity", ci.InfrastructureComplexity)
	row("Local inference", strings.Join(ci.LocalInference, ", "))
	row("Cloud inference", strings.Join(ci.CloudInference, ", "))
	row("Operational model", ci.OperationalModel)
	// Grounded service-role rows — only when the service is actually present.
	row("Shared workspace", ifPresent(ci.CloudServices, "Amazon EFS"))
	row("Durable artifacts", ifPresent(ci.CloudServices, "Amazon S3"))
	row("Centralized observability", strings.Join(ci.ObservabilityComponents, ", "))
	row("Estimated reading time", ci.EstimatedReadingTime)
	b.WriteString("\n")
}

// ifPresent returns want if it is in the list, otherwise "" (so the caller omits
// the row rather than asserting a service that is not grounded).
func ifPresent(list []string, want string) string {
	for _, s := range list {
		if s == want {
			return want
		}
	}
	return ""
}
