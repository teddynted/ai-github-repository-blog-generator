package releasegen

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// BlogPost is a long-form technical article with SEO front matter.
type BlogPost struct {
	Title           string   `json:"title"`
	MetaDescription string   `json:"metaDescription"`
	Tags            []string `json:"tags"`
	// Markdown is the complete, publication-ready document: YAML front matter,
	// an H1 title, the article body, and any architecture diagrams.
	Markdown  string `json:"markdown"`
	WordCount int    `json:"wordCount"`
}

// Blog generates a long-form technical blog post from the Release Context.
//
// It is a hybrid of deterministic assembly and LLM prose so the result is both
// accurate and reliable: the SEO front matter (title, meta description, tags)
// and any Mermaid diagrams are assembled from the context directly — never
// hallucinated — while the long-form narrative is produced by the Model from a
// section-structured, strictly-grounded prompt.
func (g *Generator) Blog(ctx context.Context, rctx *rc.ReleaseContext) (BlogPost, error) {
	if g.Model == nil {
		return BlogPost{}, fmt.Errorf("generator has no model configured")
	}
	title := blogTitle(rctx)
	meta := metaDescription(rctx)
	tags := blogTags(rctx)

	body, err := g.Model.Generate(ctx, g.blogPrompt(rctx, title))
	if err != nil {
		return BlogPost{}, fmt.Errorf("generate blog: %w", err)
	}
	md := assembleBlog(title, meta, tags, strings.TrimSpace(body), rctx)

	if g.Logger != nil {
		g.logBlog(rctx, len(md))
	}
	return BlogPost{Title: title, MetaDescription: meta, Tags: tags, Markdown: md, WordCount: wordCount(md)}, nil
}

// blogPrompt builds the section-structured, strictly-grounded prompt. Front
// matter and diagrams are added deterministically afterwards, so the model is
// told to produce only the article body (from Introduction onward).
func (g *Generator) blogPrompt(rctx *rc.ReleaseContext, title string) string {
	max := g.MaxPromptBytes
	if max <= 0 {
		max = DefaultMaxPromptBytes
	}
	ground := safeTruncate(contextBlock(rctx), max)

	var b strings.Builder
	b.WriteString("You are a senior software engineer and technical writer producing a long-form engineering blog post.\n\n")
	b.WriteString("Write a professional, educational, technically accurate article of roughly 1,500–2,500 words in GitHub-flavoured Markdown. ")
	b.WriteString("Engaging for developers, cloud engineers, and AI practitioners; avoid promotional language.\n\n")
	b.WriteString("The working title is: ")
	b.WriteString(title)
	b.WriteString("\n\nUse these second-level (##) sections, in order, including only those supported by the context:\n")
	for _, s := range blogSections {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	b.WriteString("\nRules:\n")
	b.WriteString("- Ground every statement strictly in the Release Context below. Do NOT invent facts, versions, features, code, or AWS resources.\n")
	b.WriteString("- If a detail is missing from the context, say it was not available rather than guessing.\n")
	b.WriteString("- Do NOT write YAML front matter, an H1 title, or Mermaid diagrams — those are added separately. Start at \"## Introduction\".\n")
	b.WriteString("- Use fenced code blocks for any commands or configuration you cite from the context.\n\n")
	b.WriteString("=== RELEASE CONTEXT ===\n")
	b.WriteString(ground)
	b.WriteString("\n=== END RELEASE CONTEXT ===\n")
	return b.String()
}

// blogSections is the canonical long-form article structure.
var blogSections = []string{
	"Introduction",
	"Background and Context",
	"What Was Implemented",
	"Architecture and Design",
	"Implementation Details",
	"Repository and Code Changes",
	"Benefits and Outcomes",
	"How to Use or Extend the Feature",
	"Conclusion",
}

// assembleBlog wraps the model body with deterministic front matter, an H1
// title, and (when present and not already embedded) an architecture-diagrams
// section built from the context's real Mermaid diagrams.
func assembleBlog(title, meta string, tags []string, body string, rctx *rc.ReleaseContext) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fmt.Fprintf(&b, "description: %q\n", meta)
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString(body)

	if len(rctx.Mermaid) > 0 && !strings.Contains(body, "```mermaid") {
		b.WriteString("\n\n## Architecture Diagrams\n\n")
		b.WriteString("The following diagrams are taken directly from the repository's documentation.\n")
		for _, d := range rctx.Mermaid {
			if strings.TrimSpace(d.Source) != "" {
				fmt.Fprintf(&b, "\n_%s (%s)_\n\n", d.Summary, d.Source)
			} else {
				fmt.Fprintf(&b, "\n_%s_\n\n", d.Summary)
			}
			b.WriteString("```mermaid\n")
			b.WriteString(renderMermaid(d))
			b.WriteString("\n```\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderMermaid reconstructs a minimal, valid Mermaid diagram from the parsed
// edges so the article embeds an accurate diagram (not the original raw text,
// which the context does not retain verbatim).
func renderMermaid(d rc.MermaidDiagram) string {
	if d.Type == "sequence" {
		var b strings.Builder
		b.WriteString("sequenceDiagram")
		for _, e := range d.Edges {
			if e.Label != "" {
				fmt.Fprintf(&b, "\n    %s->>%s: %s", e.From, e.To, e.Label)
			} else {
				fmt.Fprintf(&b, "\n    %s->>%s: ", e.From, e.To)
			}
		}
		return b.String()
	}
	var b strings.Builder
	b.WriteString("flowchart TD")
	for _, e := range d.Edges {
		if e.Label != "" {
			fmt.Fprintf(&b, "\n    %s -->|%s| %s", e.From, e.Label, e.To)
		} else {
			fmt.Fprintf(&b, "\n    %s --> %s", e.From, e.To)
		}
	}
	return b.String()
}

func blogTitle(rctx *rc.ReleaseContext) string {
	if len(rctx.ContentIntelligence.BlogTitles) > 0 {
		return rctx.ContentIntelligence.BlogTitles[0]
	}
	name := rctx.Repository.Name
	if name == "" {
		name = rctx.Repository.FullName
	}
	return fmt.Sprintf("Inside %s %s: What Changed and Why It Matters", name, rctx.Release.Tag)
}

// metaDescription returns a single-line SEO meta description clamped to 160
// characters, grounded in the content-intelligence summary.
func metaDescription(rctx *rc.ReleaseContext) string {
	s := strings.TrimSpace(rctx.ContentIntelligence.Summary)
	if s == "" {
		s = strings.TrimSpace(rctx.Release.Summary)
	}
	if s == "" {
		s = fmt.Sprintf("A technical deep dive into %s release %s.", rctx.Repository.Name, rctx.Release.Tag)
	}
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace/newlines
	return clampMeta(s, 160)
}

func clampMeta(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max-1]
	if i := strings.LastIndexByte(cut, ' '); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

var tagSanitizeRe = regexp.MustCompile(`[^a-z0-9]+`)

// blogTags derives suggested publishing tags from the detected technologies and
// SEO keywords, normalized for platforms like Dev.to / Medium.
func blogTags(rctx *rc.ReleaseContext) []string {
	var raw []string
	for _, t := range rctx.Technologies {
		raw = append(raw, t.Name)
	}
	raw = append(raw, rctx.ContentIntelligence.SEOKeywords...)
	raw = append(raw, "release-notes", "software-architecture")

	seen := map[string]bool{}
	var tags []string
	for _, r := range raw {
		tag := tagSanitizeRe.ReplaceAllString(strings.ToLower(r), "-")
		tag = strings.Trim(tag, "-")
		if tag == "" || len(tag) < 2 || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) == 8 {
			break
		}
	}
	return tags
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

func (g *Generator) logBlog(rctx *rc.ReleaseContext, bytes int) {
	g.Logger.Info("blog post generated",
		"repository", rctx.Repository.FullName,
		"release", rctx.Release.Tag,
		"bytes", bytes,
	)
}
