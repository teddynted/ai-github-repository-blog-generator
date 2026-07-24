package reposource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// initRepo builds a git working repo at dir with fixture files and two commits.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	wt, err := r.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	writeFile(t, filepath.Join(dir, "README.md"), "# Widget\nDoes widget things.")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "doc a")
	writeFile(t, filepath.Join(dir, "docs", "sub", "b.md"), "doc b")
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignored non-markdown")

	commit(t, wt, "initial commit")
	writeFile(t, filepath.Join(dir, "README.md"), "# Widget\nUpdated.")
	commit(t, wt, "blog: update readme")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, wt *git.Worktree, msg string) {
	t.Helper()
	if err := wt.AddGlob("."); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "Tester", Email: "t@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestFSReadme(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	got, err := FSReadme{}.Readme(context.Background(), dir)
	if err != nil {
		t.Fatalf("Readme: %v", err)
	}
	if got != "# Widget\nUpdated." {
		t.Errorf("readme = %q", got)
	}
}

func TestFSReadmeMissing(t *testing.T) {
	got, err := FSReadme{}.Readme(context.Background(), t.TempDir())
	if err != nil || got != "" {
		t.Errorf("empty repo readme = %q err=%v", got, err)
	}
}

func TestFSDocs(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	docs, err := FSDocs{}.Docs(context.Background(), dir)
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2: %+v", len(docs), docs)
	}
	// Sorted by path, and non-markdown excluded.
	if docs[0].Path != "docs/a.md" || docs[1].Path != "docs/sub/b.md" {
		t.Errorf("doc paths = %s, %s", docs[0].Path, docs[1].Path)
	}
	if docs[0].Content != "doc a" {
		t.Errorf("doc content = %q", docs[0].Content)
	}
}

func TestGitCommits(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commits, err := GitCommits{}.Commits(context.Background(), dir, 10)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	// Newest first.
	if commits[0].Message != "blog: update readme" || commits[0].Author != "Tester" {
		t.Errorf("latest commit = %+v", commits[0])
	}
}

func TestGitCommitsRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commits, err := GitCommits{}.Commits(context.Background(), dir, 1)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 1 {
		t.Errorf("limit not applied: got %d", len(commits))
	}
}

type fakeTokens struct{ tok string }

func (f fakeTokens) Token(context.Context, string) (string, error) { return f.tok, nil }

func TestGitClonerClonesLocalRepo(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "owner", "name.git")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, src)

	work := t.TempDir()
	cl := &GitCloner{Tokens: fakeTokens{}, BaseURL: base, WorkDir: work}
	dir, err := cl.Clone(context.Background(), "owner/name", "")
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Errorf("cloned working copy missing README: %v", err)
	}
}

// A clone at the default depth must fetch enough history for a multi-commit
// read; the old Depth:1 clone failed Commits with "object not found".
func TestGitClonerDefaultDepthAllowsCommitRead(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "owner", "name.git")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, src) // 2 commits
	work := t.TempDir()
	cl := &GitCloner{Tokens: fakeTokens{}, BaseURL: base, WorkDir: work} // default depth
	dir, err := cl.Clone(context.Background(), "owner/name", "")
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	commits, err := GitCommits{}.Commits(context.Background(), dir, 10)
	if err != nil {
		t.Fatalf("Commits after clone: %v", err)
	}
	if len(commits) != 2 {
		t.Errorf("got %d commits, want 2", len(commits))
	}
}

// Reading commits from a shallow clone whose history is shorter than the limit
// must return the available commits rather than error at the shallow boundary.
func TestGitCommitsToleratesShallowBoundary(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "owner", "name.git")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, src) // 2 commits
	work := t.TempDir()
	cl := &GitCloner{Tokens: fakeTokens{}, BaseURL: base, WorkDir: work, Depth: 1} // force shallow
	dir, err := cl.Clone(context.Background(), "owner/name", "")
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	commits, err := GitCommits{}.Commits(context.Background(), dir, 10)
	if err != nil {
		t.Fatalf("shallow boundary not tolerated: %v", err)
	}
	if len(commits) < 1 {
		t.Errorf("want >=1 commit from a shallow clone, got %d", len(commits))
	}
}

func TestFSAnalyzer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module x")
	writeFile(t, filepath.Join(dir, "web", "app.ts"), "export {}")
	writeFile(t, filepath.Join(dir, "web", "package.json"), "{}")
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM scratch")
	writeFile(t, filepath.Join(dir, "infra", "main.tf"), "resource {}")
	writeFile(t, filepath.Join(dir, "infra", "stack.yaml"), "AWSTemplateFormatVersion: \"2010-09-09\"\nResources: {}")
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "on: push")
	// ignored dir content must not be scanned
	writeFile(t, filepath.Join(dir, "node_modules", "junk.rb"), "ruby")

	a, err := FSAnalyzer{}.Analyze(context.Background(), dir)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	has := func(list []string, v string) bool {
		for _, x := range list {
			if x == v {
				return true
			}
		}
		return false
	}
	if !has(a.Languages, "Go") || !has(a.Languages, "TypeScript") {
		t.Errorf("languages = %v", a.Languages)
	}
	if has(a.Languages, "Ruby") {
		t.Error("node_modules should be ignored (no Ruby)")
	}
	if !has(a.PackageManagers, "Go modules") || !has(a.PackageManagers, "npm/Node") {
		t.Errorf("package managers = %v", a.PackageManagers)
	}
	if !has(a.Containers, "Docker") {
		t.Errorf("containers = %v", a.Containers)
	}
	if !has(a.IaC, "Terraform") || !has(a.IaC, "CloudFormation") {
		t.Errorf("iac = %v", a.IaC)
	}
	if !has(a.CICD, "GitHub Actions") {
		t.Errorf("cicd = %v", a.CICD)
	}
}

// --- MetaTokenSource ---

type fakeMeta struct {
	r  repo.Repository
	ok bool
}

func (f fakeMeta) Get(context.Context, string) (repo.Repository, bool, error) {
	return f.r, f.ok, nil
}

type fakePAT struct{ ref string }

func (f *fakePAT) PAT(_ context.Context, ref string) (string, error) {
	f.ref = ref
	return "github_pat_secret", nil
}

func TestMetaTokenSource(t *testing.T) {
	pg := &fakePAT{}
	ts := &MetaTokenSource{
		Meta:    fakeMeta{r: repo.Repository{RepoFullName: "acme/widget"}, ok: true},
		Secrets: pg,
	}
	tok, err := ts.Token(context.Background(), "acme/widget")
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	// The PAT is looked up by the repo full name (the shared-secret key).
	if tok != "github_pat_secret" || pg.ref != "acme/widget" {
		t.Errorf("token=%q ref=%q", tok, pg.ref)
	}
}

func TestMetaTokenSourceUnregistered(t *testing.T) {
	ts := &MetaTokenSource{Meta: fakeMeta{ok: false}, Secrets: &fakePAT{}}
	if _, err := ts.Token(context.Background(), "a/b"); apperror.CodeOf(err) != apperror.CodeNotFound {
		t.Errorf("code = %s, want not_found", apperror.CodeOf(err))
	}
}
