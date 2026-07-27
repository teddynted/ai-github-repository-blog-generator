package releasegen

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// maxBlogDiagrams caps how many architecture diagrams a blog embeds (the spec:
// never dump every diagram extracted from the repository).
const maxBlogDiagrams = 2

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
	b.WriteString("You are a senior software engineer writing a publication-quality engineering blog post for experienced software, cloud, platform, and AI engineers.\n")
	b.WriteString("This is NOT marketing copy, NOT documentation, and NOT a changelog. Write to educate engineers, in the register of the AWS Builders' Library, Stripe, Cloudflare, or Netflix engineering blogs — professional, technical, confident, clear.\n\n")
	b.WriteString("Write roughly 1,500–2,500 words in GitHub-flavoured Markdown. The working title is: ")
	b.WriteString(title)
	b.WriteString("\n\nUse these second-level (##) sections, in order, including only those the context supports (omit a section rather than pad it):\n")
	for _, s := range blogSections {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	b.WriteString("\nHARD RULES:\n")
	b.WriteString("- Ground EVERY claim in the Release Context below. Never invent facts, versions, features, code, AWS resources, or motivations.\n")
	b.WriteString("- Never speculate. Do not write phrases like \"probably\", \"this likely\", \"we wanted\", \"when I started this project\", or \"this was created because\". If something is not in the context, omit it silently — do not mention that it is missing.\n")
	b.WriteString("- Do NOT narrate the changelog or list commits (\"commit abc added X\"). Explain what the release ACCOMPLISHES and how it works.\n")
	b.WriteString("- No AI filler or marketing: never write \"this marks an exciting milestone\", \"demonstrates the power\", \"showcases innovation\", \"highlights the importance\", or \"revolutionises\".\n")
	b.WriteString("- Show engineering depth: architecture, system design, event flow, AWS services, design decisions, trade-offs, scalability, maintainability, developer experience, and extensibility — only where the context supports them.\n")
	b.WriteString("- Only discuss files or directories that are actually relevant to this release; do not describe the whole repository.\n")
	b.WriteString("- \"What's Next\": briefly introduce the next milestone or future direction using ONLY the future work / roadmap present in the context. Do not invent implementation details.\n")
	b.WriteString("- Do NOT write YAML front matter, an H1 title, or Mermaid diagrams — those are added separately. Start at \"## Introduction\".\n")
	b.WriteString("- Use fenced code blocks for any commands or configuration you cite from the context. Use proper Unicode punctuation (straight quotes and real em dashes); never emit mojibake.\n\n")
	b.WriteString("=== RELEASE CONTEXT ===\n")
	b.WriteString(ground)
	b.WriteString("\n=== END RELEASE CONTEXT ===\n")
	return b.String()
}

// blogSections is the canonical long-form article structure — the sequence a
// senior engineer would use to explain a release: what changed, why, how it
// works, the decisions and trade-offs, and where it goes next.
var blogSections = []string{
	"Introduction",
	"Background",
	"Problem Being Solved",
	"What's New in this Release",
	"Architecture",
	"Implementation Details",
	"Engineering Decisions",
	"Repository Changes",
	"Benefits",
	"Tradeoffs",
	"How Developers Can Use or Extend It",
	"What's Next",
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

	// Attach at most two architecture diagrams, chosen from the release's real
	// Mermaid diagrams — never dump every diagram extracted from the repository.
	if len(rctx.Mermaid) > 0 && !strings.Contains(body, "```mermaid") {
		diagrams := rctx.Mermaid
		if len(diagrams) > maxBlogDiagrams {
			diagrams = diagrams[:maxBlogDiagrams]
		}
		b.WriteString("\n\n## Architecture Diagrams\n\n")
		b.WriteString("The following diagrams are taken directly from the repository's documentation.\n")
		for _, d := range diagrams {
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

// SEO meta-description length window (in characters/runes).
const (
	metaMin = 150
	metaMax = 160
)

// metaDescription returns a single-line SEO meta description whose length is
// always within [metaMin, metaMax]. It starts from the content-intelligence
// summary and, when that is too short, enriches it with grounded clauses (AWS
// services, technologies, and factual descriptions of what the article covers)
// until it clears the floor — never with invented facts.
func metaDescription(rctx *rc.ReleaseContext) string {
	s := collapseWhitespace(firstNonEmptyStr(rctx.ContentIntelligence.Summary, rctx.Release.Summary))
	for _, clause := range metaEnrichments(rctx) {
		if runeLen(s) >= metaMin {
			break
		}
		s = collapseWhitespace(joinSentence(s, clause))
	}
	return fitMeta(s)
}

// metaEnrichments returns grounded clauses (most specific first) used to pad a
// short meta description up to the floor.
func metaEnrichments(rctx *rc.ReleaseContext) []string {
	var cs []string
	if svcs := firstNStr(rctx.Architecture.AWSServices, 3); len(svcs) > 0 {
		cs = append(cs, "It builds on "+strings.Join(svcs, ", ")+".")
	}
	if names := firstNStr(technologyNames(rctx), 3); len(names) > 0 {
		cs = append(cs, "Built with "+strings.Join(names, ", ")+".")
	}
	// Factual descriptions of what the blog article itself covers — true framing,
	// not fabricated release details — and enough in aggregate to clear the floor
	// even for a near-empty context.
	cs = append(cs,
		"This article explains what changed and why it matters.",
		"It covers the architecture and key design decisions.",
		"It walks through the implementation details.",
		"It shows how to use and extend the feature.",
	)
	return cs
}

// fitMeta clamps s to at most metaMax runes while keeping it at least metaMin
// (callers build s to already meet the floor). It trims on a word boundary when
// one exists above the floor, else hard-cuts, appending an ellipsis.
func fitMeta(s string) string {
	s = collapseWhitespace(s)
	r := []rune(s)
	if len(r) <= metaMax {
		return s
	}
	end := metaMax - 1 // leave one rune for the ellipsis
	for i := end; i >= metaMin; i-- {
		if r[i] == ' ' {
			end = i
			break
		}
	}
	out := strings.TrimRight(string(r[:end]), " ,;:") + "…"
	if runeLen(out) < metaMin {
		out = string(r[:metaMax-1]) + "…"
	}
	return out
}

func joinSentence(a, b string) string {
	a = strings.TrimSpace(a)
	if a == "" {
		return b
	}
	if !strings.HasSuffix(a, ".") && !strings.HasSuffix(a, "!") && !strings.HasSuffix(a, "?") {
		a += "."
	}
	return a + " " + b
}

func collapseWhitespace(s string) string { return strings.Join(strings.Fields(s), " ") }

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNStr(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

var tagSanitizeRe = regexp.MustCompile(`[^a-z0-9]+`)

// tagStopwords are meaningless publishing tags to drop (the spec forbids tags
// like "an", "the", "project", "repository").
var tagStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "on": true, "is": true, "it": true, "by": true,
	"as": true, "at": true, "be": true, "project": true, "repository": true,
	"repo": true, "code": true, "app": true,
}

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
		if tag == "" || len(tag) < 2 || seen[tag] || tagStopwords[tag] {
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
