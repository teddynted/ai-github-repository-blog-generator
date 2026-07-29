package architecture

import (
	"fmt"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ReleaseScopedMarkdown renders the architecture artifact as a RELEASE-SCOPED
// impact analysis derived from the release's blog — not a repository-wide,
// version-independent reference. It is the rendering used by the per-release
// pipeline (local and cloud): the title carries the release version, the theme
// comes from the blog, and every section is scoped to what the release documents.
// The structured, grounded diagrams are preserved so the downstream
// architecture-diagram-spec stage can still consume it.
func ReleaseScopedMarkdown(col ArchitectureCollection, rctx *rc.ReleaseContext, blog releasegen.BlogPost) string {
	var b strings.Builder

	repo := bareRepoName(col.Metadata.Repository)
	version := "current"
	theme := ""
	var arch rc.Architecture
	if rctx != nil {
		if rctx.Repository.Name != "" {
			repo = rctx.Repository.Name
		}
		version = firstNonEmpty(rctx.Release.Tag, rctx.Release.Name, "current")
		arch = rctx.Architecture
	}
	theme = firstNonEmpty(strings.TrimSpace(blog.Title), col.ContentIntelligence.ArchitectureStyle)

	fmt.Fprintf(&b, "# Release Architecture: %s %s\n\n", repo, version)
	b.WriteString("_Architecture impact analysis derived from the release blog_\n\n")
	fmt.Fprintf(&b, "> **Source:** This document is generated from the release-specific `blog.md` artifact and captures the architectural impact of **%s**.\n\n", version)

	// --- Release Context ---
	b.WriteString("---\n\n## Release Context\n\n")
	fmt.Fprintf(&b, "- **Repository:** %s\n", repo)
	fmt.Fprintf(&b, "- **Release:** %s\n", version)
	if theme != "" {
		fmt.Fprintf(&b, "- **Primary Engineering Theme:** %s\n", theme)
	}
	b.WriteString("\n")

	// --- What Changed in This Release ---
	if changed := whatChanged(rctx, blog); changed != "" {
		fmt.Fprintf(&b, "---\n\n## What Changed in This Release\n\n%s\n\n", changed)
	}

	// --- Affected AWS Components (scoped to the release's grounded services) ---
	if services := col.ContentIntelligence.CloudServices; len(services) > 0 {
		b.WriteString("---\n\n## Affected AWS Components\n\n")
		for _, s := range services {
			b.WriteString("- " + describeComponent(s) + "\n")
		}
		b.WriteString("\n")
	}

	// --- Updated Architecture Flow (the grounded diagrams) ---
	if len(col.Diagrams) > 0 {
		b.WriteString("---\n\n## Updated Architecture Flow\n\n")
		used := map[int]bool{}
		flow := col.pick(used, "Data Flow Diagram", "Event-Driven Architecture", "High-Level Architecture", "Sequence Diagram")
		deploy := col.pick(used, "High-Level Architecture", "CI/CD Pipeline", "Event-Driven Architecture")
		for _, d := range []*Diagram{flow, deploy} {
			if d == nil {
				continue
			}
			fmt.Fprintf(&b, "**%s**\n\n```mermaid\n%s\n```\n\n", d.Title, d.Mermaid)
		}
	}

	// --- Operational Impact (grounded scalability/reliability/deployment) ---
	if op := bulletsFrom("", firstNonEmpty(arch.DeploymentTopology), firstNonEmpty(arch.Scalability), firstNonEmpty(arch.Reliability)); op != "" {
		fmt.Fprintf(&b, "---\n\n## Operational Impact\n\n%s\n", op)
	}

	// --- Security Considerations (grounded) ---
	if s := strings.TrimSpace(arch.Security); s != "" {
		fmt.Fprintf(&b, "---\n\n## Security Considerations\n\n%s\n\n", s)
	}

	// --- Relationship to the Platform ---
	if rel := sanitizeOverview(strings.TrimSpace(arch.Overview)); rel != "" {
		fmt.Fprintf(&b, "---\n\n## Relationship to the Platform\n\n%s\n\n", rel)
	}

	// --- Generation Context ---
	fmt.Fprintf(&b, "---\n\n## Generation Context\n\nThis document is generated from the release-specific `blog.md` artifact for **%s** and represents the architectural impact of that release rather than a permanent repository-wide architecture reference.\n", version)

	return b.String()
}

// whatChanged summarizes the release's changes from grounded sources: the release
// notes body, then the architecture insights, then the blog description.
func whatChanged(rctx *rc.ReleaseContext, blog releasegen.BlogPost) string {
	if rctx != nil {
		if body := strings.TrimSpace(rctx.Release.Body); body != "" {
			return firstSentences(body, 4)
		}
		if ins := rctx.Architecture.Insights; len(ins) > 0 {
			var out []string
			for _, i := range ins {
				out = append(out, "- "+strings.TrimSpace(i))
			}
			return strings.Join(out, "\n")
		}
	}
	if d := strings.TrimSpace(blog.MetaDescription); d != "" {
		return d
	}
	return ""
}

// bulletsFrom returns a bullet list of the non-empty sentences, or "".
func bulletsFrom(_ string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, "- "+p)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// bareRepoName strips an owner prefix from "owner/name".
func bareRepoName(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
