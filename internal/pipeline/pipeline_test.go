package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
)

type fakeProcessor struct {
	snap    processing.Snapshot
	err     error
	gotRepo string
	gotRef  string
}

func (f *fakeProcessor) Process(_ context.Context, repoFullName, ref string) (processing.Snapshot, error) {
	f.gotRepo, f.gotRef = repoFullName, ref
	return f.snap, f.err
}

type fakeGenerator struct {
	assets   []generation.Content
	err      error
	gotKinds []generation.Kind
}

func (f *fakeGenerator) GenerateAll(_ context.Context, _ processing.Snapshot, kinds ...generation.Kind) ([]generation.Content, error) {
	f.gotKinds = kinds
	return f.assets, f.err
}

type fakePublisher struct {
	published []generation.Content
	repo      string
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, repoFullName string, assets []generation.Content) error {
	f.repo = repoFullName
	f.published = append(f.published, assets...)
	return f.err
}

func snap() processing.Snapshot { return processing.Snapshot{RepoFullName: "acme/widget"} }

func TestRunProcessGeneratePublish(t *testing.T) {
	proc := &fakeProcessor{snap: snap()}
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog, Markdown: "# x"}}}
	pub := &fakePublisher{}
	p := &Pipeline{Processor: proc, Generator: gen, Publisher: pub}

	res, err := p.Run(context.Background(), Request{RepoFullName: "acme/widget", Ref: "refs/heads/main"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if proc.gotRepo != "acme/widget" || proc.gotRef != "refs/heads/main" {
		t.Errorf("processor got %s@%s", proc.gotRepo, proc.gotRef)
	}
	if res.Published != 1 || pub.repo != "acme/widget" || len(pub.published) != 1 {
		t.Errorf("res=%+v published=%d", res, len(pub.published))
	}
}

func TestRunDefaultsToBlogKind(t *testing.T) {
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog}}}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: &fakePublisher{}}
	if _, err := p.Run(context.Background(), Request{RepoFullName: "a/b"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(gen.gotKinds) != 1 || gen.gotKinds[0] != generation.KindBlog {
		t.Errorf("default kinds = %v", gen.gotKinds)
	}
}

func TestRunPublishesSuccessesOnPartialGenerationFailure(t *testing.T) {
	gen := &fakeGenerator{
		assets: []generation.Content{{Kind: generation.KindBlog, Markdown: "# x"}},
		err:    errors.New("release-notes failed"),
	}
	pub := &fakePublisher{}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b"})
	if err == nil {
		t.Fatal("expected the partial-failure error to surface")
	}
	if res.Published != 1 || len(pub.published) != 1 {
		t.Errorf("successful asset should still be published: res=%+v", res)
	}
}

func TestRunStopsOnProcessError(t *testing.T) {
	gen := &fakeGenerator{}
	pub := &fakePublisher{}
	p := &Pipeline{Processor: &fakeProcessor{err: errors.New("no access")}, Generator: gen, Publisher: pub}
	if _, err := p.Run(context.Background(), Request{RepoFullName: "a/b"}); err == nil {
		t.Fatal("expected process error")
	}
	if len(pub.published) != 0 {
		t.Error("nothing should be published when processing fails")
	}
}

func TestRunRequiresRepo(t *testing.T) {
	p := &Pipeline{Processor: &fakeProcessor{}, Generator: &fakeGenerator{}, Publisher: &fakePublisher{}}
	if _, err := p.Run(context.Background(), Request{}); err == nil {
		t.Error("expected error for missing repository")
	}
}

// Guards: the concrete implementations satisfy the pipeline ports.
var (
	_ Snapshotter      = (*processing.Processor)(nil)
	_ ContentGenerator = (*generation.Generator)(nil)
	_ Publisher        = (*publish.LogPublisher)(nil)
)
