package releasegen

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentcheck"
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

	// The release is the TRIGGER, not the topic — so the title is timeless and
	// topic-based. Prefer the one the plan proposed; fall back to a deterministic
	// non-release title. No version number in the title.
	title := timelessTitle(plan, rctx)

	// The meta description is timeless too: prefer the one the plan proposed, else
	// the deterministic fallback. Both pass through the SEO length window.
	meta := timelessDescription(plan, rctx)

	// Stage 5: write the article, guided by the plan and grounded strictly in the
	// context — then validate and regenerate on failure (the prompt lowers the
	// violation rate; this loop rejects the residual failures).
	md, err := g.writeValidatedArticle(ctx, rctx, title, meta, tags, plan)
	if err != nil {
		return BlogPost{}, err
	}

	if g.Logger != nil {
		g.logBlog(rctx, len(md))
	}
	return BlogPost{Title: title, MetaDescription: meta, Tags: tags, Markdown: md, WordCount: wordCount(md)}, nil
}

// defaultBlogAttempts is the number of article drafts Blog will try before
// accepting the best one it produced. 4 balances a higher clean rate against
// the per-attempt Claude cost for the stubborn count/lede patterns.
const defaultBlogAttempts = 4

// correctionBlock turns a failed draft's validation errors into a corrective
// instruction appended to the next attempt's prompt, so the model is told
// exactly which sentences to eliminate instead of resampling blindly.
func correctionBlock(report contentcheck.Report) string {
	var b strings.Builder
	b.WriteString("\n\nYOUR PREVIOUS DRAFT WAS REJECTED. Rewrite the ENTIRE article and eliminate EACH of these specific problems — do not merely soften them:\n")
	for _, iss := range report.Issues {
		if iss.Severity == contentcheck.SeverityError {
			b.WriteString("- ")
			b.WriteString(iss.Message)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// writeValidatedArticle generates the article, assembles the full blog, and
// validates it; on failure it regenerates (up to MaxBlogAttempts, relying on the
// model's sampling variance to produce a different draft). It returns the first
// draft that passes content validation, or — after the last attempt — the draft
// with the fewest validation errors. A model error is fatal only when no draft
// has been produced yet.
func (g *Generator) writeValidatedArticle(ctx context.Context, rctx *rc.ReleaseContext, title, meta string, tags []string, plan string) (string, error) {
	attempts := g.MaxBlogAttempts
	if attempts <= 0 {
		attempts = defaultBlogAttempts
	}
	basePrompt := g.articlePrompt(rctx, title, plan)
	prompt := basePrompt

	bestMD := ""
	bestErrs := int(^uint(0) >> 1) // max int
	for i := 0; i < attempts; i++ {
		body, err := g.Model.Generate(ctx, prompt)
		if err != nil {
			if bestMD == "" {
				return "", fmt.Errorf("generate blog: %w", err)
			}
			break // keep the best draft produced so far
		}
		md := assembleBlog(title, meta, tags, strings.TrimSpace(body), rctx)
		report := contentcheck.Validate("blog", md)
		if report.OK() {
			if i > 0 && g.Logger != nil {
				g.Logger.Info("blog passed validation after retry", "release", rctx.Release.Tag, "attempt", i+1)
			}
			return md, nil
		}
		if n := report.Errors(); n < bestErrs {
			bestErrs, bestMD = n, md
		}
		if g.Logger != nil {
			g.Logger.Warn("blog draft failed validation; regenerating",
				"release", rctx.Release.Tag, "attempt", i+1, "of", attempts, "errors", report.Errors())
		}
		// Reflection: tell the next attempt exactly what to fix, rather than
		// resampling blindly — the specific violations are a far stronger signal.
		prompt = basePrompt + correctionBlock(report)
	}
	if g.Logger != nil {
		g.Logger.Warn("blog validation not clean after all attempts; using best draft",
			"release", rctx.Release.Tag, "attempts", attempts, "residualErrors", bestErrs)
	}
	return bestMD, nil
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
	b.WriteString("You are a senior AWS/cloud engineer preparing to write a TIMELESS engineering blog post about a repository's technical design. Do NOT write the article yet — produce a grounded PLAN.\n\n")
	b.WriteString("The release is only the TRIGGER for the article, never its subject. Plan an article about the repository's engineering that stays valuable long after the release, and would make sense to a reader who never saw it.\n")
	b.WriteString("Treat the Release Context below as the Engineering Brief — the source of truth. Use ONLY what it contains; where there is no evidence for something, skip it — never guess, infer motivations, or describe planned work as done.\n")
	b.WriteString("Plan an article about THIS repository, not a generic AWS/Go/event-driven topic — each planned point must trace to a repository artifact. Capture the repository's OWN terminology verbatim, and never invent counts or statistics (files, packages, diagrams, components); describe relationships instead.\n\n")
	b.WriteString("STAGE 1 — Repository analysis: the repository's purpose and maturity, the current and previous milestones, the release scope, and what actually changed (files, packages; documentation vs code vs infrastructure vs AI/AWS; the code-vs-documentation ratio).\n")
	b.WriteString("STAGE 2 — Engineering analysis: for each important change, what problem it solves, why it was needed, what it enables next, its trade-offs, the AWS services involved, and its effect on scalability, maintainability, cost, and reliability. Keep a point ONLY if the context supports it.\n")
	b.WriteString("STAGE 3 — Architecture analysis: read the architecture graph in the context and identify the ACTUAL components and how they connect (name the specific routers, queues, providers, and services it lists) — never abstract the system to \"two components\". Then plan AT MOST TWO diagrams designed to explain the article's engineering topic, built from those real components, not copied verbatim from the repository. Plan no diagram when one would not aid understanding.\n")
	b.WriteString("STAGE 4 — Article plan: the single main engineering theme (the problem solved — not \"four commits\"), the supporting themes, the key AWS services, the target audience, SEO keywords, and concrete reader takeaways. Also decide whether the article's core is a genuine choice between TWO concrete alternatives the context supports (e.g. self-hosted vs managed, primary vs fallback, boot-time vs pre-baked); if so, plan a single comparison table, otherwise plan none. And plan the Introduction to open on the architectural problem, not the solution.\n\n")
	b.WriteString("OUTPUT a concise, structured plan (not prose, not the article). Write each field on its own line in the exact form \"FIELD: value\" as plain text — do NOT use Markdown headings (no \"#\", no \"##\") for these fields. The FIRST line MUST begin literally with \"TITLE:\".\n")
	b.WriteString("- TITLE: one timeless, topic-based article title about the engineering — in the style of \"Designing an Event-Driven AI Agent Platform on AWS\". It names the system and the engineering problem, NOT the release. No version number, no \"release\"/\"update\"/\"changelog\", no date.\n")
	b.WriteString("- DESCRIPTION: one timeless meta description (roughly 150–160 characters) summarising the article's engineering topic for search results. No version number, no \"release\"/\"update\", no date.\n")
	b.WriteString("- THEME: one sentence naming the engineering problem this repository solves.\n")
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
	b.WriteString("You are a Staff Software Engineer / Senior AWS Engineer writing a publication-quality engineering article that DOCUMENTS THIS repository's design and engineering decisions — the register of the AWS Builders' Library, Stripe Engineering, the Cloudflare Blog, and the Netflix and Uber engineering blogs. THIS repository is the sole source of truth; the article must be impossible to write without access to it. It is NOT marketing copy, NOT a changelog, and NOT a generic AWS/Go/event-driven tutorial. Write in the third person about the system; teach the reader about THIS repository's engineering; do not praise the project.\n\n")
	b.WriteString("Write roughly 1,500–2,500 words in GitHub-flavoured Markdown, executing the PLAN below. The working title is: ")
	b.WriteString(title)
	b.WriteString("\n\nUse these second-level (##) sections, in order, omitting any the PLAN marked OMIT:\n")
	for _, s := range blogSections {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	b.WriteString("\nThis is a TIMELESS engineering article. The GitHub release is only the TRIGGER that prompted it — it is NOT the topic. Write about the system's design so the article is still valuable to an engineer who reads it years from now and never saw the release. Every paragraph should answer at least one of: what problem existed, why was it difficult, why does this matter, how does it work, why was this approach chosen and not another, what are the trade-offs (cost, security, performance, operational, scalability, maintainability, and failure modes), and how would another engineer build something similar.\n\n")
	b.WriteString("HARD RULES:\n")
	b.WriteString("- Do NOT revolve the article around a release. Never write \"In this release\", \"This release delivers\", \"This update\", \"the latest version\", \"Version vX.Y.Z introduces\", or similar. Do NOT put version numbers or dates in the body — the version lives only in the front-matter metadata, which is added separately.\n")
	b.WriteString("- Ground EVERY claim in the PLAN and the Release Context. Never fabricate facts, motivations, architecture, implementation, AWS services, or design decisions.\n")
	b.WriteString("- Write in the third person. NEVER use first-person or invented narrative — no \"I built\", \"When I started\", \"I wanted\", \"I've built enough systems\", \"we decided\", \"we chose\", \"the team\", or personal anecdotes. State observable facts about the repository — e.g. \"The repository adopts a documentation-first architecture\", not \"When I started building this platform\".\n")
	b.WriteString("- Explain THIS repository's specific engineering decisions where the evidence supports them (why a component exists, why local and managed inference are combined, why a queue decouples the pipeline, why the infrastructure is defined as code). Where the context does not state a motivation, describe what exists and omit the why. Every major section must answer \"what engineering decision does this repository implement?\", not \"how does AWS work?\".\n")
	b.WriteString("- Do NOT teach AWS. The audience already understands Lambda, EventBridge, SQS, DynamoDB, IAM, CloudFormation, Go, and GitHub Actions — do not explain how these services work unless this repository uses them in an unusual way. Explain WHY this repository uses them and how it wires them together.\n")
	b.WriteString("- Use the repository's OWN terminology verbatim (its milestones and the routing, provider-abstraction, orchestration, content-generator, and Release Context concepts exactly as they appear in the context). Do not rename repository concepts to generic alternatives.\n")
	b.WriteString("- Describe the architecture using the ACTUAL components and relationships from the architecture graph in the context — name the specific routers, queues, providers, and services and how they connect. Do NOT reduce the system to \"two components\" or generic \"Component A / Component B\"; any diagram you write must use those real component names.\n")
	b.WriteString("- Open the Introduction with the architectural PROBLEM or tension this repository confronts (the coupling, vendor lock-in, boot-time cost, availability, or portability pressure) BEFORE any solution or implementation detail. Hook a senior engineer with the engineering problem in the first paragraph; introduce the repository only once the problem is framed.\n")
	b.WriteString("- In PROSE, always name components and services in their full, readable form (e.g. \"Amazon EventBridge\", \"AWS Lambda\", \"Amazon CloudWatch\", \"Amazon EFS\", \"Amazon S3\", and each component's real name). NEVER use the architecture graph's short node identifiers (such as eb, lam, cw, efs, s3, gh, iam) inside sentences — those abbreviations may appear ONLY inside Mermaid diagram code.\n")
	b.WriteString("- When the engineering centres on a genuine choice between TWO concrete alternatives that the context supports (e.g. a self-hosted vs a managed provider, a primary vs a fallback backend, boot-time vs pre-baked provisioning), include exactly ONE compact Markdown comparison table contrasting them across the dimensions that matter (role, hosting, cost model, operational burden, when each applies). Present both sides objectively and recommend neither. Add NO table when the context contains no real two-way contrast.\n")
	b.WriteString("- In Engineering Decisions, explain WHY each choice was made over the alternative a competent engineer would otherwise reach for — the reasoning, not just the mechanism. Where the context gives no rationale, state the decision and its observable consequence and omit invented motivation.\n")
	b.WriteString("- Make every trade-off SPECIFIC to this system: name the component or decision and exactly what it costs — what is given up, what surface it adds, what must now be maintained. Never settle for generic statements like \"more components add complexity\"; say which components, and why that added surface is the price of which benefit.\n")
	b.WriteString("- Never state raw counts — of files, directories, packages, diagrams, components, or services — EVEN when the context provides them; counts go stale. Describe the role or relationship instead. FORBIDDEN verbatim: \"organizes 65 files across four top-level directories\", \"employs 15 architecture diagrams\", \"has four top-level directories\". WRITE INSTEAD: \"separates infrastructure definitions from business logic and deployment entry points\".\n")
	b.WriteString("- Do NOT teach or describe technologies in the abstract ANYWHERE in the article — not in the Introduction, not in the Background, not in any section. FORBIDDEN sentence patterns: \"Event-driven architectures decouple…\", \"AWS provides services — Lambda…\", \"Go offers…\", \"Serverless is popular…\". Every section must be about THIS repository's problem, context, and decisions — never a primer on AWS, Go, or event-driven architecture. Open each section with the repository's constraint or decision, never a generic industry statement.\n")
	b.WriteString("- Never use \"likely\", \"probably\", \"presumably\", \"appears to\", \"it seems\", \"the team wanted\", or \"this was created because\" unless the context states it. Omit unknowns silently.\n")
	b.WriteString("- Do NOT narrate the changelog or reference commits unless strictly necessary. Explain engineering, not a commit list.\n")
	b.WriteString("- No AI filler, marketing language, or empty adjectives. Forbidden: \"exciting\", \"powerful\", \"revolutionary\", \"game-changing\", \"innovative\", \"next-generation\", \"cutting-edge\", \"state-of-the-art\", \"world-class\", \"future-proof\", \"marks a milestone\", \"showcases\", \"demonstrates commitment\". Concise, factual language only.\n")
	b.WriteString("- Vary sentence and paragraph structure. Do NOT use formulaic scaffolding such as \"The immediate benefit...\", \"The second benefit...\", or \"The obvious trade-off...\". Write as an experienced engineer documenting a real system.\n")
	b.WriteString("- Describe only architecture that exists in the context; never present planned work as implemented.\n")
	b.WriteString("- Include AT MOST TWO Mermaid diagrams, only where a diagram genuinely clarifies the engineering. Design each diagram for this article's topic — never paste a diagram verbatim from the repository. Omit diagrams entirely when they would not aid understanding.\n")
	b.WriteString("- In \"What's Next\", do NOT list roadmap items or future milestones. Explain what engineering this implementation now ENABLES: the architectural foundation it establishes and the capabilities it makes possible.\n")
	b.WriteString("- Never truncate. Every section must contain complete, meaningful content — never stop mid-heading or leave a section empty.\n")
	b.WriteString("- Do NOT write YAML front matter or an H1 title — those are added separately. Start at \"## Introduction\".\n")
	b.WriteString("- Use fenced code blocks for commands or configuration cited from the context. Use proper Unicode punctuation; never emit mojibake.\n")
	b.WriteString("- This article is the source that downstream generators (LinkedIn, X thread, video scripts, SEO metadata) transform. Write for engineers, not social media: clear section boundaries, consistent terminology, and each concept explained once. Do NOT add calls to action, hashtags, or engagement hooks.\n\n")
	b.WriteString("Before returning, verify: could this article be reused for a DIFFERENT repository by only changing the name? If yes, it is too generic — rewrite it. Does every major section reference THIS repository's implementation? Does it explain repository decisions rather than teach AWS? Third-person engineering voice; no invented personal stories, motivations, or statistics; not a release announcement; every claim grounded in the Release Context; at most two topic-specific Mermaid diagrams; complete sections; valid UTF-8 with no mojibake; a Staff Engineer would recognise it as repository documentation, not AI output. If any check fails, rewrite the article before returning it.\n\n")
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
	"Solution Overview",
	"Architecture",
	"Implementation Details",
	"Engineering Decisions",
	"Repository Changes",
	"Benefits",
	"Tradeoffs",
	"Applying the Pattern",
	"What's Next",
	"Conclusion",
}

// assembleBlog wraps the model body with deterministic front matter, an H1
// title, and (when present and not already embedded) an architecture-diagrams
// section built from the context's real Mermaid diagrams. It is defensive about
// the model's output: it sanitises the title, strips any stray front matter or
// H1 the model emitted (so they cannot duplicate the deterministic ones), and
// places the diagram appendix inside the article rather than after the
// Conclusion.
func assembleBlog(title, meta string, tags []string, body string, rctx *rc.ReleaseContext) string {
	title = sanitizeTitle(title)
	if title == "" {
		title = firstNonEmptyStr(rctx.Repository.Name, rctx.Repository.FullName, "Untitled")
	}
	body = stripConversationalScaffolding(stripStrayHeader(strings.TrimSpace(body)))

	// Attach at most two architecture diagrams from the release's real Mermaid
	// diagrams (never dump every diagram), and place them BEFORE the Conclusion
	// so the article does not end on a diagram appendix.
	if len(rctx.Mermaid) > 0 && !strings.Contains(body, "```mermaid") {
		body = insertBeforeConclusion(body, diagramSection(rctx))
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fmt.Fprintf(&b, "description: %q\n", meta)
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString(body)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// stripStrayHeader removes a leading YAML front-matter block and/or a leading H1
// that the model may have emitted despite being told not to, so they do not
// duplicate the ones assembleBlog adds deterministically.
func stripStrayHeader(body string) string {
	body = strings.TrimLeft(body, "\n")
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---"); end >= 0 {
			body = strings.TrimLeft(body[4+end+len("\n---"):], "\n")
		}
	}
	if strings.HasPrefix(body, "# ") {
		if nl := strings.IndexByte(body, '\n'); nl >= 0 {
			body = strings.TrimLeft(body[nl+1:], "\n")
		} else {
			body = ""
		}
	}
	return body
}

// scaffoldingLineRe matches a single line of conversational scaffolding a chat
// model may wrap around the article — a preamble announcing the deliverable
// ("Here is the article:", "Sure, here's the blog post…"), an assistant action
// or offer ("I'll deliver it inline", "Let me know if…", "If you'd like…"), or
// a narrated file-write refusal. It is deliberately narrow: every alternative
// references the deliverable, an assistant action, or an offer, so ordinary
// article sentences are not mistaken for scaffolding. This is the shared,
// provider-agnostic safeguard — the claude-code CLI is prevented from emitting
// this at the source, but any provider (Anthropic, Bedrock) routes through here.
var scaffoldingLineRe = regexp.MustCompile(`(?i)^\s*(?:` +
	// preamble that announces the deliverable
	`(?:sure|certainly|of course|absolutely)?[,.!]?\s*(?:here'?s|here is|below is|below you'?ll find|this is)\b[^.]*\b(?:article|post|blog|markdown|draft|revision|version|rewrite|plan)\b` +
	`|` +
	// explicit assistant action on the deliverable
	`(?:i'?ll|i will|i have|i'?ve|let me)\b[^.]*\b(?:write|written|draft(?:ed)?|deliver(?:ed|ing)?|provide[d]?|revis(?:e|ed)|save[d]?|paste[d]?)\b` +
	`|` +
	// narrated file-write refusal ("the write to output/ wasn't permitted…")
	`.*\bwrite\b[^.]*\b(?:was|wasn'?t|were|weren'?t)\b[^.]*\b(?:permitted|denied|declined|allowed|rejected)\b` +
	`|` +
	// trailing offers / sign-offs
	`(?:let me know\b|if you'?d like\b|if you would like\b|feel free to\b|would you like me\b|hope (?:this|it) helps\b|as requested\b|executing the plan\b|delivering it inline\b|i can save this\b)` +
	`).*$`)

// stripConversationalScaffolding removes chat scaffolding that a model may place
// before or after the article body: a preamble, a trailing offer, or a bare
// horizontal rule used to fence the article. It only strips leading/trailing
// lines that are blank, a lone rule, or match scaffoldingLineRe, and stops at
// the first real content on each end — so article prose is never touched.
func stripConversationalScaffolding(body string) string {
	lines := strings.Split(body, "\n")
	strip := func(s string) bool {
		t := strings.TrimSpace(s)
		return t == "" || t == "---" || t == "***" || t == "___" || scaffoldingLineRe.MatchString(t)
	}
	start, end := 0, len(lines)
	for start < end && strip(lines[start]) {
		start++
	}
	for end > start && strip(lines[end-1]) {
		end--
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

// diagramSection renders the architecture-diagram appendix from the release's
// real Mermaid diagrams (capped at maxBlogDiagrams).
func diagramSection(rctx *rc.ReleaseContext) string {
	diagrams := rctx.Mermaid
	if len(diagrams) > maxBlogDiagrams {
		diagrams = diagrams[:maxBlogDiagrams]
	}
	var b strings.Builder
	b.WriteString("## Architecture Diagrams\n\n")
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
	return b.String()
}

// insertBeforeConclusion splices section in just before the article's Conclusion
// heading (the last one), so a diagram appendix never trails after the
// conclusion. When there is no Conclusion, it appends.
func insertBeforeConclusion(body, section string) string {
	if strings.TrimSpace(section) == "" {
		return body
	}
	marker := "\n## Conclusion"
	if i := strings.LastIndex(body, marker); i >= 0 {
		return strings.TrimRight(body[:i], "\n") + "\n\n" + strings.TrimRight(section, "\n") + "\n" + body[i:]
	}
	return strings.TrimRight(body, "\n") + "\n\n" + section
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

// planTitleRe / planDescriptionRe extract the "TITLE: …" / "DESCRIPTION: …"
// lines the plan emits (plain text, optionally bulleted or bolded).
var (
	planTitleRe       = regexp.MustCompile(`(?mi)^\s*(?:[-*]\s*)?(?:\*\*)?TITLE(?:\*\*)?:\s*(.+?)\s*$`)
	planDescriptionRe = regexp.MustCompile(`(?mi)^\s*(?:[-*]\s*)?(?:\*\*)?DESCRIPTION(?:\*\*)?:\s*(.+?)\s*$`)
)

// planField returns the first value of a "FIELD: value" line the plan emitted,
// stripped of surrounding quotes; "" when absent.
func planField(plan string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(plan); m != nil {
		return strings.TrimSpace(strings.Trim(m[1], "\"'`"))
	}
	return ""
}

// timelessTitle returns the article title. It prefers the timeless, topic-based
// title the plan proposed; failing that, a deterministic non-release fallback.
// Either way the title carries no version — the release is the trigger, not the
// topic.
func timelessTitle(plan string, rctx *rc.ReleaseContext) string {
	if t := sanitizeTitle(planField(plan, planTitleRe)); t != "" {
		return t
	}
	return blogTitle(rctx)
}

// sanitizeTitle strips Markdown emphasis/heading markers and surrounding quotes
// from a title so the assembled H1 and front matter can never be degenerate —
// e.g. a model returning "**", "# **Title**", or a `code` title. It returns ""
// when nothing meaningful survives, so callers fall back to a deterministic
// title instead of emitting "# **".
func sanitizeTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimLeft(s, "#")) // stray heading markers
	s = strings.Trim(s, "*_`\"'")                   // surrounding emphasis / quotes
	s = strings.TrimSpace(s)
	if strings.Trim(s, "*_`~#>|-—–. \t") == "" {
		return "" // only punctuation/emphasis left — degenerate
	}
	return collapseWhitespace(s)
}

// timelessDescription returns the SEO meta description. It prefers the timeless
// description the plan proposed (run through the same length window as the
// deterministic path); failing that, the deterministic fallback.
func timelessDescription(plan string, rctx *rc.ReleaseContext) string {
	if d := planField(plan, planDescriptionRe); d != "" {
		return fitDescription(d, rctx)
	}
	return metaDescription(rctx)
}

// blogTitle is the deterministic fallback title. It is timeless — no version —
// because the article's value must outlast the release that triggered it.
func blogTitle(rctx *rc.ReleaseContext) string {
	if len(rctx.ContentIntelligence.BlogTitles) > 0 {
		if t := sanitizeTitle(rctx.ContentIntelligence.BlogTitles[0]); t != "" {
			return t
		}
	}
	name := rctx.Repository.Name
	if name == "" {
		name = rctx.Repository.FullName
	}
	return fmt.Sprintf("%s: Architecture and Engineering Design", name)
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
	return fitDescription(firstNonEmptyStr(rctx.ContentIntelligence.Summary, rctx.Release.Summary), rctx)
}

// fitDescription takes a seed description and returns one whose length is always
// within [metaMin, metaMax]: it pads a short seed with grounded enrichments and
// clamps a long one. Shared by the plan-derived and deterministic descriptions.
func fitDescription(seed string, rctx *rc.ReleaseContext) string {
	s := collapseWhitespace(seed)
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
	raw = append(raw, "software-architecture", "cloud-computing")

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
