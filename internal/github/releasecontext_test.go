package github

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

func TestGetReleaseByTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widget/releases/tags/v0.2.0" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","name":"Release 0.2.0","body":"notes","published_at":"2026-07-20T10:00:00Z","html_url":"https://x","prerelease":false,"author":{"login":"teddy"},"assets":[{"name":"bin","size":42,"content_type":"application/octet-stream","browser_download_url":"https://d/bin"}]}`))
	}))
	defer srv.Close()

	rel, err := New(WithBaseURL(srv.URL)).GetReleaseByTag(context.Background(), "acme", "widget", "tok", "v0.2.0")
	if err != nil {
		t.Fatalf("GetReleaseByTag: %v", err)
	}
	if rel.Tag != "v0.2.0" || rel.Name != "Release 0.2.0" || rel.AuthorLogin != "teddy" {
		t.Errorf("release = %+v", rel)
	}
	if len(rel.Assets) != 1 || rel.Assets[0].Size != 42 {
		t.Errorf("assets = %+v", rel.Assets)
	}
}

func TestCompareCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/compare/v0.1.0...v0.2.0") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"commits":[{"sha":"a1","commit":{"message":"feat: add x\n\nbody","author":{"name":"Teddy","date":"2026-07-18T00:00:00Z"}},"parents":[{"sha":"p1"}]},{"sha":"m1","commit":{"message":"Merge pull request #1"},"parents":[{"sha":"p1"},{"sha":"p2"}]}],"files":[{"filename":"internal/x.go","status":"modified","additions":10,"deletions":2}]}`))
	}))
	defer srv.Close()

	commits, files, err := New(WithBaseURL(srv.URL)).CompareCommits(context.Background(), "acme", "widget", "tok", "v0.1.0", "v0.2.0")
	if err != nil {
		t.Fatalf("CompareCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("commits = %d", len(commits))
	}
	if commits[0].Message != "feat: add x" || commits[0].Parents != 1 {
		t.Errorf("commit[0] = %+v (message must be first line)", commits[0])
	}
	if commits[1].Parents != 2 {
		t.Errorf("merge parents = %d, want 2", commits[1].Parents)
	}
	if len(files) != 1 || files[0].Additions != 10 {
		t.Errorf("files = %+v", files)
	}
}

func TestCompareCommitsEmptyBase(t *testing.T) {
	// No base (first release) returns no error and no data, without an API call.
	commits, files, err := New().CompareCommits(context.Background(), "a", "b", "t", "", "v1")
	if err != nil || commits != nil || files != nil {
		t.Errorf("empty base = %v / %v / %v", commits, files, err)
	}
}

func TestGetTreeAndFileContent(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("# Hello"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/git/trees/"):
			if r.URL.Query().Get("recursive") != "1" {
				t.Error("tree must be recursive")
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"tree":[{"path":"README.md","type":"blob","size":7},{"path":"docs","type":"tree"}],"truncated":false}`))
		case strings.Contains(r.URL.Path, "/contents/"):
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"content":"` + encoded + `","encoding":"base64"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	tree, err := c.GetTree(context.Background(), "acme", "widget", "tok", "v0.2.0")
	if err != nil || len(tree) != 2 {
		t.Fatalf("tree = %+v err=%v", tree, err)
	}
	content, err := c.GetFileContent(context.Background(), "acme", "widget", "tok", "README.md", "v0.2.0")
	if err != nil {
		t.Fatalf("GetFileContent: %v", err)
	}
	if content != "# Hello" {
		t.Errorf("content = %q, want decoded base64", content)
	}
}

func TestGetFileContentNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := New(WithBaseURL(srv.URL)).GetFileContent(context.Background(), "a", "b", "t", "x.md", "v1")
	if apperror.CodeOf(err) != apperror.CodeNotFound {
		t.Errorf("code = %s, want not_found", apperror.CodeOf(err))
	}
}

func TestGetRepositoryDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"full_name":"acme/widget","description":"d","topics":["go","aws"],"visibility":"public","default_branch":"main","language":"Go","html_url":"https://x","license":{"spdx_id":"MIT","name":"MIT License"}}`))
	}))
	defer srv.Close()
	d, err := New(WithBaseURL(srv.URL)).GetRepositoryDetail(context.Background(), "acme", "widget", "tok")
	if err != nil {
		t.Fatalf("GetRepositoryDetail: %v", err)
	}
	if d.FullName != "acme/widget" || d.License != "MIT" || len(d.Topics) != 2 || d.Language != "Go" {
		t.Errorf("detail = %+v", d)
	}
}
