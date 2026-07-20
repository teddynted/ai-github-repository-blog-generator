package publishing

import (
	"context"
	"log/slog"
)

// Notification event kinds.
const (
	EventPublished       = "published"
	EventFailed          = "failed"
	EventScheduled       = "scheduled"
	EventRetry           = "retry"
	EventApprovalMissing = "approval-missing"
	EventQuotaExceeded   = "quota-exceeded"
)

// LogNotifier logs events via slog. Slack/Discord/Email/Teams notifiers
// implement the same Notifier interface and are swapped in via config.
type LogNotifier struct {
	Logger *slog.Logger
}

// Notify logs the event. It never logs secrets — Event carries no credentials.
func (n LogNotifier) Notify(_ context.Context, ev Event) error {
	l := n.Logger
	if l == nil {
		l = slog.Default()
	}
	l.Info("publishing event",
		slog.String("kind", ev.Kind),
		slog.String("platform", string(ev.Platform)),
		slog.String("content", ev.Content),
		slog.String("url", ev.URL),
		slog.String("message", ev.Message),
	)
	return nil
}

// nopNotifier discards events (default when none is configured).
type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, Event) error { return nil }
