// Package publish contains adapters for delivering generated content to a
// destination. The MVP ships a placeholder LogPublisher; real destinations
// (a Git repository, object store, or CMS) are future work behind the same
// Publisher shape used by the pipeline.
package publish

import (
	"context"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// LogPublisher is a placeholder that logs what would be published instead of
// writing to a real destination. It never fails.
type LogPublisher struct {
	Logger *slog.Logger
}

// Publish logs each asset's kind and size (never the full content at info level).
func (p *LogPublisher) Publish(_ context.Context, repoFullName string, assets []generation.Content) error {
	if p.Logger == nil {
		return nil
	}
	for _, a := range assets {
		p.Logger.Info("would publish content (destination pending)",
			slog.String("repo", repoFullName),
			slog.String("kind", string(a.Kind)),
			slog.Int("markdown_bytes", len(a.Markdown)))
	}
	return nil
}
