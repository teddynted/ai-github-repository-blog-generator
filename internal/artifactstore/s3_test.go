package artifactstore

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	objs map[string][]byte
	puts int
}

func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	b, ok := f.objs[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	b, _ := io.ReadAll(in.Body)
	if f.objs == nil {
		f.objs = map[string][]byte{}
	}
	f.objs[aws.ToString(in.Key)] = b
	f.puts++
	return &s3.PutObjectOutput{}, nil
}

type doc struct{ N int }

func TestS3StoreRoundtripAndMiss(t *testing.T) {
	f := &fakeS3{objs: map[string][]byte{}}
	s := NewS3(context.Background(), f, "bucket", "gc", "acme", "widget", "v0.6.0", nil)

	// Missing object → not found, no error, no generation skipped.
	var d doc
	if ok, err := s.Load("blog", &d); ok || err != nil {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}
	// Save then hit → reused.
	if err := s.Save("blog", doc{N: 7}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if ok, err := s.Load("blog", &d); !ok || err != nil || d.N != 7 {
		t.Fatalf("hit: ok=%v err=%v d=%+v", ok, err, d)
	}
	// Key layout mirrors the publisher's release layout.
	want := "gc/acme/widget/releases/v0.6.0/.artifacts/blog.json"
	if _, ok := f.objs[want]; !ok {
		t.Errorf("expected key %q; have %v", want, keys(f.objs))
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
