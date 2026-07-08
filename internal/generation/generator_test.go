package generation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

type fakeModel struct {
	gotPrompts []string
	out        string
	// failOn returns an error when the prompt contains this substring.
	failOn string
}

func (f *fakeModel) Generate(_ context.Context, prompt string) (string, error) {
	f.gotPrompts = append(f.gotPrompts, prompt)
	if f.failOn != "" && strings.Contains(prompt, f.failOn) {
		return "", errors.New("model error")
	}
	if f.out == "" {
		return "# Generated\n\ncontent", nil
	}
	return f.out, nil
}

func sampleSnapshot() processing.Snapshot {
	return processing.Snapshot{
		RepoFullName: "acme/widget",
		Ref:          "refs/heads/main",
		Readme:       "# Widget\nDoes widget things.",
		Docs:         []processing.Document{{Path: "docs/arch.md", Content: "layered"}},
		Commits:      []processing.Commit{{SHA: "abcdef1234", Message: "blog: add feature\n\nbody"}},
	}
}

func TestBlogPostBuildsPromptAndReturnsContent(t *testing.T) {
	m := &fakeModel{out: "# Widget deep dive\n\n...content..."}
	c, err := (&Generator{Model: m}).BlogPost(context.Background(), sampleSnapshot())
	if err != nil {
		t.Fatalf("BlogPost: %v", err)
	}
	if c.Kind != KindBlog || !strings.HasPrefix(c.Markdown, "# Widget deep dive") {
		t.Errorf("content = %+v", c)
	}

	p := m.gotPrompts[0]
	for _, want := range []string{"acme/widget", "# Widget", "docs/arch.md", "abcdef1", "blog: add feature", "blog post"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "\nbody") {
		t.Error("prompt should include only the commit subject, not the body")
	}
}

func TestGenerateEachKindHasDistinctPrompt(t *testing.T) {
	markers := map[Kind]string{
		KindBlog:         "blog post",
		KindReadme:       "README",
		KindDocs:         "documentation",
		KindArchitecture: "architecture",
		KindReleaseNotes: "release notes",
	}
	for kind, marker := range markers {
		m := &fakeModel{}
		c, err := (&Generator{Model: m}).Generate(context.Background(), kind, sampleSnapshot())
		if err != nil {
			t.Fatalf("Generate(%s): %v", kind, err)
		}
		if c.Kind != kind {
			t.Errorf("kind = %s, want %s", c.Kind, kind)
		}
		if !strings.Contains(strings.ToLower(m.gotPrompts[0]), strings.ToLower(marker)) {
			t.Errorf("prompt for %s should mention %q", kind, marker)
		}
	}
}

func TestGenerateRejectsUnknownKind(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{}}).Generate(context.Background(), Kind("nope"), sampleSnapshot())
	if apperror.CodeOf(err) != apperror.CodeInvalidInput {
		t.Errorf("code = %s, want invalid_input", apperror.CodeOf(err))
	}
}

func TestGenerateRequiresRepo(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{}}).Generate(context.Background(), KindBlog, processing.Snapshot{})
	if apperror.CodeOf(err) != apperror.CodeInvalidInput {
		t.Errorf("code = %s, want invalid_input", apperror.CodeOf(err))
	}
}

func TestGenerateRejectsEmptyOutput(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{out: "   "}}).Generate(context.Background(), KindBlog, sampleSnapshot())
	if apperror.CodeOf(err) != apperror.CodeUpstream {
		t.Errorf("code = %s, want upstream", apperror.CodeOf(err))
	}
}

func TestGenerateAllProducesEveryKind(t *testing.T) {
	kinds := []Kind{KindBlog, KindReadme, KindDocs, KindArchitecture, KindReleaseNotes}
	out, err := (&Generator{Model: &fakeModel{}}).GenerateAll(context.Background(), sampleSnapshot(), kinds...)
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}
	if len(out) != len(kinds) {
		t.Fatalf("got %d assets, want %d", len(out), len(kinds))
	}
}

func TestGenerateAllContinuesPastFailures(t *testing.T) {
	// Fail only the release-notes prompt; the rest should still be produced.
	m := &fakeModel{failOn: "release notes"}
	out, err := (&Generator{Model: m}).GenerateAll(context.Background(), sampleSnapshot(),
		KindBlog, KindReadme, KindReleaseNotes)
	if err == nil {
		t.Fatal("expected an aggregated error for the failed kind")
	}
	if len(out) != 2 {
		t.Errorf("expected 2 successful assets, got %d", len(out))
	}
	if !strings.Contains(err.Error(), string(KindReleaseNotes)) {
		t.Errorf("error should name the failed kind: %v", err)
	}
}

func TestPromptBudgetTruncatesLargeContext(t *testing.T) {
	huge := strings.Repeat("x", 100000)
	snap := processing.Snapshot{RepoFullName: "acme/widget", Readme: huge}
	m := &fakeModel{}
	g := &Generator{Model: m, MaxPromptBytes: 2000}

	if _, err := g.Generate(context.Background(), KindBlog, snap); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	p := m.gotPrompts[0]
	if len(p) > 2000 {
		t.Errorf("prompt exceeded budget: %d bytes", len(p))
	}
	// The instruction, repo name, closing, and truncation marker survive.
	for _, want := range []string{"blog post", "acme/widget", "starting with a top-level title", "context truncated"} {
		if !strings.Contains(p, want) {
			t.Errorf("budgeted prompt missing %q", want)
		}
	}
}

func TestPromptNotTruncatedWhenSmall(t *testing.T) {
	snap := sampleSnapshot()
	m := &fakeModel{}
	g := &Generator{Model: m} // default budget
	if _, err := g.Generate(context.Background(), KindBlog, snap); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(m.gotPrompts[0], "context truncated") {
		t.Error("small prompt should not be truncated")
	}
}

// Guard: the concrete ollama.Client satisfies the Model port.
var _ Model = (*ollama.Client)(nil)
