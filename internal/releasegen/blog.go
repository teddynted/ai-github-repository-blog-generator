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

	// Stage 1–4: reason about the repository, the engineering, and the
	// architecture, then plan the article — BEFORE writing a word. The reasoning
	// happens in its own model turn so the writer executes a grounded narrative
	// instead of narrating the release. The plan is internal and non-fatal: if it
	// fails, the writer still works from the context alone.
	plan, planErr := g.planArticle(ctx, rctx)
	if planErr != nil && g.Logger != nil {
		g.Logger.Warn("blog planning failed; writing without a plan",
			"release", rctx.Release.Tag, "error", planErr.Error())
	}

	// Stage 5: write the article, section by section, guided by the plan and
	// grounded strictly in the context.
	body, err := g.Model.Generate(ctx, g.articlePrompt(rctx, title, plan))
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
// promptBudget bounds the grounding block in each prompt.
func (g *Generator) promptBudget() int {
	if g.MaxPromptBytes > 0 {
		return g.MaxPromptBytes
	}
	return DefaultMaxPromptBytes
}

// planArticle runs Stages 1–4 (repository, engineering, and architecture
// analysis, then article planning) as a distinct model turn and returns a
// concise, grounded plan the writer follows. Reasoning-before-writing is the
// change that turns release-note narration into an engineering narrative.
func (g *Generator) planArticle(ctx context.Context, rctx *rc.ReleaseContext) (string, error) {
	out, err := g.Model.Generate(ctx, g.planPrompt(rctx))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// planPrompt asks the model to analyse and plan — NOT to write the article.
// Every stage is evidence-only: unsupported points are skipped, never guessed.
func (g *Generator) planPrompt(rctx *rc.ReleaseContext) string {
	ground := safeTruncate(contextBlock(rctx), g.promptBudget())
	var b strings.Builder
	b.WriteString("You are a senior AWS/cloud engineer preparing to write an engineering blog post about a software release. Do NOT write the article yet — produce a grounded PLAN.\n\n")
	b.WriteString("Work through these stages using ONLY the Release Context below. Where there is no evidence for something, skip it — never guess, infer motivations, or describe planned work as done.\n\n")
	b.WriteString("STAGE 1 — Repository analysis: the repository's purpose and maturity, the current and previous milestones, the release scope, and what actually changed (files, packages; documentation vs code vs infrastructure vs AI/AWS; the code-vs-documentation ratio).\n")
	b.WriteString("STAGE 2 — Engineering analysis: for each important change, what problem it solves, why it was needed, what it enables next, its trade-offs, the AWS services involved, and its effect on scalability, maintainability, cost, and reliability. Keep a point ONLY if the context supports it.\n")
	b.WriteString("STAGE 3 — Architecture analysis: only architecture that EXISTS in the context. If diagrams are present, choose AT MOST TWO that best explain this release; otherwise none.\n")
	b.WriteString("STAGE 4 — Article plan: the single main engineering theme (the problem solved — not \"four commits\"), the supporting themes, the key AWS services, the target audience, SEO keywords, and concrete reader takeaways.\n\n")
	b.WriteString("OUTPUT a concise, structured plan (not prose, not the article):\n")
	b.WriteString("- THEME: one sentence naming the engineering problem this release addresses.\n")
	b.WriteString("- SUPPORTING THEMES / AWS SERVICES / AUDIENCE / SEO KEYWORDS / TAKEAWAYS: short, evidence-backed lists.\n")
	b.WriteString("- OUTLINE: for each of these sections, 1–3 grounded bullet points it will make, or the single word OMIT when the context offers nothing: ")
	b.WriteString(strings.Join(blogSections, ", "))
	b.WriteString(".\n\n")
	b.WriteString("=== RELEASE CONTEXT ===\n")
	b.WriteString(ground)
	b.WriteString("\n=== END RELEASE CONTEXT ===\n")
	return b.String()
}

// articlePrompt is Stage 5: write the article from the plan, grounded strictly
// in the context. Front matter, the H1, and diagrams are added deterministically
// afterwards, so the model produces only the body from "## Introduction" onward.
func (g *Generator) articlePrompt(rctx *rc.ReleaseContext, title, plan string) string {
	ground := safeTruncate(contextBlock(rctx), g.promptBudget())
	var b strings.Builder
	b.WriteString("You are a senior AWS/cloud engineer writing a publication-quality engineering blog post — the register of the AWS Builders' Library, Stripe, Cloudflare, or Netflix engineering blogs. It is NOT marketing copy, NOT documentation, and NOT a changelog. Teach the reader; do not praise the project.\n\n")
	b.WriteString("Write roughly 1,500–2,500 words in GitHub-flavoured Markdown, executing the PLAN below. The working title is: ")
	b.WriteString(title)
	b.WriteString("\n\nUse these second-level (##) sections, in order, omitting any the PLAN marked OMIT:\n")
	for _, s := range blogSections {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	b.WriteString("\nThe release version only identifies WHAT changed; the article explains WHY it matters. Every paragraph should answer at least one of: why does this matter, how does it work, why was this approach chosen, what are the trade-offs, how would another engineer build something similar.\n\n")
	b.WriteString("HARD RULES:\n")
	b.WriteString("- Ground EVERY claim in the PLAN and the Release Context. Never fabricate facts, motivations, architecture, implementation, AWS services, or design decisions.\n")
	b.WriteString("- Never use \"likely\", \"probably\", \"presumably\", \"appears to\", \"it seems\", \"the team wanted\", or \"this was created because\" unless the context states it. Omit unknowns silently.\n")
	b.WriteString("- Do NOT narrate the changelog or reference commits unless strictly necessary. Explain engineering, not a commit list.\n")
	b.WriteString("- No AI filler and no adjectives that add no information (\"exciting\", \"powerful\", \"showcases innovation\", \"revolutionises\"). Concise language only.\n")
	b.WriteString("- Describe only architecture that exists in the context; never present planned work as implemented.\n")
	b.WriteString("- Do NOT write YAML front matter, an H1 title, or Mermaid diagrams — those are added separately. Start at \"## Introduction\".\n")
	b.WriteString("- Use fenced code blocks for commands or configuration cited from the context. Use proper Unicode punctuation; never emit mojibake.\n\n")
	if strings.TrimSpace(plan) != "" {
		b.WriteString("=== PLAN (follow this) ===\n")
		b.WriteString(plan)
		b.WriteString("\n=== END PLAN ===\n\n")
	}
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
	"Engineering Problem",
	"What Changed",
	"Architecture",
	"Implementation Details",
	"Engineering Decisions",
	"Repository Changes",
	"Benefits",
	"Tradeoffs",
	"How Developers Can Apply This",
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
