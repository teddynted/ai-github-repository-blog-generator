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
		"Introduction", "Architecture and Design", "How to Use or Extend the Feature", "Conclusion",
		"Do NOT invent", "not available", "1,500–2,500 words",
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

func TestMetaDescriptionClamped(t *testing.T) {
	c := sampleContext()
	c.ContentIntelligence.Summary = strings.Repeat("word ", 60) // ~300 chars
	m := metaDescription(c)
	if len(m) > 160 {
		t.Errorf("meta length = %d, want <= 160", len(m))
	}
	if !strings.HasSuffix(m, "…") {
		t.Error("long meta should be truncated with an ellipsis")
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
