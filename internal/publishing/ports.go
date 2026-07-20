package publishing

import (
	"context"
	"net/http"
	"time"
)

// Publisher is the common interface every platform adapter implements. The
// application layer depends only on this interface, so new platforms are added
// without modifying existing business logic.
type Publisher interface {
	// Name is the platform this publisher targets.
	Name() Platform
	// Supports reports whether the publisher can handle a content type.
	Supports(t ContentType) bool
	// Validate checks the content + metadata are publishable to this platform
	// (without calling the API).
	Validate(c Content, m PlatformMetadata) error
	// Publish distributes the content and returns the platform id + URL.
	Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error)
	// Update edits an already-published item.
	Update(ctx context.Context, platformID string, c Content, m PlatformMetadata) (PublicationResult, error)
	// Delete removes a published item.
	Delete(ctx context.Context, platformID string) error
	// GetStatus returns the platform-side status of a published item.
	GetStatus(ctx context.Context, platformID string) (string, error)
}

// Repository persists publications. In-memory today; SQLite/PostgreSQL adapters
// implement the same interface.
type Repository interface {
	Save(p *Publication) error
	Get(id string) (*Publication, error)
	List() ([]*Publication, error)
	ByStatus(s PublicationStatus) ([]*Publication, error)
}

// Notifier emits publication events (success/failure/scheduled/retry/…). A log
// adapter ships by default; Slack/Discord/Email/Teams adapters implement the
// same interface.
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
}

// Event is a notification payload.
type Event struct {
	Kind     string // published | failed | scheduled | retry | approval-missing | quota-exceeded
	Platform Platform
	Content  string // content ID
	URL      string
	Message  string
}

// Clock is the time port (deterministic in tests).
type Clock func() time.Time

// Sleeper is the delay port, so retry/backoff is instant in tests.
type Sleeper func(ctx context.Context, d time.Duration)

// HTTPDoer is the HTTP port publisher adapters use, so they are testable with a
// mock and never require real network/credentials in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// realSleeper waits, respecting context cancellation.
func realSleeper(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
