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

func TestBlogPromptStructureAndGrounding(t *testing.T) {
	fm := &fakeModel{}
	_, _ = (&Generator{Model: fm}).Blog(context.Background(), blogContext())
	p := fm.prompts[0]
	for _, want := range []string{
		"Introduction", "Architecture", "Engineering Decisions", "Tradeoffs",
		"How Developers Can Use or Extend It", "What's Next", "Conclusion",
		"Never invent", "omit it silently", "Do NOT narrate the changelog", "1,500–2,500 words",
		"acme/widget", "v0.2.0", "add release context builder", // grounding
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	// The model must not be asked to write front matter or the H1.
	if !strings.Contains(p, "Do NOT write YAML front matter") {
		t.Error("prompt should forbid front matter / H1 in the body")
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
