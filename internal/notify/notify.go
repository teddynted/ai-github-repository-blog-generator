// Package notify delivers run notifications (published, held, failed). The MVP
// ships a LogNotifier; email / Slack / webhook channels can follow behind the
// same Notifier port without changing callers.
package notify

import (
	"context"
	"log/slog"
)

// Status is the outcome being notified.
type Status string

const (
	StatusPublished Status = "published"
	StatusHeld      Status = "held"
	StatusFailed    Status = "failed"
)

// Event is a single notification.
type Event struct {
	Repo   string
	Status Status
	Assets int
	Err    string
}

// Notifier delivers notifications.
type Notifier interface {
	Notify(ctx context.Context, e Event) error
}

// LogNotifier writes notifications to the structured logger. It is the MVP
// channel and never fails.
type LogNotifier struct {
	Logger *slog.Logger
}

// Notify logs the event.
func (n *LogNotifier) Notify(_ context.Context, e Event) error {
	if n.Logger == nil {
		return nil
	}
	attrs := []any{
		slog.String("repo", e.Repo),
		slog.String("status", string(e.Status)),
		slog.Int("assets", e.Assets),
	}
	if e.Err != "" {
		attrs = append(attrs, slog.String("error", e.Err))
	}
	n.Logger.Info("notification", attrs...)
	return nil
}
