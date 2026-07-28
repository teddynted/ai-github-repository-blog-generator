package releasegen

// Quality-regression guards for the blog-generation prompt.
//
// The blog prompt was redesigned to produce timeless, repository-grounded
// engineering articles instead of GitHub-release summaries. These tests lock
// that intent in: they assert on the *instructions the generator hands the
// model* (the plan and article prompts) and on the *deterministic pieces it
// assembles itself* (the title). They deliberately do NOT run a live model —
// the fake records prompts — so they stay fast and deterministic while still
// failing loudly if a future prompt edit reintroduces release-announcement
// writing, marketing language, hallucination latitude, weak engineering
// explanations, or version-centric titles.
//
// These extend (never replace) the existing prompt-structure tests in
// blog_test.go; overlap with those is intentionally avoided.

import (
	"context"
	"regexp"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// ---- helpers -------------------------------------------------------------

// buildBlogPrompts drives Blog with a prompt-recording fake model and returns
// the two prompts the generator emits: the Stage 1–4 plan prompt and the
// Stage 5 article prompt. Asserting on these (rather than a live model's prose)
// keeps the guards deterministic while still testing what the model is told.
func buildBlogPrompts(t *testing.T, rctx *rc.ReleaseContext) (plan, article string) {
	t.Helper()
	fm := &fakeModel{}
	// One attempt: these guards inspect prompt CONTENT, not the retry loop.
	if _, err := (&Generator{Model: fm, MaxBlogAttempts: 1}).Blog(context.Background(), rctx); err != nil {
		t.Fatalf("Blog: %v", err)
	}
	if len(fm.prompts) != 2 {
		t.Fatalf("expected 2 model turns (plan + article), got %d", len(fm.prompts))
	}
	return fm.prompts[0], fm.prompts[1]
}

// negationCues are the ways the prompt phrases a prohibition. forbidsPhrase uses
// them so a guard survives harmless rewording but still catches the dangerous
// case: a phrase that stays in the prompt but is flipped from banned to allowed.
var negationCues = []string{
	"never", "do not", "don't", "avoid", "must not", "no ", "not ",
	"omit", "forbid", "without", "instead of",
}

// forbidsPhrase reports whether the prompt instructs the model to AVOID phrase:
// the phrase must appear on a line that also carries a negation cue. Requiring
// the cue on the same line is what stops a future edit from satisfying the test
// by, say, telling the model to *use* "In this release".
func forbidsPhrase(prompt, phrase string) bool {
	lp := strings.ToLower(phrase)
	for _, line := range strings.Split(prompt, "\n") {
		ll := strings.ToLower(line)
		if !strings.Contains(ll, lp) {
			continue
		}
		for _, cue := range negationCues {
			if strings.Contains(ll, cue) {
				return true
			}
		}
	}
	return false
}

// containsFold is a case-insensitive substring check.
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// lineWith returns the first line of text containing sub (case-insensitive), or "".
func lineWith(text, sub string) string {
	ls := strings.ToLower(sub)
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(strings.ToLower(line), ls) {
			return line
		}
	}
	return ""
}

// versionRe matches version-like tokens ("v0.4.0", "1.2.0", "v2.1") so title
// guards can reject version-centric titles without brittle exact matching.
var versionRe = regexp.MustCompile(`(?i)v?\d+\.\d+(\.\d+)?`)

// ---- Test 1: Timeless article (highest priority) -------------------------

// TestBlogPromptForbidsReleaseCentricLanguage guards the core promise: the
// article is about the repository's engineering, and the GitHub release is only
// the trigger — never the topic. It fails if a future edit drops the ban on
// release-announcement phrasing or the "no version numbers in the body" rule,
// which is how articles regress into changelog/release-notes writing.
func TestBlogPromptForbidsReleaseCentricLanguage(t *testing.T) {
	_, article := buildBlogPrompts(t, sampleContext())

	// Release-announcement phrasing the article prompt must explicitly ban.
	banned := []string{
		"In this release",
		"This release delivers",
		"This update",
		"the latest version",
		"version numbers", // no version numbers in the body
	}
	for _, phrase := range banned {
		if !forbidsPhrase(article, phrase) {
			t.Errorf("article prompt no longer forbids release-centric phrasing %q "+
				"(the release must be the trigger, not the topic)", phrase)
		}
	}

	// The prompt must positively frame the article as timeless and the release
	// as merely the trigger.
	for _, cue := range []string{"timeless", "trigger", "years from now"} {
		if !containsFold(article, cue) {
			t.Errorf("article prompt missing timeless-framing cue %q", cue)
		}
	}
}

// TestPlanPromptEncouragesTimelessTopicTitle guards the planning turn: it must
// ask for a topic-driven, version-free title and offer a concrete exemplar of
// the desired register, so the model is steered away from "Inside X v1.2.0".
func TestPlanPromptEncouragesTimelessTopicTitle(t *testing.T) {
	plan, _ := buildBlogPrompts(t, sampleContext())

	if !containsFold(plan, "TITLE") || !containsFold(plan, "timeless") {
		t.Error("plan prompt no longer requests a timeless TITLE")
	}
	// The version ban must apply to the title the plan proposes.
	if !forbidsPhrase(plan, "version number") {
		t.Error("plan prompt no longer forbids a version number in the title")
	}
	// A concrete good exemplar keeps the model anchored on topic-driven titles.
	if !containsFold(plan, "Designing an Event-Driven AI Agent Platform on AWS") {
		t.Error("plan prompt dropped the timeless-title exemplar")
	}
}

// TestTimelessDescriptionPrefersPlanAndDropsRelease guards the SEO description:
// it must use the timeless description the plan proposes (not the release-centric
// ContentIntelligence summary), stay within the length window, and carry no
// version — and fall back cleanly when the plan proposes none.
func TestTimelessDescriptionPrefersPlanAndDropsRelease(t *testing.T) {
	c := sampleContext()
	c.ContentIntelligence.Summary = "Release v0.2.0 delivers 4 analyzed changes across 5 files."
	c.Release.Tag = "v0.2.0"

	plan := "TITLE: Designing an Event-Driven Platform on AWS\nDESCRIPTION: How the platform decouples event ingestion from processing with a durable queue and scheduled compute.\nTHEME: …"
	got := timelessDescription(plan, c)
	if n := runeLen(got); n < metaMin || n > metaMax {
		t.Errorf("description length = %d, want %d–%d: %q", n, metaMin, metaMax, got)
	}
	if versionRe.MatchString(got) || containsFold(got, "release") {
		t.Errorf("description is not timeless: %q", got)
	}
	if !containsFold(got, "decouples event ingestion") {
		t.Errorf("description did not use the plan's proposal: %q", got)
	}

	// With no plan description, it falls back to the deterministic path.
	if fb := timelessDescription("TITLE: X\nTHEME: y", c); fb == "" {
		t.Error("fallback description should be non-empty")
	}
}

// ---- Test 2: Repository grounding ----------------------------------------

// TestBlogPromptMandatesRepositoryGrounding guards the anti-hallucination
// contract: every claim grounded in evidence, nothing invented, unknowns
// omitted. It fails if the "never fabricate …" instruction stops covering any
// of the categories a model is most tempted to invent.
func TestBlogPromptMandatesRepositoryGrounding(t *testing.T) {
	_, article := buildBlogPrompts(t, sampleContext())

	if !(containsFold(article, "ground") && containsFold(article, "claim")) {
		t.Error("article prompt no longer requires grounding every claim in evidence")
	}

	// The single "never fabricate …" instruction must still enumerate every
	// invention category. Checking they share one negated line prevents a split
	// that quietly drops the prohibition on one of them.
	fabricateLine := lineWith(article, "fabricate")
	if fabricateLine == "" {
		fabricateLine = lineWith(article, "invent")
	}
	if fabricateLine == "" {
		t.Fatal("article prompt no longer forbids fabrication/invention at all")
	}
	for _, category := range []string{
		"motivations", "architecture", "implementation", "AWS services", "decisions",
	} {
		if !containsFold(fabricateLine, category) {
			t.Errorf("fabrication ban no longer covers %q (line: %q)", category, fabricateLine)
		}
	}

	// Unverifiable material must be dropped, not guessed at.
	if !containsFold(article, "Omit unknowns") {
		t.Error("article prompt no longer instructs the model to omit unknowns")
	}
}

// ---- Test 3: Engineering depth -------------------------------------------

// TestBlogPromptRequiresEngineeringDepthOverCommits guards that the article
// teaches engineering rather than narrating commits, and keeps the depth-
// oriented sections in a sensible teaching order. It fails if the prompt starts
// tolerating changelog narration or the section spine loses its shape.
func TestBlogPromptRequiresEngineeringDepthOverCommits(t *testing.T) {
	_, article := buildBlogPrompts(t, sampleContext())

	// Explain engineering, not a commit list.
	if !forbidsPhrase(article, "narrate the changelog") {
		t.Error("article prompt no longer discourages narrating the changelog")
	}
	if !containsFold(article, "commit list") {
		t.Error("article prompt no longer contrasts engineering explanation with a commit list")
	}

	// Each paragraph must answer a real engineering question — these cues keep
	// the prompt demanding reasoning, not summary.
	for _, cue := range []string{
		"why does this matter", "how does it work",
		"why was this approach chosen", "trade-offs",
	} {
		if !containsFold(article, cue) {
			t.Errorf("article prompt dropped the engineering-question cue %q", cue)
		}
	}

	// The depth sections must exist and stay in teaching order (constraint →
	// solution → architecture → implementation → decisions → tradeoffs → next).
	wantOrder := []string{
		"The Engineering Constraint", "The Solution", "Architecture Diagram",
		"Key Implementation Details", "Why These Decisions Were Made", "Tradeoffs",
		"What This Enables Next",
	}
	assertSubsequence(t, blogSections, wantOrder)
}

// assertSubsequence fails if want does not appear as an in-order subsequence of
// got (extra sections between the wanted ones are fine).
func assertSubsequence(t *testing.T, got, want []string) {
	t.Helper()
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	if i != len(want) {
		t.Errorf("blogSections %v does not contain %v in order (matched %d/%d)",
			got, want, i, len(want))
	}
}

// ---- Test 4: Architecture fidelity ---------------------------------------

// TestBlogPromptEnforcesArchitectureFidelity guards that only implemented
// architecture is described, roadmap work is never presented as shipped, and
// the two-diagram ceiling stays in place at both the instruction and the
// constant. (The behavioural cap is exercised separately in blog_test.go; this
// protects the *policy* those behaviours implement.)
func TestBlogPromptEnforcesArchitectureFidelity(t *testing.T) {
	plan, article := buildBlogPrompts(t, sampleContext())

	if !forbidsPhrase(article, "planned work as implemented") {
		t.Error("article prompt no longer forbids presenting planned work as implemented")
	}
	if !containsFold(article, "only architecture that exists") {
		t.Error("article prompt no longer restricts description to architecture that exists")
	}

	// The plan turn must cap diagram selection at two.
	if !containsFold(plan, "at most two") {
		t.Error("plan prompt no longer caps diagram selection at two")
	}
	// A silent bump of the assembly-time ceiling is itself a regression.
	if maxBlogDiagrams != 2 {
		t.Errorf("maxBlogDiagrams = %d, want 2 (the article embeds at most two diagrams)", maxBlogDiagrams)
	}
}

// ---- Test 5: Downstream (hybrid-routing) awareness -----------------------

// TestBlogPromptCarriesDownstreamAwareness guards that the blog is written as
// the upstream SOURCE the Ollama generators transform — optimised for engineers,
// not for social media. The authoritative Claude-vs-Ollama artifact split is
// enforced and tested in internal/airouter/router_test.go; this covers the
// blog-prompt side of that contract, which router tests cannot see.
func TestBlogPromptCarriesDownstreamAwareness(t *testing.T) {
	_, article := buildBlogPrompts(t, sampleContext())

	if !containsFold(article, "downstream generators") {
		t.Error("article prompt no longer tells Claude it is the source for downstream generators")
	}
	if !containsFold(article, "for engineers") {
		t.Error("article prompt no longer directs the writing at engineers")
	}
	// Social-media optimisation belongs downstream (Ollama), not in the blog.
	for _, phrase := range []string{"hashtags", "calls to action"} {
		if !forbidsPhrase(article, phrase) {
			t.Errorf("article prompt no longer keeps social-media device %q out of the blog", phrase)
		}
	}
}

// ---- Test 6: Calm engineering register (anti-marketing) ------------------

// TestBlogPromptMaintainsCalmEngineeringRegister guards the voice: anchored to
// engineering publications, free of marketing copy, praise, and empty
// adjectives. It fails if the reference register or the bans on hype are
// weakened.
func TestBlogPromptMaintainsCalmEngineeringRegister(t *testing.T) {
	_, article := buildBlogPrompts(t, sampleContext())

	// Register anchors — the calibre the prompt targets.
	for _, ref := range []string{"AWS Builders' Library", "Stripe", "Cloudflare", "Netflix"} {
		if !containsFold(article, ref) {
			t.Errorf("article prompt dropped register anchor %q", ref)
		}
	}

	// Marketing tone and self-praise must stay banned.
	for _, phrase := range []string{"marketing copy", "praise", "filler"} {
		if !forbidsPhrase(article, phrase) {
			t.Errorf("article prompt no longer rules out %q", phrase)
		}
	}
	// The empty-adjective examples double as a marker that the ban is concrete.
	if !forbidsPhrase(article, "exciting") {
		t.Error("article prompt no longer lists banned hype adjectives (e.g. \"exciting\")")
	}
}

// ---- Test 7: Title quality (topic-driven, not version-driven) ------------

// TestBlogTitleIsTopicDrivenNotVersioned guards the title end to end. The
// deterministic fallback must never carry a version, timelessTitle must honour
// a plan-proposed topic title, and the assembled article's title/H1 must be
// version-free. This is the regression guard against the old
// "Inside <name> <tag>: What Changed" format.
func TestBlogTitleIsTopicDrivenNotVersioned(t *testing.T) {
	// Fallback title (no content-intelligence titles available) must be
	// topic-based and free of any version token.
	c := sampleContext()
	c.ContentIntelligence.BlogTitles = nil
	c.Release.Tag = "v0.4.0"
	fallback := blogTitle(c)
	if versionRe.MatchString(fallback) {
		t.Errorf("fallback title %q contains a version token; titles must be timeless", fallback)
	}
	if containsFold(fallback, "release notes") || containsFold(fallback, "what changed") {
		t.Errorf("fallback title %q reads like a release announcement", fallback)
	}

	// timelessTitle must prefer a topic-driven title the plan proposed.
	plan := "reasoning…\nTITLE: Designing an Event-Driven AI Agent Platform on AWS\nOUTLINE: …"
	if got := timelessTitle(plan, c); got != "Designing an Event-Driven AI Agent Platform on AWS" {
		t.Errorf("timelessTitle did not adopt the plan's title: got %q", got)
	}

	// End to end: the assembled article's title and H1 must be version-free even
	// when the triggering release carries a version tag.
	fm := &fakeModel{reply: func(string) (string, error) { return "## Introduction\n\nBody.", nil }}
	post, err := (&Generator{Model: fm}).Blog(context.Background(), c)
	if err != nil {
		t.Fatalf("Blog: %v", err)
	}
	if versionRe.MatchString(post.Title) {
		t.Errorf("assembled title %q contains a version token", post.Title)
	}
	if h1 := lineWith(post.Markdown, "# "); versionRe.MatchString(h1) {
		t.Errorf("assembled H1 %q contains a version token", h1)
	}
}
