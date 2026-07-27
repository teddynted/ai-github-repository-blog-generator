package releasegen

import (
	"context"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

func blogContext() *rc.ReleaseContext {
	c := sampleContext()
	c.Mermaid = []rc.MermaidDiagram{{
		Source: "docs/architecture.md", Type: "flowchart",
		Nodes:     []string{"A", "B", "C"},
		Edges:     []rc.MermaidEdge{{From: "A", To: "B"}, {From: "B", To: "C", Label: "match"}},
		NodeCount: 3, EdgeCount: 2, Summary: "flowchart diagram with 3 nodes and 2 relationships.",
	}}
	return c
}

func TestBlogAssemblesFrontMatterAndBody(t *testing.T) {
	fm := &fakeModel{reply: func(string) (string, error) {
		return "## Introduction\n\nThis release matters.\n\n## Conclusion\n\nThat's it.", nil
	}}
	g := &Generator{Model: fm}
	post, err := g.Blog(context.Background(), blogContext())
	if err != nil {
		t.Fatalf("Blog: %v", err)
	}

	if post.Title != "Inside widget v0.2.0" {
		t.Errorf("title = %q", post.Title)
	}
	if post.MetaDescription == "" || len(post.MetaDescription) > 160 {
		t.Errorf("meta description length = %d (%q)", len(post.MetaDescription), post.MetaDescription)
	}
	if len(post.Tags) == 0 {
		t.Error("no tags")
	}

	md := post.Markdown
	for _, want := range []string{
		"---\n", "title: \"Inside widget v0.2.0\"", "description: ", "tags: [",
		"# Inside widget v0.2.0", "## Introduction", "This release matters.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	if post.WordCount == 0 {
		t.Error("word count is zero")
	}
}

func TestBlogEmbedsMermaidDiagrams(t *testing.T) {
	fm := &fakeModel{reply: func(string) (string, error) { return "## Introduction\n\nProse.", nil }}
	post, err := (&Generator{Model: fm}).Blog(context.Background(), blogContext())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(post.Markdown, "## Architecture Diagrams") {
		t.Error("architecture diagrams section not added")
	}
	if !strings.Contains(post.Markdown, "```mermaid") {
		t.Error("mermaid block not embedded")
	}
	// Reconstructed diagram must reflect the parsed edges (including the label).
	if !strings.Contains(post.Markdown, "A --> B") || !strings.Contains(post.Markdown, "B -->|match| C") {
		t.Errorf("diagram edges not rendered:\n%s", post.Markdown)
	}
}

func TestBlogDoesNotDuplicateMermaid(t *testing.T) {
	// If the model already emitted a mermaid block, don't append another section.
	fm := &fakeModel{reply: func(string) (string, error) {
		return "## Architecture and Design\n\n```mermaid\nflowchart TD\n  X --> Y\n```", nil
	}}
	post, _ := (&Generator{Model: fm}).Blog(context.Background(), blogContext())
	if strings.Contains(post.Markdown, "## Architecture Diagrams") {
		t.Error("should not add a diagrams section when the body already has mermaid")
	}
}

func TestBlogRetriesUntilValid(t *testing.T) {
	articleAttempts := 0
	fm := &fakeModel{reply: func(p string) (string, error) {
		if strings.Contains(p, "produce a grounded PLAN") {
			return "TITLE: Designing an Event-Driven Platform\nDESCRIPTION: How the platform decouples ingestion from processing.\nTHEME: x", nil
		}
		articleAttempts++
		if articleAttempts == 1 {
			// Invalid: a generic technology lede + no Conclusion.
			return "## Introduction\n\nEvent-driven architectures decouple producers from consumers.", nil
		}
		// Valid: grounded, complete.
		return "## Introduction\n\nThe repository routes events through a durable queue.\n\n## Conclusion\n\nThe pattern holds.", nil
	}}

	post, err := (&Generator{Model: fm, MaxBlogAttempts: 3}).Blog(context.Background(), blogContext())
	if err != nil {
		t.Fatalf("Blog: %v", err)
	}
	if articleAttempts != 2 {
		t.Errorf("expected 2 article attempts (1 invalid, 1 valid), got %d", articleAttempts)
	}
	if !strings.Contains(post.Markdown, "routes events through a durable queue") ||
		strings.Contains(post.Markdown, "Event-driven architectures decouple") {
		t.Error("Blog returned the invalid draft instead of the clean retry")
	}
}

func TestBlogFallsBackToBestDraft(t *testing.T) {
	// Every draft is invalid (missing Conclusion) — Blog must still return the
	// best-effort draft without erroring.
	fm := &fakeModel{reply: func(p string) (string, error) {
		if strings.Contains(p, "produce a grounded PLAN") {
			return "TITLE: X\nTHEME: y", nil
		}
		return "## Introduction\n\nThe repository decouples ingestion from processing.", nil
	}}
	post, err := (&Generator{Model: fm, MaxBlogAttempts: 3}).Blog(context.Background(), blogContext())
	if err != nil {
		t.Fatalf("Blog: %v", err)
	}
	if post.Markdown == "" || !strings.Contains(post.Markdown, "## Introduction") {
		t.Error("expected a best-effort draft even when all attempts fail validation")
	}
}

func TestBlogPlanThenWritePrompts(t *testing.T) {
	fm := &fakeModel{}
	// One attempt: this test checks the plan/article prompt content, not retries.
	_, _ = (&Generator{Model: fm, MaxBlogAttempts: 1}).Blog(context.Background(), blogContext())

	// Two model turns: Stage 1–4 plan, then Stage 5 article.
	if len(fm.prompts) != 2 {
		t.Fatalf("expected 2 model calls (plan + write), got %d", len(fm.prompts))
	}
	plan := fm.prompts[0]
	for _, want := range []string{
		"Do NOT write the article yet", "STAGE 1", "STAGE 2", "STAGE 3", "STAGE 4",
		"THEME", "OUTLINE", "TITLE", // timeless title requested in the plan
		"only the TRIGGER",      // release is trigger, not topic
		"acme/widget", "v0.2.0", // grounding
	} {
		if !strings.Contains(plan, want) {
			t.Errorf("plan prompt missing %q", want)
		}
	}

	article := fm.prompts[1]
	for _, want := range []string{
		"Introduction", "Engineering Problem", "Solution Overview", "Engineering Decisions",
		"Tradeoffs", "Applying the Pattern", "What's Next", "Conclusion",
		"Never fabricate", "Omit unknowns silently", "1,500–2,500 words",
		"Do NOT write YAML front matter",
		"TIMELESS engineering article", "This release delivers", // timeless framing + forbidden phrase
		"acme/widget", "add release context builder", // grounding
	} {
		if !strings.Contains(article, want) {
			t.Errorf("article prompt missing %q", want)
		}
	}
	// The plan must be threaded into the writing prompt.
	if !strings.Contains(article, "PLAN (follow this)") {
		t.Error("article prompt should embed the plan")
	}
}

func TestMetaDescriptionWithinWindow(t *testing.T) {
	cases := map[string]string{
		"long":    strings.Repeat("word ", 60), // ~300 chars -> must trim
		"short":   "Small release.",            // must pad up to the floor
		"empty":   "",                          // nothing -> still within window
		"midlow":  strings.Repeat("word ", 28), // ~140 chars -> just under the floor
		"inrange": strings.Repeat("a", 155),    // already in [150,160]
	}
	for name, summary := range cases {
		t.Run(name, func(t *testing.T) {
			c := sampleContext()
			c.ContentIntelligence.Summary = summary
			c.Release.Summary = ""
			m := metaDescription(c)
			n := len([]rune(m))
			if n < 150 || n > 160 {
				t.Errorf("meta length = %d, want 150–160: %q", n, m)
			}
		})
	}
}

func TestMetaDescriptionLongIsTruncated(t *testing.T) {
	c := sampleContext()
	c.ContentIntelligence.Summary = strings.Repeat("word ", 60)
	m := metaDescription(c)
	if !strings.HasSuffix(m, "…") {
		t.Errorf("long meta should end with an ellipsis: %q", m)
	}
}

func TestSanitizeTitle(t *testing.T) {
	cases := map[string]string{
		"**":                       "",
		"# **":                     "",
		"**Designing on AWS**":     "Designing on AWS",
		"# Designing on AWS":       "Designing on AWS",
		"`code title`":             "code title",
		"\"Quoted Title\"":         "Quoted Title",
		"Designing an AI Platform": "Designing an AI Platform",
		"   Padded   Title   ":     "Padded Title",
	}
	for in, want := range cases {
		if got := sanitizeTitle(in); got != want {
			t.Errorf("sanitizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAssembleBlogHandlesDegenerateTitle(t *testing.T) {
	rctx := blogContext() // Repository.Name = "widget"
	md := assembleBlog("**", "desc padded to a reasonable length for the SEO window here", []string{"aws"}, "## Introduction\n\nBody.\n\n## Conclusion\n\nEnd.", rctx)
	if strings.Contains(md, "# **") || strings.Contains(md, `title: "**"`) {
		t.Errorf("degenerate title leaked into output:\n%s", md[:120])
	}
	if !strings.Contains(md, "# widget") {
		t.Errorf("expected fallback title from repo name, got:\n%s", md[:160])
	}
}

func TestAssembleBlogStripsStrayHeader(t *testing.T) {
	// The model disobeys and emits its own front matter + H1; assembleBlog must
	// strip them so they do not duplicate the deterministic ones.
	body := "---\ntitle: \"Model Title\"\ntags: [x]\n---\n\n# Model H1\n\n## Introduction\n\nBody.\n\n## Conclusion\n\nEnd."
	md := assembleBlog("Real Title", "desc padded to a reasonable length for the SEO window here now", []string{"aws"}, body, blogContext())
	if strings.Count(md, "---\n") != 2 { // exactly one front-matter block (open+close)
		t.Errorf("stray front matter not stripped:\n%s", md)
	}
	if strings.Contains(md, "# Model H1") || strings.Contains(md, "Model Title") {
		t.Errorf("stray model header leaked:\n%s", md)
	}
	if !strings.Contains(md, "# Real Title") {
		t.Error("deterministic H1 missing")
	}
}

func TestAssembleBlogPlacesDiagramsBeforeConclusion(t *testing.T) {
	rctx := blogContext() // has one Mermaid diagram, no inline mermaid in body
	body := "## Introduction\n\nIntro.\n\n## Conclusion\n\nFinal words."
	md := assembleBlog("Title", "desc padded to a reasonable length for the SEO window right here", []string{"aws"}, body, rctx)
	dIdx := strings.Index(md, "## Architecture Diagrams")
	cIdx := strings.LastIndex(md, "## Conclusion")
	if dIdx < 0 {
		t.Fatal("diagram section not embedded")
	}
	if dIdx > cIdx {
		t.Errorf("diagram section is AFTER the Conclusion (misplaced): diagrams@%d, conclusion@%d", dIdx, cIdx)
	}
}

func TestBlogTagsNormalized(t *testing.T) {
	tags := blogTags(blogContext())
	for _, tag := range tags {
		if tag != strings.ToLower(tag) || strings.Contains(tag, " ") {
			t.Errorf("tag %q not normalized (lowercase, no spaces)", tag)
		}
	}
	if len(tags) > 8 {
		t.Errorf("too many tags: %d", len(tags))
	}
}

func TestBlogNoModel(t *testing.T) {
	if _, err := (&Generator{}).Blog(context.Background(), blogContext()); err == nil {
		t.Error("expected error without a model")
	}
}

func TestBlogCapsArchitectureDiagramsAtTwo(t *testing.T) {
	rctx := blogContext()
	// Give the context more diagrams than the cap.
	rctx.Mermaid = []rc.MermaidDiagram{
		{Summary: "one", Type: "flowchart", Edges: []rc.MermaidEdge{{From: "A", To: "B"}}},
		{Summary: "two", Type: "flowchart", Edges: []rc.MermaidEdge{{From: "C", To: "D"}}},
		{Summary: "three", Type: "flowchart", Edges: []rc.MermaidEdge{{From: "E", To: "F"}}},
	}
	md := assembleBlog("Title", "desc", []string{"aws"}, "## Introduction\n\nBody.", rctx)
	if n := strings.Count(md, "```mermaid"); n > maxBlogDiagrams {
		t.Errorf("embedded %d diagrams, want <= %d", n, maxBlogDiagrams)
	}
	// The third diagram must not appear.
	if strings.Contains(md, "three") {
		t.Error("a diagram beyond the cap was embedded")
	}
}

func TestBlogTagsDropStopwords(t *testing.T) {
	c := sampleContext()
	c.ContentIntelligence.SEOKeywords = []string{"an", "the", "project", "aws", "golang"}
	tags := blogTags(c)
	for _, junk := range []string{"an", "the", "project"} {
		for _, tag := range tags {
			if tag == junk {
				t.Errorf("stopword %q was not filtered from tags: %v", junk, tags)
			}
		}
	}
}
