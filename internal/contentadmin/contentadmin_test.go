package contentadmin

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	versions []types.ObjectVersion
	objects  []types.Object
	bodies   map[string]string // "key" or "key@version" -> content
	copies   []*s3.CopyObjectInput
	puts     []*s3.PutObjectInput
}

func (f *fakeS3) ListObjectVersions(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	return &s3.ListObjectVersionsOutput{Versions: f.versions}, nil
}
func (f *fakeS3) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{Contents: f.objects}, nil
}
func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	k := aws.ToString(in.Key)
	if v := aws.ToString(in.VersionId); v != "" {
		k += "@" + v
	}
	body, ok := f.bodies[k]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(body))}, nil
}
func (f *fakeS3) CopyObject(_ context.Context, in *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	f.copies = append(f.copies, in)
	return &s3.CopyObjectOutput{VersionId: aws.String("new-ver")}, nil
}
func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.puts = append(f.puts, in)
	return &s3.PutObjectOutput{}, nil
}

func admin(f *fakeS3) *Admin { return &Admin{API: f, Bucket: "bkt", Prefix: "gc"} }

func TestHistoryFiltersAndSorts(t *testing.T) {
	key := "gc/acme/widget/releases/v0.3.0/blog.md"
	t1 := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	f := &fakeS3{versions: []types.ObjectVersion{
		{Key: aws.String(key), VersionId: aws.String("old"), LastModified: aws.Time(t1), Size: aws.Int64(10)},
		{Key: aws.String(key), VersionId: aws.String("new"), LastModified: aws.Time(t2), Size: aws.Int64(20), IsLatest: aws.Bool(true)},
		{Key: aws.String("gc/acme/widget/releases/v0.3.0/blog.md.bak"), VersionId: aws.String("sibling"), LastModified: aws.Time(t2)},
	}}
	vs, err := admin(f).History(context.Background(), "acme/widget", "v0.3.0", "blog.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("versions = %d, want 2 (sibling excluded)", len(vs))
	}
	if vs[0].VersionID != "new" || !vs[0].IsLatest { // newest first
		t.Errorf("first = %+v, want newest 'new'", vs[0])
	}
}

func TestGetVersioned(t *testing.T) {
	key := "gc/acme/widget/releases/v0.3.0/blog.md"
	f := &fakeS3{bodies: map[string]string{
		key:          "current",
		key + "@old": "was here",
	}}
	cur, _ := admin(f).Get(context.Background(), "acme/widget", "v0.3.0", "blog.md", "")
	old, _ := admin(f).Get(context.Background(), "acme/widget", "v0.3.0", "blog.md", "old")
	if string(cur) != "current" || string(old) != "was here" {
		t.Errorf("cur=%q old=%q", cur, old)
	}
}

func TestCompareProducesDiff(t *testing.T) {
	key := "gc/acme/widget/releases/v0.3.0/blog.md"
	f := &fakeS3{bodies: map[string]string{
		key + "@a": "one\ntwo\nthree\n",
		key + "@b": "one\nTWO\nthree\n",
	}}
	diff, err := admin(f).Compare(context.Background(), "acme/widget", "v0.3.0", "blog.md", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "- two") || !strings.Contains(diff, "+ TWO") || !strings.Contains(diff, "  one") {
		t.Errorf("diff =\n%s", diff)
	}
}

func TestRollbackCopiesVersionOntoKey(t *testing.T) {
	f := &fakeS3{}
	newVer, err := admin(f).Rollback(context.Background(), "acme/widget", "v0.3.0", "blog.md", "target-ver")
	if err != nil {
		t.Fatal(err)
	}
	if newVer != "new-ver" {
		t.Errorf("new version = %q", newVer)
	}
	if len(f.copies) != 1 {
		t.Fatalf("copies = %d", len(f.copies))
	}
	src := aws.ToString(f.copies[0].CopySource)
	if !strings.Contains(src, "bkt/gc/acme/widget/releases/v0.3.0/blog.md") || !strings.Contains(src, "versionId=target-ver") {
		t.Errorf("copy source = %q", src)
	}
	if aws.ToString(f.copies[0].Key) != "gc/acme/widget/releases/v0.3.0/blog.md" {
		t.Errorf("dest key = %q", aws.ToString(f.copies[0].Key))
	}
}

func TestPromoteLatestCopiesAndWritesPointer(t *testing.T) {
	relPrefix := "gc/acme/widget/releases/v0.3.0/"
	f := &fakeS3{
		objects: []types.Object{
			{Key: aws.String(relPrefix + "blog.md")},
			{Key: aws.String(relPrefix + "architecture-diagram.svg")},
			{Key: aws.String(relPrefix + "metadata.json")},
			{Key: aws.String(relPrefix + "latest.json")}, // a stray pointer — must be skipped
		},
		bodies: map[string]string{
			"gc/acme/widget/releases/v0.3.0/metadata.json": `{"generationId":"gid-9"}`,
		},
	}
	n, err := admin(f).PromoteLatest(context.Background(), "acme/widget", "v0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 { // blog + svg + metadata copied; stray latest.json skipped
		t.Errorf("copied = %d, want 3", n)
	}
	// Every copy targets the latest/ prefix.
	for _, c := range f.copies {
		if !strings.Contains(aws.ToString(c.Key), "/latest/") {
			t.Errorf("copy dest not under latest/: %q", aws.ToString(c.Key))
		}
	}
	// A latest.json pointer was written, naming the release + generation.
	if len(f.puts) != 1 {
		t.Fatalf("puts = %d, want 1 (latest.json)", len(f.puts))
	}
	body, _ := io.ReadAll(f.puts[0].Body)
	if !strings.Contains(string(body), "v0.3.0") || !strings.Contains(string(body), "gid-9") {
		t.Errorf("latest.json = %s", body)
	}
}

func TestLineDiff(t *testing.T) {
	if d := LineDiff("same\ntext", "same\ntext"); d != "" {
		t.Errorf("identical should be empty, got %q", d)
	}
	d := LineDiff("a\nb\nc", "a\nx\nc")
	if d != "  a\n- b\n+ x\n  c\n" {
		t.Errorf("diff = %q", d)
	}
}
