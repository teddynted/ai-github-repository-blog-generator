// Package contentadmin performs operator actions against the versioned
// generated-content S3 bucket: list an artifact's version history, compare two
// versions, roll a single artifact back to a prior version, and (re-)promote a
// release into latest/. It is the read/rollback counterpart to the write-only
// publisher — kept in its own package so it is testable with a fake S3 and so
// the worker never links these read/copy capabilities.
package contentadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentmeta"
)

// S3API is the subset of the S3 client the admin needs (satisfied by *s3.Client).
type S3API interface {
	ListObjectVersions(ctx context.Context, in *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	CopyObject(ctx context.Context, in *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Admin operates on one content bucket + prefix.
type Admin struct {
	API    S3API
	Bucket string
	Prefix string // e.g. "generated-content"
}

// Version is one stored version of an artifact.
type Version struct {
	VersionID    string
	LastModified time.Time
	Size         int64
	IsLatest     bool
}

// History returns the version history of a single release artifact (newest
// first), e.g. artifact "blog.md" of release "v0.3.0".
func (a *Admin) History(ctx context.Context, repo, release, artifact string) ([]Version, error) {
	key, err := a.releaseKey(repo, release, artifact)
	if err != nil {
		return nil, err
	}
	out, err := a.API.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(a.Bucket), Prefix: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	var vs []Version
	for _, v := range out.Versions {
		if aws.ToString(v.Key) != key { // Prefix can match siblings; keep exact
			continue
		}
		vs = append(vs, Version{
			VersionID:    aws.ToString(v.VersionId),
			LastModified: aws.ToTime(v.LastModified),
			Size:         aws.ToInt64(v.Size),
			IsLatest:     aws.ToBool(v.IsLatest),
		})
	}
	sort.SliceStable(vs, func(i, j int) bool { return vs[i].LastModified.After(vs[j].LastModified) })
	return vs, nil
}

// Get fetches an artifact's bytes; versionID "" reads the current version.
func (a *Admin) Get(ctx context.Context, repo, release, artifact, versionID string) ([]byte, error) {
	key, err := a.releaseKey(repo, release, artifact)
	if err != nil {
		return nil, err
	}
	in := &s3.GetObjectInput{Bucket: aws.String(a.Bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	out, err := a.API.GetObject(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// Compare returns a unified-style line diff between two versions of an artifact.
func (a *Admin) Compare(ctx context.Context, repo, release, artifact, versionA, versionB string) (string, error) {
	ba, err := a.Get(ctx, repo, release, artifact, versionA)
	if err != nil {
		return "", err
	}
	bb, err := a.Get(ctx, repo, release, artifact, versionB)
	if err != nil {
		return "", err
	}
	return LineDiff(string(ba), string(bb)), nil
}

// Rollback makes toVersion the current version of an artifact by server-side
// copying it back onto the live key (creating a new current version equal to
// the old one — nothing is destroyed). Returns the new version id.
func (a *Admin) Rollback(ctx context.Context, repo, release, artifact, toVersion string) (string, error) {
	if toVersion == "" {
		return "", fmt.Errorf("rollback: a target version id is required")
	}
	key, err := a.releaseKey(repo, release, artifact)
	if err != nil {
		return "", err
	}
	out, err := a.API.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(a.Bucket),
		Key:        aws.String(key),
		CopySource: aws.String(a.copySource(key, toVersion)),
	})
	if err != nil {
		return "", fmt.Errorf("rollback copy: %w", err)
	}
	return aws.ToString(out.VersionId), nil
}

// PromoteLatest copies a release's current artifacts into latest/ and writes a
// latest.json pointer, making that release the one latest/ serves. It reads the
// release's generation id from its metadata.json when present.
func (a *Admin) PromoteLatest(ctx context.Context, repo, release string) (int, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return 0, err
	}
	prefix := path.Join(a.Prefix, owner, name, "releases", safeSeg(release)) + "/"
	out, err := a.API.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.Bucket), Prefix: aws.String(prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("list release: %w", err)
	}

	var filenames []string
	copied := 0
	for _, o := range out.Contents {
		srcKey := aws.ToString(o.Key)
		filename := path.Base(srcKey)
		if filename == contentmeta.LatestKind+"."+contentmeta.LatestExt {
			continue // never copy a stale pointer
		}
		dstKey := path.Join(a.Prefix, owner, name, "latest", filename)
		if _, err := a.API.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(a.Bucket),
			Key:        aws.String(dstKey),
			CopySource: aws.String(a.copySource(srcKey, "")),
		}); err != nil {
			return copied, fmt.Errorf("copy %s: %w", filename, err)
		}
		copied++
		if filename != contentmeta.Kind+"."+contentmeta.Ext {
			filenames = append(filenames, filename)
		}
	}

	pointer, perr := contentmeta.BuildLatest(release, a.generationID(ctx, repo, release), filenames, nil).Content()
	if perr == nil {
		_, _ = a.API.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(a.Bucket),
			Key:         aws.String(path.Join(a.Prefix, owner, name, "latest", pointer.Filename())),
			Body:        strings.NewReader(pointer.Markdown),
			ContentType: aws.String(pointer.ContentType()),
		})
	}
	return copied, nil
}

// generationID reads a release's metadata.json generationId, best-effort.
func (a *Admin) generationID(ctx context.Context, repo, release string) string {
	b, err := a.Get(ctx, repo, release, contentmeta.Kind+"."+contentmeta.Ext, "")
	if err != nil {
		return ""
	}
	var m struct {
		GenerationID string `json:"generationId"`
	}
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return m.GenerationID
}

func (a *Admin) releaseKey(repo, release, artifact string) (string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(artifact, "/\\") || strings.Contains(artifact, "..") {
		return "", fmt.Errorf("invalid artifact name %q", artifact)
	}
	return path.Join(a.Prefix, owner, name, "releases", safeSeg(release), artifact), nil
}

// copySource builds a CopyObject source ("bucket/key[?versionId=...]") with the
// key path URL-escaped, as the S3 API requires.
func (a *Admin) copySource(key, versionID string) string {
	src := a.Bucket + "/" + escapeKey(key)
	if versionID != "" {
		src += "?versionId=" + url.QueryEscape(versionID)
	}
	return src
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func splitRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(repo, "..") {
		return "", "", fmt.Errorf("invalid repository %q (want owner/name)", repo)
	}
	return owner, name, nil
}

func safeSeg(s string) string {
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "..", "-")
	return strings.Trim(s, "-")
}
