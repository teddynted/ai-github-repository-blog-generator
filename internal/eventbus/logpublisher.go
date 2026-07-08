// Package eventbus contains adapters for the webhook Publisher port. The
// production EventBridge adapter is added in Milestone 5; for now LogPublisher
// records matched events so the vertical slice is observable end to end without
// yet starting compute.
package eventbus

import (
	"context"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/webhook"
)

// LogPublisher is a placeholder Publisher that logs matched events instead of
// publishing them to EventBridge. It lets Milestone 4 run without the
// downstream event plumbing.
type LogPublisher struct {
	Logger *slog.Logger
}

// Publish logs the matched event. It never fails.
func (p *LogPublisher) Publish(_ context.Context, ev webhook.Event) error {
	if p.Logger != nil {
		p.Logger.Info("matched event (EventBridge publish pending Milestone 5)",
			slog.String("repo", ev.RepoFullName),
			slog.String("ref", ev.Ref),
			slog.String("commit_sha", ev.CommitSHA))
	}
	return nil
}
