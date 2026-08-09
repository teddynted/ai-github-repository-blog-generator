package releasegen

import (
	"context"
	"fmt"
	"hash/fnv"
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
// hallucinated — while the long-form narrative is produced by the Model from an
// archetype-driven, strictly-grounded prompt.
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

// promptBudget bounds the grounding (Release Context) block in each prompt.
func (g *Generator) promptBudget() int {
	if g.MaxPromptBytes > 0 {
		return g.MaxPromptBytes
	}
	return DefaultMaxPromptBytes
}

// planArticle runs the analysis + planning stages (repository, engineering, and
// architecture analysis, then the archetype + story-driven-heading plan) as a
// distinct model turn and returns a concise, grounded plan the writer follows.
// Reasoning-before-writing is what turns release-note narration into a narrative.
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
	archetype := chooseArchetype(rctx)
	var b strings.Builder
	b.WriteString("You are a senior AWS/cloud engineer preparing to write a TIMELESS engineering blog post about a repository's technical design. Do NOT write the article yet — produce a grounded PLAN.\n\n")
	b.WriteString("The release is only the TRIGGER for the article, never its subject. Plan an article about the repository's engineering that stays valuable long after the release, and would make sense to a reader who never saw it.\n")
	b.WriteString("Treat the Release Context below as the Engineering Brief — the source of truth. Use ONLY what it contains; where there is no evidence for something, skip it — never guess, infer motivations, or describe planned work as done.\n")
	b.WriteString("Plan an article about THIS repository, not a generic AWS/Go/event-driven topic — each planned point must trace to a repository artifact. Capture the repository's OWN terminology verbatim, and never invent counts or statistics (files, packages, diagrams, components); describe relationships instead.\n\n")
	b.WriteString("STAGE 1 — Repository analysis: the repository's purpose and maturity, the current and previous milestones, the release scope, and what actually changed (files, packages; documentation vs code vs infrastructure vs AI/AWS; the code-vs-documentation ratio).\n")
	b.WriteString("STAGE 2 — Engineering analysis: for each important change, what problem it solves, why it was needed, what it enables next, its trade-offs, the AWS services involved, and its effect on scalability, maintainability, cost, and reliability. Keep a point ONLY if the context supports it.\n")
	b.WriteString("STAGE 3 — Architecture analysis: read the architecture graph in the context and identify the ACTUAL components and how they connect (name the specific routers, queues, providers, and services it lists) — never abstract the system to \"two components\". Then plan AT MOST TWO diagrams designed to explain the article's engineering topic, built from those real components, not copied verbatim from the repository. Plan no diagram when one would not aid understanding.\n")
	b.WriteString("STAGE 4 — Engineering facts (extract, do not output as prose): identify the triggering observation, the real problem, the root cause, the constraint that made it hard, the chosen solution, the single most interesting implementation detail, the tradeoff introduced, and the future capability enabled. Keep only the ones the context supports.\n")
	b.WriteString("STAGE 5 — Narrative plan: this article uses the **" + archetype + "** archetype — " + archetypeShapes[archetype] + " Plan the article's spine to fit that archetype using the Stage 4 facts, and propose 4–7 SPECIFIC, story-driven section headings that emerge from THIS system's story (e.g. \"The 4-Minute Boot That Broke the Cost Model\", \"What the CloudWatch Logs Revealed\", \"The Assumption That Turned Out to Be Wrong\"). Do NOT propose generic template headings (\"Why This Matters\", \"The Solution\", \"Benefits\", \"Tradeoffs\", \"Conclusion\").\n\n")
	b.WriteString("OUTPUT a concise, structured plan (not prose, not the article). Write each field on its own line in the exact form \"FIELD: value\" as plain text — do NOT use Markdown headings (no \"#\", no \"##\") for these fields. The FIRST line MUST begin literally with \"TITLE:\".\n")
	b.WriteString("- TITLE: one timeless, topic-based article title about the engineering — in the style of \"Designing an Event-Driven AI Agent Platform on AWS\". It names the system and the engineering problem, NOT the release. No version number, no \"release\"/\"update\"/\"changelog\", no date.\n")
	b.WriteString("- DESCRIPTION: one timeless meta description (roughly 150–160 characters) summarising the article's engineering topic for search results. No version number, no \"release\"/\"update\", no date.\n")
	fmt.Fprintf(&b, "- ARCHETYPE: %s\n", archetype)
	b.WriteString("- THEME: one sentence naming the engineering problem this repository solves.\n")
	b.WriteString("- SUPPORTING THEMES / AWS SERVICES / AUDIENCE / SEO KEYWORDS / TAKEAWAYS: short, evidence-backed lists.\n")
	b.WriteString("- HEADINGS: the 4–7 specific, story-driven section headings in order, one per line under this field. Each must be concrete to this system, not a generic template heading. Every word in a heading must be a REAL, correctly-spelled English word or an actual product/technology/service name (e.g. Lambda, EventBridge, CloudWatch, Claude) — never an invented, garbled, abbreviated, or nonsense token, never a mangled or truncated product name, and never a grammatically incomplete phrase. Use the EXACT names of components as they appear in the Release Context; do not shorten or alter them.\n")
	b.WriteString("- COMPARISON: YES only when the article's core is a genuine choice between TWO concrete alternatives the context supports (self-hosted vs managed, primary vs fallback, boot-time vs pre-baked) that a single comparison table would clarify; otherwise NO.\n\n")
	b.WriteString("=== RELEASE CONTEXT ===\n")
	b.WriteString(ground)
	b.WriteString("\n=== END RELEASE CONTEXT ===\n")
	return b.String()
}

// articlePrompt writes the article from the plan, grounded strictly in the
// context, in the release's chosen narrative archetype with story-driven
// headings and a grounded first-person voice. Front matter, the H1, and diagrams
// are added deterministically afterwards, so the model produces only the body
// starting at its first ## heading.
func (g *Generator) articlePrompt(rctx *rc.ReleaseContext, title, plan string) string {
	ground := safeTruncate(contextBlock(rctx), g.promptBudget())
	var b strings.Builder
	archetype := chooseArchetype(rctx)
	b.WriteString("You are a senior platform engineer writing a publication-quality engineering article, in the register of the AWS Builders' Library, Stripe Engineering, the Cloudflare Blog, and the Netflix and Uber engineering blogs. Write as the engineer who built and OPERATED this system, reflecting on the real decisions, surprises, and tradeoffs. THIS repository is the sole source of truth; the article must be impossible to write without access to it. It is NOT marketing copy, NOT a changelog, and NOT a generic AWS/Go/event-driven tutorial; do not praise the project.\n\n")
	b.WriteString("Write a CONCISE, publication-quality article of 1,400–1,900 words (hard maximum 2,200) in GitHub-flavoured Markdown, executing the PLAN below. The working title is: ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("This is a TIMELESS engineering article — the GitHub release is only the TRIGGER that prompted it, never the topic. Write so it stays valuable to an engineer who reads it years from now and never saw the release. Every section should answer at least one real engineering question: why does this matter, how does it work, why was this approach chosen and not another, and what are the trade-offs.\n\n")
	b.WriteString("NARRATIVE STRUCTURE:\n")
	fmt.Fprintf(&b, "- This article uses the **%s** archetype: %s\n", archetype, archetypeShapes[archetype])
	b.WriteString("- Use the SPECIFIC, story-driven ## section headings from the PLAN's HEADINGS field, in order. If the plan gives none, write 4–7 headings that emerge from THIS system's story. Every heading must be concrete to this system (e.g. \"The 4-Minute Boot That Broke the Cost Model\", \"What the CloudWatch Logs Revealed\", \"The Assumption That Turned Out to Be Wrong\") — never a generic template heading. Every word in a heading must be a REAL, correctly-spelled word or an actual product/technology name — never an invented, garbled, or nonsense token, never a mangled or truncated product name, and never a grammatically incomplete phrase.\n")
	b.WriteString("- Let the story resolve naturally; do NOT force a summary section if it already lands. A final reflective heading is fine, but never a generic \"Conclusion\".\n\n")
	b.WriteString("FORBIDDEN HEADINGS — never use any of these as a heading (they are the tell of a template): ")
	b.WriteString(strings.Join(contentcheck.ForbiddenBlogHeadings, ", "))
	b.WriteString(".\n\n")
	b.WriteString("VOICE:\n")
	b.WriteString("- Write in a first-person engineering voice, reflecting on what building and operating THIS system taught. Natural phrasings are welcome: \"The part that surprised me was…\", \"At first I assumed…\", \"What took longer to notice was…\", \"The logs pointed somewhere unexpected.\", \"In practice, the bottleneck was…\".\n")
	b.WriteString("- Ground EVERY reflection in a fact from the Release Context. Do NOT invent incidents, measurements, timelines, debugging sessions, numbers, or feelings the context does not support. If the context has no operational anecdote, reflect on the DESIGN decision itself — never an invented event. If a metric is unknown, describe the effect qualitatively; never invent a number.\n")
	b.WriteString("- Avoid the tells of AI writing: \"This demonstrates the importance of…\", \"It is important to note that…\", \"This highlights the benefits of…\", \"Organizations can leverage…\", \"provides scalability, reliability, and maintainability.\"\n\n")
	b.WriteString("STRUCTURE & RHYTHM:\n")
	b.WriteString("- Vary paragraph length significantly; use an occasional single-sentence paragraph for emphasis.\n")
	b.WriteString("- Open the article with something concrete — a metric, a log line, a failed deploy, a surprising measurement, or a sharp design question — not a generic industry statement.\n")
	b.WriteString("- Use active voice; keep most sentences under 30 words. Use a table ONLY when it genuinely adds clarity (e.g. one real two-way comparison the PLAN marks COMPARISON: YES).\n")
	b.WriteString("- Explain each cross-cutting idea ONCE, then reference it briefly — never re-explain it later.\n\n")
	b.WriteString("HARD RULES:\n")
	b.WriteString("- Do NOT revolve the article around a release. Never write \"In this release\", \"This release delivers\", \"This update\", \"the latest version\", \"Version vX.Y.Z introduces\", or similar. Do NOT put version numbers or dates in the body — the version lives only in the front-matter metadata, which is added separately.\n")
	b.WriteString("- Ground EVERY claim in the PLAN and the Release Context. Never fabricate facts, motivations, architecture, implementation, AWS services, or design decisions.\n")
	b.WriteString("- Explain THIS repository's specific engineering decisions where the evidence supports them (why a component exists, why local and managed inference are combined, why a queue decouples the pipeline, why the infrastructure is defined as code). Where the context does not state a motivation, describe what exists and omit the why. Every section must be about THIS system's engineering, not \"how AWS works\".\n")
	b.WriteString("- Do NOT teach AWS. The audience already understands Lambda, EventBridge, SQS, DynamoDB, IAM, CloudFormation, Go, and GitHub Actions — do not explain how these services work unless this repository uses them in an unusual way. Explain WHY this repository uses them and how it wires them together.\n")
	b.WriteString("- Use the repository's OWN terminology verbatim (its milestones and the routing, provider-abstraction, orchestration, content-generator, and Release Context concepts exactly as they appear in the context). Do not rename repository concepts to generic alternatives.\n")
	b.WriteString("- Be terminology-CONSISTENT: pick ONE name for each concept and use it throughout. Reproduce Go identifiers, environment variables, state names, file paths, and boolean signal names EXACTLY as the context spells them (case and underscores included), and refer to each by that same token every time.\n")
	b.WriteString("- Describe the architecture using the ACTUAL components and relationships from the architecture graph in the context — name the specific routers, queues, providers, and services and how they connect. Do NOT reduce the system to \"two components\" or generic \"Component A / Component B\"; any diagram you write must use those real component names.\n")
	b.WriteString("- When the system separates distinct planes or paths (e.g. a control plane that schedules and dispatches versus a data plane that carries the work, or a trusted versus an untrusted path), name each plane explicitly and keep them distinct in both prose and diagrams. Only draw a plane distinction the context actually supports.\n")
	b.WriteString("- In PROSE, always name components and services in their full, readable form (e.g. \"Amazon EventBridge\", \"AWS Lambda\", \"Amazon CloudWatch\", \"Amazon EFS\", \"Amazon S3\", and each component's real name). NEVER use the architecture graph's short node identifiers (such as eb, lam, cw, efs, s3, gh, iam) inside sentences — those abbreviations may appear ONLY inside Mermaid diagram code.\n")
	b.WriteString("- Explain WHY each key choice was made over the alternative a competent engineer would otherwise reach for — the reasoning, not just the mechanism. Where the context gives no rationale, state the decision and its observable consequence and omit invented motivation.\n")
	b.WriteString("- Make every trade-off SPECIFIC to this system: name the component or decision and exactly what it costs — what is given up, what surface it adds, what must now be maintained. Never settle for \"more components add complexity\". When the engineering introduces pre-baked machine images or other baked build artifacts (for example custom AMIs and their backing snapshots), name the ongoing cost of owning those images — snapshot storage that accumulates per immutable version, the rebuild-and-re-version cadence, and security patching.\n")
	b.WriteString("- Never state raw counts — of files, directories, packages, diagrams, components, or services — EVEN when the context provides them; counts go stale. Describe the role or relationship instead. FORBIDDEN verbatim: \"organizes 65 files across four top-level directories\", \"employs 15 architecture diagrams\". WRITE INSTEAD: \"separates infrastructure definitions from business logic and deployment entry points\".\n")
	b.WriteString("- Do NOT teach or describe technologies in the abstract ANYWHERE. FORBIDDEN sentence patterns: \"Event-driven architectures decouple…\", \"AWS provides services — Lambda…\", \"Go offers…\", \"Serverless is popular…\". Open every section with THIS system's constraint, decision, or observation — never a generic industry statement.\n")
	b.WriteString("- Never use \"likely\", \"probably\", \"presumably\", \"appears to\", \"it seems\", \"the team wanted\", or \"this was created because\" unless the context states it. Omit unknowns silently.\n")
	b.WriteString("- Do NOT narrate the changelog or reference commits unless strictly necessary. Explain engineering, not a commit list.\n")
	b.WriteString("- No AI filler, marketing language, or empty adjectives. Forbidden: \"exciting\", \"powerful\", \"powerful solution\", \"revolutionary\", \"game-changing\", \"innovative\", \"next-generation\", \"cutting-edge\", \"state-of-the-art\", \"world-class\", \"future-proof\", \"seamlessly\", \"leverage the power of\", \"robust and scalable\", \"marks a milestone\", \"showcases\", \"demonstrates commitment\". Concise, factual language only.\n")
	b.WriteString("- Describe only architecture that exists in the context; never present planned work as implemented. Never introduce an orchestration, workflow, scheduling, queue, or container component the context does not show (Kubernetes, Amazon ECS, AWS Step Functions, Amazon SQS, …). Describe only the components the architecture graph names.\n")
	b.WriteString("- Include AT MOST TWO Mermaid diagrams, only where a diagram genuinely clarifies the engineering. Design each diagram for this article's topic — never paste a diagram verbatim from the repository. Omit diagrams entirely when they would not aid understanding.\n")
	b.WriteString("- Never truncate. Every section must contain complete, meaningful content — never stop mid-heading or leave a section empty, and finish with a complete final section.\n")
	b.WriteString("- Do NOT write YAML front matter or an H1 title — those are added separately. Start directly at your first ## story-driven heading.\n")
	b.WriteString("- Use fenced code blocks for commands or configuration cited from the context. Use proper Unicode punctuation; never emit mojibake.\n")
	b.WriteString("- This article is the source that downstream generators (LinkedIn, X thread, video scripts, SEO metadata) transform. Write for engineers, not social media. Do NOT add calls to action, hashtags, engagement hooks, \"link in bio\" language, subscription prompts, or TikTok-style hooks. Optimise for credible engineering writing first.\n\n")
	b.WriteString("Before returning, verify: could this article be reused for a DIFFERENT repository by only changing the name? If yes, it is too generic — rewrite it. Does it follow the chosen archetype with specific, story-driven headings and none of the forbidden generic headings? Is the first-person voice grounded in real facts, with no invented incidents, measurements, or numbers? Not a release announcement; every claim grounded in the Release Context; at most two topic-specific Mermaid diagrams; complete sections; valid UTF-8 with no mojibake; every heading uses only real, correctly-spelled words and the exact component names from the Release Context (no invented, garbled, or truncated tokens); reads like an engineer reflecting on a real system, not AI output. If any check fails, rewrite the article before returning it.\n\n")
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

// blogArchetypes are the narrative structures the article rotates through, so
// releases stop producing the same "Why This Matters → Solution → Benefits →
// Conclusion" template that reads as AI-generated. One is chosen per release and
// drives the article's shape and its story-driven headings.
var blogArchetypes = []string{
	"Incident-Driven",
	"Decision Journal",
	"Unexpected Bottleneck",
	"Myth-Busting",
	"Timeline / Evolution",
	"Tradeoff Analysis",
	"Measurement Changed Everything",
	"The Small Detail That Changed the System",
	"Operational Lesson Learned",
	"What I Wouldn't Build the Same Way Again",
}

// archetypeShapes gives the writer a one-line narrative spine per archetype.
var archetypeShapes = map[string]string{
	"Incident-Driven":                          "Open on a concrete symptom (a log line, a failed deploy, a cost spike), trace it to the root cause, then to the design change that resolved it.",
	"Decision Journal":                         "Walk the decision as it was actually made: the options on the table, what tilted the choice, and what it cost.",
	"Unexpected Bottleneck":                    "Start where the limit was assumed to be, then reveal where it actually was and how the design moved it.",
	"Myth-Busting":                             "Name an assumption that sounds right, show why it does not hold for this system, and what the evidence actually shows.",
	"Timeline / Evolution":                     "Trace how the design arrived here across milestones — what each step solved and why the next was needed.",
	"Tradeoff Analysis":                        "Centre the article on one genuine two-way choice, weigh both sides on this system's terms, and state which won and why.",
	"Measurement Changed Everything":           "Lead with a measurement (or its absence) and show how what it revealed reshaped the design.",
	"The Small Detail That Changed the System": "Start from one small, easily-missed detail (a flag, a device name, a cleanup step) and show how it decided the outcome.",
	"Operational Lesson Learned":               "Frame the article around what running the system taught that building it did not.",
	"What I Wouldn't Build the Same Way Again": "Reflect honestly on what the current design gets right and what a second attempt would change.",
}

// chooseArchetype selects a narrative archetype deterministically from the
// repository + release, so consecutive releases vary (different tags hash to
// different archetypes) while a regenerate of the SAME release is stable.
func chooseArchetype(rctx *rc.ReleaseContext) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(rctx.Repository.FullName + "@" + rctx.Release.Tag))
	return blogArchetypes[int(h.Sum32())%len(blogArchetypes)]
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
		body = insertBeforeLastSection(body, diagramSection(rctx))
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

// insertBeforeLastSection splices section in just before the article's LAST
// second-level (##) heading, so a diagram appendix sits with the body rather
// than trailing after the closing section. With story-driven headings there is
// no fixed "Conclusion" anchor, so the last ## section stands in for it. When
// the body has no ## heading, it appends.
func insertBeforeLastSection(body, section string) string {
	if strings.TrimSpace(section) == "" {
		return body
	}
	marker := "\n## "
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
