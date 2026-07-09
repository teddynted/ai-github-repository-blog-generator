package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/memory"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/pipeline"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/reposource"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/review"
)

// stubCloner returns a pre-created local repo instead of cloning.
type stubCloner struct{ dir string }

func (s stubCloner) Clone(_ context.Context, _, _ string) (string, error) { return s.dir, nil }

// echoModel returns valid Markdown so review passes.
type echoModel struct{}

func (echoModel) Generate(_ context.Context, _ string) (string, error) {
	return "# Generated\n\n" + strings.Repeat("real content here. ", 20), nil
}

func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := r.Worktree()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# Widget\nDoes widget things.")
	write("main.go", "package main")
	write("go.mod", "module widget")
	write("docs/design.md", "layered design")
	_ = wt.AddGlob(".")
	if _, err := wt.Commit("blog: initial", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@x", When: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestPipelineEndToEnd wires the real adapters (FS retrieval, analysis, review,
// file publish, memory) through the pipeline with only the LLM stubbed.
func TestPipelineEndToEnd(t *testing.T) {
	repoDir := makeRepo(t)
	outDir := t.TempDir()
	memDir := t.TempDir()

	processor := &processing.Processor{
		Cloner:      stubCloner{dir: repoDir},
		Readme:      reposource.FSReadme{},
		Docs:        reposource.FSDocs{},
		Commits:     reposource.GitCommits{},
		Analyzer:    reposource.FSAnalyzer{},
		CommitLimit: 10,
	}
	kinds := []generation.Kind{
		generation.KindBlog, generation.KindReadme, generation.KindDocs,
		generation.KindArchitecture, generation.KindReleaseNotes,
	}
	mem := &memory.Store{Dir: memDir, Now: func() time.Time { return time.Unix(1700000000, 0) }}
	pipe := &pipeline.Pipeline{
		Processor: processor,
		Generator: &generation.Generator{Model: echoModel{}},
		Publisher: &publish.FilePublisher{Dir: outDir, Now: func() time.Time { return time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC) }},
		Memory:    mem,
		Reviewer:  review.Reviewer{},
		Kinds:     kinds,
	}

	// First run: full pipeline produces and publishes every kind.
	res, err := pipe.Run(context.Background(), pipeline.Request{RepoFullName: "acme/widget", CommitSHA: "sha1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Published != len(kinds) {
		t.Fatalf("published %d, want %d", res.Published, len(kinds))
	}
	base := filepath.Join(outDir, "acme", "widget", "2026-07-09")
	for _, k := range kinds {
		if _, err := os.Stat(filepath.Join(base, string(k)+".md")); err != nil {
			t.Errorf("missing published asset %s.md: %v", k, err)
		}
	}
	if done, _ := mem.AlreadyPublished(context.Background(), "acme/widget", "sha1"); !done {
		t.Error("memory should record the published commit")
	}

	// Second run for the same commit: skipped via Repository Memory.
	res2, err := pipe.Run(context.Background(), pipeline.Request{RepoFullName: "acme/widget", CommitSHA: "sha1"})
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if !res2.Skipped || res2.Published != 0 {
		t.Errorf("second run should be skipped: %+v", res2)
	}
}
