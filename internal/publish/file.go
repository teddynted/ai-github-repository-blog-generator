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
// <dir>/<owner>/<name>/<YYYY-MM-DD>/<kind>.md. On the instance <dir> is on the
// persistent EBS volume. It is the first real publish destination; remote
// destinations (Git / object store / CMS) can follow behind the same Publisher.
type FilePublisher struct {
	Dir    string
	Now    func() time.Time
	Logger *slog.Logger
}

// Publish writes each asset as a Markdown file and returns the first error.
func (p *FilePublisher) Publish(_ context.Context, repoFullName string, assets []generation.Content) error {
	dir, err := p.targetDir(repoFullName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	for _, a := range assets {
		path := filepath.Join(dir, string(a.Kind)+".md")
		if err := os.WriteFile(path, []byte(a.Markdown), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if p.Logger != nil {
		p.Logger.Info("published content",
			slog.String("repo", repoFullName),
			slog.String("dir", dir),
			slog.Int("assets", len(assets)))
	}
	return nil
}

// targetDir builds the dated output directory, guarding against path traversal
// from an unexpected repository name.
func (p *FilePublisher) targetDir(repoFullName string) (string, error) {
	owner, name, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(repoFullName, "..") {
		return "", apperror.New(apperror.CodeInvalidInput, "invalid repository name")
	}
	date := p.now().UTC().Format("2006-01-02")
	return filepath.Join(p.Dir, owner, name, date), nil
}

func (p *FilePublisher) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
