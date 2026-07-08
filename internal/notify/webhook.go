package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/retry"
)

// WebhookNotifier POSTs a JSON notification to a URL. The payload includes a
// Slack-compatible "text" field plus the structured event fields, so it works
// with Slack incoming webhooks and generic webhook receivers alike.
type WebhookNotifier struct {
	url    string
	http   *http.Client
	retry  retry.Config
	logger *slog.Logger
}

// WebhookOption configures a WebhookNotifier.
type WebhookOption func(*WebhookNotifier)

// WithWebhookRetry overrides the retry policy (e.g. faster in tests).
func WithWebhookRetry(cfg retry.Config) WebhookOption {
	return func(n *WebhookNotifier) { n.retry = cfg }
}

// NewWebhook builds a WebhookNotifier for the given URL.
func NewWebhook(url string, logger *slog.Logger, opts ...WebhookOption) *WebhookNotifier {
	n := &WebhookNotifier{
		url:    url,
		http:   &http.Client{Timeout: 10 * time.Second},
		retry:  retry.Default,
		logger: logger,
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

// Notify posts the event, retrying transient failures.
func (n *WebhookNotifier) Notify(ctx context.Context, e Event) error {
	payload := map[string]any{
		"text":   summarize(e),
		"repo":   e.Repo,
		"status": string(e.Status),
		"assets": e.Assets,
	}
	if e.Err != "" {
		payload["error"] = e.Err
	}
	raw, _ := json.Marshal(payload)

	return retry.Do(ctx, n.retry, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(raw))
		if err != nil {
			return apperror.Wrap(err, apperror.CodeInternal, "build notify request")
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := n.http.Do(req)
		if err != nil {
			return apperror.Wrap(err, apperror.CodeUpstream, "notify webhook failed")
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 300 {
			return apperror.New(apperror.CodeUpstream, fmt.Sprintf("notify webhook returned status %d", resp.StatusCode))
		}
		return nil
	})
}

func summarize(e Event) string {
	switch e.Status {
	case StatusPublished:
		return fmt.Sprintf("✅ %s: published %d asset(s)", e.Repo, e.Assets)
	case StatusHeld:
		return fmt.Sprintf("⏸ %s: content held for approval", e.Repo)
	case StatusFailed:
		return fmt.Sprintf("❌ %s: run failed (%s)", e.Repo, e.Err)
	default:
		return fmt.Sprintf("%s: %s", e.Repo, e.Status)
	}
}

// Multi fans a notification out to several notifiers (best-effort: it attempts
// all and returns the first error).
type Multi []Notifier

// Notify delivers to every notifier.
func (m Multi) Notify(ctx context.Context, e Event) error {
	var firstErr error
	for _, n := range m {
		if err := n.Notify(ctx, e); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
