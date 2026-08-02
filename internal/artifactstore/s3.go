// Package artifactstore provides an S3-backed contentsuite.ArtifactStore so the
// cloud AI job is idempotent: before generating any artifact the orchestrator
// checks S3, and reuses an artifact that already exists instead of regenerating
// it. This avoids re-spending Anthropic/Bedrock tokens on artifacts a previous
// run already produced — existing generated artifacts are never regenerated.
package artifactstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3API is the subset of the S3 client this store uses.
type S3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Store persists each stage's structured output as a JSON sidecar next to the
// published Markdown, under
// <prefix>/<owner>/<name>/releases/<tag>/.artifacts/<stage>.json.
// It is built per release (it carries the release's key prefix and the run
// context), which is safe because the worker processes releases serially.
type S3Store struct {
	ctx    context.Context
	api    S3API
	bucket string
	base   string // the release's .artifacts key prefix
	logger *slog.Logger
}

// NewS3 builds an S3Store for one release. prefix defaults to "generated-content"
// to match the publisher's layout.
func NewS3(ctx context.Context, api S3API, bucket, prefix, owner, name, tag string, logger *slog.Logger) *S3Store {
	if prefix == "" {
		prefix = "generated-content"
	}
	base := path.Join(strings.Trim(prefix, "/"), safe(owner), safe(name), "releases", safe(tag), ".artifacts")
	return &S3Store{ctx: ctx, api: api, bucket: bucket, base: base, logger: logger}
}

func (s *S3Store) key(stage string) string { return path.Join(s.base, stage+".json") }

// Load reports whether the stage's artifact already exists in S3 and, if so,
// decodes its sidecar into v (so the stage is reused, not regenerated). A missing
// object returns found=false.
func (s *S3Store) Load(stage string, v any) (bool, error) {
	out, err := s.api.GetObject(s.ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(stage))})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	defer out.Body.Close()
	b, err := io.ReadAll(out.Body)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return false, err
	}
	if s.logger != nil {
		s.logger.Info("artifact reused from s3 (no model call)", "stage", stage, "key", s.key(stage))
	}
	return true, nil
}

// Save writes the stage's struct as a JSON sidecar so a later run reuses it.
func (s *S3Store) Save(stage string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = s.api.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.key(stage)),
		Body:        bytes.NewReader(b),
		ContentType: aws.String("application/json"),
	})
	return err
}

// isNotFound reports whether an S3 GetObject error means the object is absent.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "NoSuchKey") || strings.Contains(s, "status code: 404") ||
		strings.Contains(strings.ToLower(s), "not found")
}

// safe sanitizes a key segment (mirrors the publisher's safeSegment).
func safe(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
