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

type fakeMemory struct {
	already   bool
	lookupErr error
	recordErr error
	recorded  bool
	recCommit string
	recKinds  []string
}

func (f *fakeMemory) AlreadyPublished(_ context.Context, _, _ string) (bool, error) {
	return f.already, f.lookupErr
}
func (f *fakeMemory) RecordPublished(_ context.Context, _, commitSHA string, kinds []string) error {
	f.recorded, f.recCommit, f.recKinds = true, commitSHA, kinds
	return f.recordErr
}

func TestRunSkipsAlreadyPublishedCommit(t *testing.T) {
	gen := &fakeGenerator{}
	pub := &fakePublisher{}
	mem := &fakeMemory{already: true}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub, Memory: mem}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b", CommitSHA: "sha1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Skipped {
		t.Error("expected Skipped=true")
	}
	if len(gen.gotKinds) != 0 || len(pub.published) != 0 {
		t.Error("skipped run must not generate or publish")
	}
}

func TestRunRecordsMemoryAfterPublish(t *testing.T) {
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog}, {Kind: generation.KindReadme}}}
	mem := &fakeMemory{}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: &fakePublisher{}, Memory: mem}

	if _, err := p.Run(context.Background(), Request{RepoFullName: "a/b", CommitSHA: "sha1"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !mem.recorded || mem.recCommit != "sha1" || len(mem.recKinds) != 2 {
		t.Errorf("memory not recorded correctly: %+v", mem)
	}
}

func TestRunProceedsWhenMemoryLookupFails(t *testing.T) {
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog}}}
	pub := &fakePublisher{}
	mem := &fakeMemory{lookupErr: errors.New("disk error")}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub, Memory: mem}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b", CommitSHA: "sha1"})
	if err != nil {
		t.Fatalf("memory read failure must not fail the run: %v", err)
	}
	if res.Skipped || len(pub.published) != 1 {
		t.Errorf("run should proceed and publish despite memory error: %+v", res)
	}
}

type fakeReviewer struct {
	passed   []generation.Content
	findings []string
	got      []generation.Content
}

func (f *fakeReviewer) Review(_ context.Context, assets []generation.Content) ([]generation.Content, []string) {
	f.got = assets
	return f.passed, f.findings
}

type fakeApprover struct {
	approved bool
	err      error
	called   bool
}

func (f *fakeApprover) Approve(_ context.Context, _ string, _ []generation.Content) (bool, error) {
	f.called = true
	return f.approved, f.err
}

func TestRunPublishesOnlyReviewedAssets(t *testing.T) {
	blog := generation.Content{Kind: generation.KindBlog, Markdown: "# good"}
	readme := generation.Content{Kind: generation.KindReadme, Markdown: "bad"}
	gen := &fakeGenerator{assets: []generation.Content{blog, readme}}
	rev := &fakeReviewer{passed: []generation.Content{blog}, findings: []string{"readme-improvements: too short"}}
	pub := &fakePublisher{}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub, Reviewer: rev}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Published != 1 || len(pub.published) != 1 || pub.published[0].Kind != generation.KindBlog {
		t.Errorf("only reviewed assets should publish: %+v", pub.published)
	}
}

func TestRunHoldsWhenNotApproved(t *testing.T) {
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog, Markdown: "# x"}}}
	ap := &fakeApprover{approved: false}
	pub := &fakePublisher{}
	mem := &fakeMemory{}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub, Approver: ap, Memory: mem}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b", CommitSHA: "sha1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Held || res.Published != 0 {
		t.Errorf("expected held, not published: %+v", res)
	}
	if len(pub.published) != 0 {
		t.Error("held content must not be published")
	}
	if mem.recorded {
		t.Error("held content must not be recorded in memory")
	}
}

func TestRunPublishesWhenApproved(t *testing.T) {
	gen := &fakeGenerator{assets: []generation.Content{{Kind: generation.KindBlog, Markdown: "# x"}}}
	ap := &fakeApprover{approved: true}
	pub := &fakePublisher{}
	p := &Pipeline{Processor: &fakeProcessor{snap: snap()}, Generator: gen, Publisher: pub, Approver: ap}

	res, err := p.Run(context.Background(), Request{RepoFullName: "a/b"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ap.called || res.Published != 1 || len(pub.published) != 1 {
		t.Errorf("approved content should publish: called=%v res=%+v", ap.called, res)
	}
}

// Guards: the concrete implementations satisfy the pipeline ports.
var (
	_ Snapshotter      = (*processing.Processor)(nil)
	_ ContentGenerator = (*generation.Generator)(nil)
	_ Publisher        = (*publish.LogPublisher)(nil)
)
