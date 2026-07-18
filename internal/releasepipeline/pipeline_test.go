package releasepipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/review"
)

// Compile-time proof that the real platform types satisfy the pipeline ports.
var (
	_ Generator = (*releasegen.Generator)(nil)
	_ Reviewer  = review.Reviewer{}
	_ Publisher = (*publish.FilePublisher)(nil)
)

type fakeModel struct{ reply func(string) (string, error) }

func (f fakeModel) Generate(_ context.Context, prompt string) (string, error) {
	if f.reply != nil {
		return f.reply(prompt)
	}
	return "## Heading\n\nSufficiently long generated body for this test.", nil
}

type fakeBuilder struct {
	ctx *rc.ReleaseContext
	err error
}

func (f *fakeBuilder) Build(context.Context, rc.Request) (*rc.ReleaseContext, error) {
	return f.ctx, f.err
}

type fakeReviewer struct{ passAll bool }

func (f fakeReviewer) Review(_ context.Context, assets []generation.Content) ([]generation.Content, []string) {
	if f.passAll {
		return assets, nil
	}
	issues := make([]string, len(assets))
	for i := range assets {
		issues[i] = "rejected"
	}
	return nil, issues
}

type fakePublisher struct {
	repo   string
	assets []generation.Content
	err    error
}

func (f *fakePublisher) Publish(_ context.Context, repo string, assets []generation.Content) error {
	f.repo, f.assets = repo, assets
	return f.err
}

type fakeNotifier struct {
	subject string
	body    string
	calls   int
}

func (f *fakeNotifier) Notify(_ context.Context, subject, body string) error {
	f.subject, f.body = subject, body
	f.calls++
	return nil
}

func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		ContextID:   "ctx-1",
		Repository:  rc.Repository{Name: "widget", FullName: "acme/widget"},
		Release:     rc.Release{Tag: "v0.2.0", Summary: "Release v0.2.0."},
		CommitStats: rc.CommitStats{Analyzed: 3, ByCategory: map[string]int{"Features": 2, "Bug Fixes": 1}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary: "widget v0.2.0 delivers 3 changes.", BlogTitles: []string{"Inside widget v0.2.0"},
		},
	}
}

func newPipeline(reviewer Reviewer, pub Publisher, note Notifier) *Pipeline {
	return &Pipeline{
		Builder:   &fakeBuilder{ctx: sampleContext()},
		Generator: &releasegen.Generator{Model: fakeModel{}},
		Reviewer:  reviewer,
		Publisher: pub,
		Notifier:  note,
	}
}

func TestPipelineHappyPath(t *testing.T) {
	pub := &fakePublisher{}
	note := &fakeNotifier{}
	p := newPipeline(fakeReviewer{passAll: true}, pub, note)

	res, err := p.Run(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Generated != 2 || res.Published != 2 || res.Rejected != 0 {
		t.Errorf("result = %+v", res)
	}
	if pub.repo != "acme/widget" || len(pub.assets) != 2 {
		t.Errorf("published = %s / %d assets", pub.repo, len(pub.assets))
	}
	// The blog asset must carry the assembled front matter + title.
	var haveBlog bool
	for _, a := range pub.assets {
		if a.Kind == "blog" {
			haveBlog = true
			if !strings.Contains(a.Markdown, "# Inside widget v0.2.0") {
				t.Errorf("blog markdown not assembled: %q", a.Markdown[:min(80, len(a.Markdown))])
			}
		}
	}
	if !haveBlog {
		t.Error("blog asset missing")
	}
	if note.calls != 1 || !strings.Contains(note.subject, "published") {
		t.Errorf("notify = %d / %q", note.calls, note.subject)
	}
}

func TestPipelineAllRejected(t *testing.T) {
	note := &fakeNotifier{}
	p := newPipeline(fakeReviewer{passAll: false}, &fakePublisher{}, note)
	res, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"})
	if err == nil {
		t.Fatal("expected error when all assets fail review")
	}
	if res.Published != 0 || res.Rejected != 2 {
		t.Errorf("result = %+v", res)
	}
	if !strings.Contains(note.subject, "FAILED") {
		t.Errorf("notify subject = %q", note.subject)
	}
}

func TestPipelineGeneratorFailsAll(t *testing.T) {
	p := &Pipeline{
		Builder:   &fakeBuilder{ctx: sampleContext()},
		Generator: &releasegen.Generator{Model: fakeModel{reply: func(string) (string, error) { return "", errors.New("model down") }}},
		Reviewer:  fakeReviewer{passAll: true},
		Publisher: &fakePublisher{},
	}
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"}); err == nil {
		t.Error("expected error when no content is generated")
	}
}

func TestPipelinePublishError(t *testing.T) {
	p := newPipeline(fakeReviewer{passAll: true}, &fakePublisher{err: errors.New("disk full")}, &fakeNotifier{})
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"}); err == nil {
		t.Error("expected publish error to propagate")
	}
}

func TestPipelineMissingStage(t *testing.T) {
	p := &Pipeline{Builder: &fakeBuilder{ctx: sampleContext()}}
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"}); err == nil {
		t.Error("expected error for missing stages")
	}
}
