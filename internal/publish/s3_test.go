package publish

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

type fakeS3 struct {
	puts []*s3.PutObjectInput
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.puts = append(f.puts, in)
	return &s3.PutObjectOutput{}, nil
}

func TestS3PublisherWritesDatedKeys(t *testing.T) {
	f := &fakeS3{}
	p := NewS3(f, "my-bucket", "generated-content", nil)
	p.Now = func() time.Time { return time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC) }

	err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "# post"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(f.puts) != 1 {
		t.Fatalf("puts = %d", len(f.puts))
	}
	in := f.puts[0]
	if aws.ToString(in.Bucket) != "my-bucket" {
		t.Errorf("bucket = %q", aws.ToString(in.Bucket))
	}
	if got := aws.ToString(in.Key); got != "generated-content/acme/widget/2026-07-09/blog.md" {
		t.Errorf("key = %q", got)
	}
	if aws.ToString(in.ContentType) != "text/markdown" {
		t.Errorf("content type = %q", aws.ToString(in.ContentType))
	}
	body, _ := io.ReadAll(in.Body)
	if string(body) != "# post" {
		t.Errorf("body = %q", body)
	}
}

func TestS3PublisherRejectsBadRepo(t *testing.T) {
	p := NewS3(&fakeS3{}, "b", "", nil)
	if err := p.Publish(context.Background(), "noslash", nil); apperror.CodeOf(err) != apperror.CodeInvalidInput {
		t.Errorf("code = %s, want invalid_input", apperror.CodeOf(err))
	}
}

func TestS3PublisherReleaseFirstClassKeys(t *testing.T) {
	f := &fakeS3{}
	p := NewS3(f, "my-bucket", "generated-content", nil)
	p.Now = func() time.Time { return time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC) }

	err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "# post", Release: "v0.3.0"},
		{Kind: "storyboard", Markdown: "sb", Release: "v0.3.0"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// A release run is addressable by tag, with no date segment.
	want := map[string]bool{
		"generated-content/acme/widget/releases/v0.3.0/blog.md":       true,
		"generated-content/acme/widget/releases/v0.3.0/storyboard.md": true,
	}
	for _, in := range f.puts {
		k := aws.ToString(in.Key)
		if !want[k] {
			t.Errorf("unexpected key %q", k)
		}
		delete(want, k)
	}
	if len(want) != 0 {
		t.Errorf("missing keys: %v", want)
	}
}

func TestS3PublisherSanitisesReleaseSegment(t *testing.T) {
	f := &fakeS3{}
	p := NewS3(f, "b", "gc", nil)
	// A tag that tries to traverse must be neutralised.
	if err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "x", Release: "../../etc"},
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	k := aws.ToString(f.puts[0].Key)
	if got := k; got != "gc/acme/widget/releases/etc/blog.md" {
		t.Errorf("unsanitised key: %q", got)
	}
}
