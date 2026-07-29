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
	if rctx != nil {
		if rctx.Repository.Name != "" {
			repo = rctx.Repository.Name
		}
		version = firstNonEmpty(rctx.Release.Tag, rctx.Release.Name, "current")
	}
	theme := firstNonEmpty(strings.TrimSpace(blog.Title), col.ContentIntelligence.ArchitectureStyle)

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

	blogDiagrams := extractMermaidBlocks(blog.Markdown)

	// --- Affected AWS Components — scoped to the RELEASE via the blog: services in
	// the release's own diagrams, plus services the blog discusses more than once
	// (one-off mentions like a passing "Bedrock as fallback" are excluded). Falls
	// back to the context's services only when the blog names none. ---
	services := releaseServices(blog.Markdown, blogDiagrams)
	if len(services) == 0 {
		services = col.ContentIntelligence.CloudServices
	}
	if len(services) > 0 {
		b.WriteString("---\n\n## Affected AWS Components\n\n")
		for _, s := range services {
			b.WriteString("- " + describeComponent(s) + "\n")
		}
		b.WriteString("\n")
	}

	// --- Updated Architecture Flow — the RELEASE's own diagrams from the blog
	// (release-specific), not the baseline platform topology. Falls back to the
	// grounded context diagrams only when the blog contains none. ---
	if len(blogDiagrams) > 0 {
		b.WriteString("---\n\n## Updated Architecture Flow\n\n")
		for _, d := range blogDiagrams {
			fmt.Fprintf(&b, "```mermaid\n%s\n```\n\n", strings.TrimRight(d, "\n"))
		}
	} else if len(col.Diagrams) > 0 {
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

	// The remaining sections are derived from the release blog — NOT the context's
	// platform-wide Architecture fields, which describe the whole repository
	// (deployment inventory, baseline security, directory layout) and would leak
	// non-release information into a release-scoped artifact.
	prose := blogProse(blog.Markdown)

	// --- Operational Impact — release-specific operational consequences ---
	if op := sentencesMatching(prose, operationalKeywords, 4); len(op) > 0 {
		b.WriteString("---\n\n## Operational Impact\n\n")
		for _, s := range op {
			b.WriteString("- " + s + "\n")
		}
		b.WriteString("\n")
	}

	// --- Security Considerations — only security the release itself touches ---
	if sec := sentencesMatching(prose, securityKeywords, 3); len(sec) > 0 {
		b.WriteString("---\n\n## Security Considerations\n\n")
		for _, s := range sec {
			b.WriteString("- " + s + "\n")
		}
		b.WriteString("\n")
	}

	// --- Relationship to the Platform — a concise release-integration statement ---
	fmt.Fprintf(&b, "---\n\n## Relationship to the Platform\n\n%s\n\n", relationshipStatement(theme))

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

// operationalKeywords select release-specific operational sentences from the blog.
var operationalKeywords = []string{
	"startup", "latency", "boot", "provision", "scal", "deploy", "cost",
	"interrupt", "drain", "spot", "warm", "cold start", "throughput", "faster",
	"reduce",
}

// securityKeywords select release-specific security sentences from the blog —
// concrete security actions, not broad terms like "permission" that also match
// baseline platform descriptions.
var securityKeywords = []string{
	"credential", "secret", "least-privilege", "least privilege", "encrypt",
	"harden", "cleanup", "machine-id", "scrub", "sanitize", "revoke",
}

// baselineNoise flags sentences that describe the broader platform topology
// rather than the release, so they are excluded from the release-scoped sections.
var baselineNoise = []string{"claw", "n8n", "ollama", "bedrock"}

// blogProse returns the blog's plain prose: front matter, fenced code blocks,
// headings, and table rows removed, so sentence extraction sees only sentences.
func blogProse(md string) string {
	var out []string
	inFrontMatter, inFence := false, false
	for i, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if i == 0 && t == "---" {
			inFrontMatter = true
			continue
		}
		if inFrontMatter {
			if t == "---" {
				inFrontMatter = false
			}
			continue
		}
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, " ")
}

// splitSentences splits prose into sentences on terminal punctuation.
func splitSentences(text string) []string {
	text = strings.Join(strings.Fields(text), " ")
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		if c := text[i]; c == '.' || c == '!' || c == '?' {
			if i+1 >= len(text) || text[i+1] == ' ' {
				out = append(out, text[start:i+1])
				start = i + 1
			}
		}
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// sentencesMatching returns up to max distinct blog sentences that mention any of
// the keywords — the grounded, release-specific statements for a section.
func sentencesMatching(prose string, keywords []string, max int) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range splitSentences(prose) {
		s = strings.TrimSpace(strings.TrimLeft(s, "-*> "))
		if len(s) < 25 || len(s) > 300 || seen[s] {
			continue
		}
		ls := strings.ToLower(s)
		// Skip sentences that describe the broader platform (baseline components not
		// introduced by this release) — they are not release-specific impact.
		if containsAny(ls, baselineNoise...) {
			continue
		}
		for _, k := range keywords {
			if strings.Contains(ls, k) {
				seen[s] = true
				out = append(out, s)
				break
			}
		}
		if len(out) >= max {
			break
		}
	}
	return out
}

// relationshipStatement is a concise release-integration sentence built from the
// release theme — never the platform-wide overview or repository directory layout.
func relationshipStatement(theme string) string {
	short := strings.TrimSpace(theme)
	if i := strings.Index(short, ":"); i > 0 {
		short = strings.TrimSpace(short[:i])
	}
	if short == "" {
		short = "the changes described above"
	}
	return "This release delivers **" + short + "**. It integrates with the existing platform " +
		"architecture rather than redefining it, and affects only the components and flows described above."
}

// extractMermaidBlocks returns the raw contents of every ```mermaid fenced block
// in a Markdown document, in order — the release's own, GitHub-renderable diagrams.
func extractMermaidBlocks(md string) []string {
	var blocks []string
	var cur []string
	in := false
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case !in && strings.HasPrefix(t, "```mermaid"):
			in, cur = true, nil
		case in && strings.HasPrefix(t, "```"):
			in = false
			// Keep only diagrams with real content: the type declaration plus at
			// least one statement (skip degenerate empty blocks like "flowchart TD").
			nonBlank := 0
			for _, c := range cur {
				if strings.TrimSpace(c) != "" {
					nonBlank++
				}
			}
			if nonBlank >= 2 {
				blocks = append(blocks, strings.Join(cur, "\n"))
			}
		case in:
			cur = append(cur, ln)
		}
	}
	return blocks
}

// releaseServices returns the catalogued AWS services that are relevant to the
// release: those appearing in the release's own diagrams, plus those the blog
// mentions at least twice. Whole-word matching avoids false positives (e.g. "rds"
// inside "words"); the frequency floor drops one-off, non-focal mentions.
func releaseServices(blogMd string, diagrams []string) []string {
	diagLower := strings.ToLower(strings.Join(diagrams, "\n"))
	fullLower := strings.ToLower(blogMd)
	seen := map[string]bool{}
	var out []string
	for _, entry := range catalogueByKeyLen {
		if seen[entry.info.Canonical] {
			continue
		}
		if containsWholeWord(diagLower, entry.key) || countWholeWord(fullLower, entry.key) >= 2 {
			seen[entry.info.Canonical] = true
			out = append(out, entry.info.Canonical)
		}
	}
	return out
}

// isWordChar reports whether b is part of an identifier word (letters/digits).
func isWordChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// countWord counts whole-word (boundary-delimited) occurrences of word in text.
func countWholeWord(text, word string) int {
	if word == "" {
		return 0
	}
	n, idx := 0, 0
	for {
		i := strings.Index(text[idx:], word)
		if i < 0 {
			return n
		}
		i += idx
		before := i == 0 || !isWordChar(text[i-1])
		after := i+len(word) >= len(text) || !isWordChar(text[i+len(word)])
		if before && after {
			n++
		}
		idx = i + 1
	}
}

// containsWord reports whether text contains word as a whole word.
func containsWholeWord(text, word string) bool { return countWholeWord(text, word) > 0 }

// bareRepoName strips an owner prefix from "owner/name".
func bareRepoName(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
