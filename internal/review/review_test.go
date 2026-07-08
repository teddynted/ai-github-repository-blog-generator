package review

import (
	"context"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

func TestReviewSeparatesPassAndFail(t *testing.T) {
	good := generation.Content{Kind: generation.KindBlog, Markdown: "# Title\n\n" + strings.Repeat("body text ", 30)}
	empty := generation.Content{Kind: generation.KindReadme, Markdown: "  "}
	short := generation.Content{Kind: generation.KindDocs, Markdown: "# hi"}
	noHeading := generation.Content{Kind: generation.KindArchitecture, Markdown: strings.Repeat("no heading here ", 20)}

	passed, findings := Reviewer{}.Review(context.Background(), []generation.Content{good, empty, short, noHeading})

	if len(passed) != 1 || passed[0].Kind != generation.KindBlog {
		t.Errorf("passed = %+v", passed)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %v", len(findings), findings)
	}
	joined := strings.Join(findings, " | ")
	for _, want := range []string{"readme-improvements", "empty", "documentation", "too short", "architecture-summary", "no Markdown heading"} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings missing %q: %s", want, joined)
		}
	}
}

func TestReviewCustomMinLength(t *testing.T) {
	c := generation.Content{Kind: generation.KindBlog, Markdown: "# short but ok"}
	passed, findings := Reviewer{MinLength: 5}.Review(context.Background(), []generation.Content{c})
	if len(passed) != 1 || len(findings) != 0 {
		t.Errorf("custom min length: passed=%d findings=%v", len(passed), findings)
	}
}
