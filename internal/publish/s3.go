package publish

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// S3API is the subset of the S3 client this adapter uses.
type S3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Publisher writes generated Markdown to S3 under
// <prefix>/<owner>/<name>/releases/<tag>/<kind>.md for a release (first-class,
// tag-addressable) or <prefix>/<owner>/<name>/<YYYY-MM-DD>/<kind>.md for a
// snapshot — the same layout as FilePublisher, but remote.
type S3Publisher struct {
	api    S3API
	bucket string
	prefix string
	Now    func() time.Time
	Logger *slog.Logger
}

// NewS3 builds an S3Publisher. A blank prefix defaults to "generated-content".
func NewS3(api S3API, bucket, prefix string, logger *slog.Logger) *S3Publisher {
	if prefix == "" {
		prefix = "generated-content"
	}
	return &S3Publisher{api: api, bucket: bucket, prefix: strings.Trim(prefix, "/"), Logger: logger}
}

// Publish uploads each asset as an object.
func (p *S3Publisher) Publish(ctx context.Context, repoFullName string, assets []generation.Content) error {
	_, err := p.PublishWithResults(ctx, repoFullName, assets)
	return err
}

// PublishWithResults publishes the assets and returns where each landed,
// including the S3 object VersionID (the content bucket is versioned). The
// release pipeline uses these to pin each artifact to an immutable version in
// the release metadata.json.
func (p *S3Publisher) PublishWithResults(ctx context.Context, repoFullName string, assets []generation.Content) ([]generation.PutResult, error) {
	owner, name, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || name == "" {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid repository name")
	}
	date := p.now().UTC().Format("2006-01-02")
	release := batchRelease(assets)
	segs := runSegments(release, date)
	results := make([]generation.PutResult, 0, len(assets))
	for _, a := range assets {
		parts := append([]string{p.prefix, owner, name}, segs...)
		key := path.Join(append(parts, a.Filename())...)
		out, err := p.api.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(p.bucket),
			Key:         aws.String(key),
			Body:        strings.NewReader(a.Markdown),
			ContentType: aws.String(a.ContentType()),
		})
		if err != nil {
			return results, fmt.Errorf("put %s: %w", key, err)
		}
		results = append(results, generation.PutResult{
			Kind: string(a.Kind), Ext: a.Ext, Key: key, VersionID: aws.ToString(out.VersionId),
		})
	}
	if p.Logger != nil {
		p.Logger.Info("published content to s3",
			slog.String("repo", repoFullName),
			slog.String("release", release),
			slog.String("bucket", p.bucket),
			slog.Int("assets", len(assets)))
	}
	return results, nil
}

func (p *S3Publisher) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
