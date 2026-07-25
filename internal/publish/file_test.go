package publish

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

func TestFilePublisherWritesDatedAssets(t *testing.T) {
	dir := t.TempDir()
	p := &FilePublisher{Dir: dir, Now: func() time.Time { return time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC) }}

	err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "# post"},
		{Kind: generation.KindReadme, Markdown: "readme improvements"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	base := filepath.Join(dir, "acme", "widget", "2026-07-08")
	blog, err := os.ReadFile(filepath.Join(base, "blog.md"))
	if err != nil || string(blog) != "# post" {
		t.Errorf("blog.md = %q err=%v", blog, err)
	}
	readme, err := os.ReadFile(filepath.Join(base, "readme-improvements.md"))
	if err != nil || string(readme) != "readme improvements" {
		t.Errorf("readme-improvements.md = %q err=%v", readme, err)
	}
}

func TestFilePublisherRejectsBadRepoName(t *testing.T) {
	p := &FilePublisher{Dir: t.TempDir()}
	for _, bad := range []string{"", "noslash", "../etc/passwd", "owner/"} {
		if err := p.Publish(context.Background(), bad, nil); apperror.CodeOf(err) != apperror.CodeInvalidInput {
			t.Errorf("repo %q: code = %s, want invalid_input", bad, apperror.CodeOf(err))
		}
	}
}

func TestFilePublisherReleaseFirstClass(t *testing.T) {
	dir := t.TempDir()
	p := &FilePublisher{Dir: dir, Now: func() time.Time { return time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC) }}
	if err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "# post", Release: "v0.3.0"},
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// Release run → releases/<tag>, no date segment.
	blog := filepath.Join(dir, "acme", "widget", "releases", "v0.3.0", "blog.md")
	if _, err := os.ReadFile(blog); err != nil {
		t.Errorf("release blog not at %s: %v", blog, err)
	}
}
