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
	gotPrompt string
	out       string
	err       error
}

func (f *fakeModel) Generate(_ context.Context, prompt string) (string, error) {
	f.gotPrompt = prompt
	return f.out, f.err
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
	if c.Kind != "blog" || !strings.HasPrefix(c.Markdown, "# Widget deep dive") {
		t.Errorf("content = %+v", c)
	}

	// The prompt should include the repo, README, docs, and commit subject
	// (but only the first line of the commit message).
	p := m.gotPrompt
	for _, want := range []string{"acme/widget", "# Widget", "docs/arch.md", "abcdef1", "blog: add feature"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "body") {
		t.Error("prompt should include only the commit subject, not the body")
	}
}

func TestBlogPostRequiresRepo(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{out: "x"}}).BlogPost(context.Background(), processing.Snapshot{})
	if apperror.CodeOf(err) != apperror.CodeInvalidInput {
		t.Errorf("code = %s, want invalid_input", apperror.CodeOf(err))
	}
}

func TestBlogPostRejectsEmptyOutput(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{out: "   "}}).BlogPost(context.Background(), sampleSnapshot())
	if apperror.CodeOf(err) != apperror.CodeUpstream {
		t.Errorf("code = %s, want upstream", apperror.CodeOf(err))
	}
}

func TestBlogPostPropagatesModelError(t *testing.T) {
	_, err := (&Generator{Model: &fakeModel{err: errors.New("ollama down")}}).BlogPost(context.Background(), sampleSnapshot())
	if err == nil {
		t.Error("expected model error to propagate")
	}
}

// Guard: the concrete ollama.Client satisfies the Model port.
var _ Model = (*ollama.Client)(nil)
