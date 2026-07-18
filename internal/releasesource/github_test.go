package releasesource

import (
	"context"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

type fakeGH struct {
	detail       github.RepositoryDetail
	release      github.ReleaseDetail
	releases     []github.ReleaseRef
	commits      []github.CommitInfo
	files        []github.ChangedFile
	tree         []github.TreeEntry
	contents     map[string]string
	compareCalls int
	contentCalls int
	lastToken    string
}

func (f *fakeGH) GetRepositoryDetail(_ context.Context, _, _, token string) (github.RepositoryDetail, error) {
	f.lastToken = token
	return f.detail, nil
}
func (f *fakeGH) GetReleaseByTag(context.Context, string, string, string, string) (github.ReleaseDetail, error) {
	return f.release, nil
}
func (f *fakeGH) ListReleases(context.Context, string, string, string) ([]github.ReleaseRef, error) {
	return f.releases, nil
}
func (f *fakeGH) CompareCommits(context.Context, string, string, string, string, string) ([]github.CommitInfo, []github.ChangedFile, error) {
	f.compareCalls++
	return f.commits, f.files, nil
}
func (f *fakeGH) GetTree(context.Context, string, string, string, string) ([]github.TreeEntry, error) {
	return f.tree, nil
}
func (f *fakeGH) GetFileContent(_ context.Context, _, _, _, filePath, _ string) (string, error) {
	f.contentCalls++
	if c, ok := f.contents[filePath]; ok {
		return c, nil
	}
	return "", apperror.New(apperror.CodeNotFound, "missing")
}

func sampleGH() *fakeGH {
	return &fakeGH{
		detail: github.RepositoryDetail{FullName: "acme/widget", Description: "d", Language: "Go", License: "MIT", Visibility: "public", DefaultBranch: "main"},
		release: github.ReleaseDetail{Tag: "v0.2.0", Name: "v0.2.0", Body: "notes", AuthorLogin: "teddy", PublishedAt: "2026-07-20T10:00:00Z",
			Assets: []github.ReleaseAssetDetail{{Name: "bin", Size: 10}}},
		releases: []github.ReleaseRef{
			{Tag: "v0.2.0", PublishedAt: "2026-07-20T10:00:00Z"},
			{Tag: "v0.1.0", PublishedAt: "2026-07-16T10:00:00Z"},
		},
		commits: []github.CommitInfo{{SHA: "a1", Message: "feat: x", AuthorName: "T", Parents: 1}},
		files:   []github.ChangedFile{{Path: "internal/x.go", Status: "modified", Additions: 3}},
		tree: []github.TreeEntry{
			{Path: "README.md", Type: "blob", Size: 100},
			{Path: "internal/x.go", Type: "blob", Size: 50},
			{Path: "vendor/dep/y.go", Type: "blob", Size: 50},
			{Path: "docs", Type: "tree"},
		},
		contents: map[string]string{"README.md": "# Widget"},
	}
}

func TestGitHubSourceMapping(t *testing.T) {
	gh := sampleGH()
	src := New(gh, "tok")
	req := rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"}
	ctx := context.Background()

	meta, err := src.RepositoryMeta(ctx, req)
	if err != nil || meta.FullName != "acme/widget" || meta.License != "MIT" {
		t.Fatalf("meta = %+v err=%v", meta, err)
	}

	rel, err := src.ReleaseByTag(ctx, req)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if rel.Tag != "v0.2.0" || rel.PreviousTag != "v0.1.0" || rel.Author != "teddy" || len(rel.Assets) != 1 {
		t.Errorf("release = %+v", rel)
	}

	commits, err := src.CommitsBetween(ctx, req, rel.PreviousTag)
	if err != nil || len(commits) != 1 || commits[0].SHA != "a1" {
		t.Errorf("commits = %+v err=%v", commits, err)
	}
	changed, err := src.ChangedFilesBetween(ctx, req, rel.PreviousTag)
	if err != nil || len(changed) != 1 {
		t.Errorf("changed = %+v err=%v", changed, err)
	}
	// CommitsBetween + ChangedFilesBetween must share ONE compare call.
	if gh.compareCalls != 1 {
		t.Errorf("compareCalls = %d, want 1 (memoized)", gh.compareCalls)
	}

	files, err := src.Files(ctx, req)
	if err != nil {
		t.Fatalf("files: %v", err)
	}
	// vendored file dropped; tree node dropped; README + internal/x.go kept.
	var haveReadme, haveVendor bool
	for _, f := range files {
		if f.Path == "README.md" {
			haveReadme = true
			if f.Content != "# Widget" {
				t.Errorf("README content not fetched: %q", f.Content)
			}
		}
		if f.Path == "vendor/dep/y.go" {
			haveVendor = true
		}
	}
	if !haveReadme {
		t.Error("README.md missing from inventory")
	}
	if haveVendor {
		t.Error("vendored file should be skipped")
	}
	// Content fetched only for README (a .go file is inventory-only).
	if gh.contentCalls != 1 {
		t.Errorf("contentCalls = %d, want 1", gh.contentCalls)
	}
}

func TestTokenForResolvesPerRepoAndMemoizes(t *testing.T) {
	gh := sampleGH()
	src := New(gh, "static-fallback")
	calls := 0
	src.TokenFor = func(_ context.Context, repoFullName string) (string, error) {
		calls++
		if repoFullName != "acme/widget" {
			t.Errorf("unexpected repo %q", repoFullName)
		}
		return "repo-pat", nil
	}
	req := rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v0.2.0"}
	ctx := context.Background()

	if _, err := src.RepositoryMeta(ctx, req); err != nil {
		t.Fatal(err)
	}
	if gh.lastToken != "repo-pat" {
		t.Errorf("token used = %q, want the resolved repo PAT", gh.lastToken)
	}
	// A second call for the same repo must reuse the memoized token.
	if _, err := src.ReleaseByTag(ctx, req); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("TokenFor called %d times, want 1 (memoized)", calls)
	}
}

func TestTokenForFallsBackOnErrorOrEmpty(t *testing.T) {
	for name, fn := range map[string]func(context.Context, string) (string, error){
		"error": func(context.Context, string) (string, error) { return "", context.Canceled },
		"empty": func(context.Context, string) (string, error) { return "", nil },
	} {
		t.Run(name, func(t *testing.T) {
			gh := sampleGH()
			src := New(gh, "static-fallback")
			src.TokenFor = fn
			if _, err := src.RepositoryMeta(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v1"}); err != nil {
				t.Fatal(err)
			}
			if gh.lastToken != "static-fallback" {
				t.Errorf("token = %q, want the static fallback", gh.lastToken)
			}
		})
	}
}

func TestNoTokenForUsesStatic(t *testing.T) {
	gh := sampleGH()
	src := New(gh, "static")
	if _, err := src.RepositoryMeta(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v1"}); err != nil {
		t.Fatal(err)
	}
	if gh.lastToken != "static" {
		t.Errorf("token = %q, want static", gh.lastToken)
	}
}

func TestGitHubSourceFirstReleaseHasNoPrevious(t *testing.T) {
	gh := sampleGH()
	gh.releases = []github.ReleaseRef{{Tag: "v0.2.0", PublishedAt: "2026-07-20T10:00:00Z"}}
	src := New(gh, "")
	rel, err := src.ReleaseByTag(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if rel.PreviousTag != "" {
		t.Errorf("previousTag = %q, want empty", rel.PreviousTag)
	}
}
