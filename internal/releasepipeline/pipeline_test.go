package releasepipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
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

func TestPipelineFullSuite(t *testing.T) {
	pub := &fakePublisher{}
	note := &fakeNotifier{}
	// A model returning a multi-section blog body so the suite has scenes to work with.
	model := fakeModel{reply: func(string) (string, error) {
		return "## Overview\n\nWhat changed and why it matters in this release.\n\n## How it works\n\nThe implementation, grounded in the release context.\n", nil
	}}
	p := &Pipeline{
		Builder:   &fakeBuilder{ctx: sampleContext()},
		Generator: &releasegen.Generator{Model: model},
		Suite:     &contentsuite.Orchestrator{Model: model},
		Reviewer:  fakeReviewer{passAll: true},
		Publisher: pub,
		Notifier:  note,
	}
	res, err := p.Run(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The full suite produces many more artifacts than the 2-format default path.
	if res.Generated < 6 {
		t.Fatalf("full suite should generate many artifacts, got %d", res.Generated)
	}
	if res.Published != res.Generated {
		t.Errorf("all passing review should publish: generated=%d published=%d", res.Generated, res.Published)
	}
	// The published set spans multimedia stages — proof the suite flowed through
	// the SAME review/publish stages, not just the written formats.
	kinds := map[generation.Kind]bool{}
	for _, a := range pub.assets {
		kinds[a.Kind] = true
	}
	for _, want := range []generation.Kind{"blog", "storyboard", "youtube", "linkedin"} {
		if !kinds[want] {
			t.Errorf("published set missing %q", want)
		}
	}
}

func TestPipelineAllRejected(t *testing.T) {
	note := &fakeNotifier{}
	p := newPipeline(fakeReviewer{passAll: false}, &fakePublisher{}, note)
	res, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1.0.0"})
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
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1.0.0"}); err == nil {
		t.Error("expected error when no content is generated")
	}
}

func TestPipelinePublishError(t *testing.T) {
	p := newPipeline(fakeReviewer{passAll: true}, &fakePublisher{err: errors.New("disk full")}, &fakeNotifier{})
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1.0.0"}); err == nil {
		t.Error("expected publish error to propagate")
	}
}

func TestPipelineRejectsInvalidSemVer(t *testing.T) {
	pub := &fakePublisher{}
	p := newPipeline(fakeReviewer{passAll: true}, pub, &fakeNotifier{})
	// A non-SemVer tag must fail the first gate, before any context build/generation.
	_, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "not-a-version"})
	if !errors.Is(err, ErrInvalidVersion) {
		t.Fatalf("expected ErrInvalidVersion, got %v", err)
	}
	if len(pub.assets) != 0 {
		t.Error("nothing should be generated/published for an invalid version")
	}
	// A valid SemVer tag (with or without a leading v) passes the gate.
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v0.2.0"}); err != nil {
		t.Errorf("valid SemVer should pass the gate: %v", err)
	}
}

func TestPipelineMissingStage(t *testing.T) {
	p := &Pipeline{Builder: &fakeBuilder{ctx: sampleContext()}}
	if _, err := p.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1.0.0"}); err == nil {
		t.Error("expected error for missing stages")
	}
}

// --- Stage 2: engineering analysis ---

type fakeAnalyzer struct {
	ec     *rc.EngineeringContext
	err    error
	called bool
}

func (f *fakeAnalyzer) Analyze(_ context.Context, _ *rc.ReleaseContext) (*rc.EngineeringContext, error) {
	f.called = true
	return f.ec, f.err
}

// The analyzer must run and its output must reach generation grounding.
func TestPipelineRunsAnalyzerAndGroundsGeneration(t *testing.T) {
	an := &fakeAnalyzer{ec: &rc.EngineeringContext{
		Problem:              "webhook spikes overwhelmed the worker",
		EngineeringDecisions: []rc.EngineeringDecision{{Decision: "buffer via SQS", Rationale: "decouple ingest from compute"}},
	}}
	var sawAnalysis bool
	p := &Pipeline{
		Builder:  &fakeBuilder{ctx: sampleContext()},
		Analyzer: an,
		Generator: &releasegen.Generator{Model: fakeModel{reply: func(prompt string) (string, error) {
			if strings.Contains(prompt, "ENGINEERING ANALYSIS") && strings.Contains(prompt, "buffer via SQS") {
				sawAnalysis = true
			}
			return "# Post\n\nBody.", nil
		}}},
		Reviewer:  fakeReviewer{passAll: true},
		Publisher: &fakePublisher{},
	}
	if _, err := p.Run(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !an.called {
		t.Error("analyzer was not called")
	}
	if !sawAnalysis {
		t.Error("engineering analysis did not reach the generation prompt")
	}
}

// A failing analyzer must NOT fail the run — generation proceeds on facts alone.
func TestPipelineAnalyzerFailureIsNonFatal(t *testing.T) {
	p := &Pipeline{
		Builder:   &fakeBuilder{ctx: sampleContext()},
		Analyzer:  &fakeAnalyzer{err: errors.New("model down")},
		Generator: &releasegen.Generator{Model: fakeModel{}},
		Reviewer:  fakeReviewer{passAll: true},
		Publisher: &fakePublisher{},
	}
	res, err := p.Run(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatalf("analyzer failure should be non-fatal, got: %v", err)
	}
	if res.Published == 0 {
		t.Error("run should still publish despite analysis failure")
	}
}
