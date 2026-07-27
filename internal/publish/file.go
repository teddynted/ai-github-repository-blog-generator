package publish

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// FilePublisher writes generated Markdown to the local filesystem under
// <dir>/<owner>/<name>/releases/<tag>/<kind>.md for a release, or
// <dir>/<owner>/<name>/<YYYY-MM-DD>/<kind>.md for a snapshot. On the instance <dir> is on the
// persistent EBS volume. It is the first real publish destination; remote
// destinations (Git / object store / CMS) can follow behind the same Publisher.
type FilePublisher struct {
	Dir    string
	Now    func() time.Time
	Logger *slog.Logger
}

// Publish writes each asset as a Markdown file and returns the first error.
func (p *FilePublisher) Publish(_ context.Context, repoFullName string, assets []generation.Content) error {
	dir, err := p.targetDir(repoFullName, batchRelease(assets), batchExperiment(assets))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	for _, a := range assets {
		path := filepath.Join(dir, a.Filename())
		if err := os.WriteFile(path, []byte(a.Markdown), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if p.Logger != nil {
		p.Logger.Info("published content",
			slog.String("repo", repoFullName),
			slog.String("release", batchRelease(assets)),
			slog.String("dir", dir),
			slog.Int("assets", len(assets)))
	}
	return nil
}

// PublishLatest writes the assets to the repository's stable "latest" area
// (<dir>/<owner>/<name>/latest/<kind>.<ext>), overwriting the previous latest.
// It is how the newest approved release is exposed without a tag.
func (p *FilePublisher) PublishLatest(_ context.Context, repoFullName string, assets []generation.Content) error {
	owner, name, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(repoFullName, "..") {
		return apperror.New(apperror.CodeInvalidInput, "invalid repository name")
	}
	dir := filepath.Join(p.Dir, owner, name, "latest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create latest dir: %w", err)
	}
	for _, a := range assets {
		path := filepath.Join(dir, a.Filename())
		if err := os.WriteFile(path, []byte(a.Markdown), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// targetDir builds the output directory for a run, guarding against path
// traversal from an unexpected repository name. A release run is first-class
// (<dir>/<owner>/<name>/releases/<tag>); a snapshot keeps the dated layout
// (<dir>/<owner>/<name>/<date>).
func (p *FilePublisher) targetDir(repoFullName, release, experiment string) (string, error) {
	owner, name, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(repoFullName, "..") {
		return "", apperror.New(apperror.CodeInvalidInput, "invalid repository name")
	}
	date := p.now().UTC().Format("2006-01-02")
	parts := append([]string{p.Dir, owner, name}, runSegments(release, experiment, date)...)
	return filepath.Join(parts...), nil
}

func (p *FilePublisher) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
