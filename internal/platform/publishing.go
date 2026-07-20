package platform

import (
	"context"
	"fmt"
)

// Publishing capabilities.
const (
	CapPublishArticle Capability = "publish-article"
	CapPublishVideo   Capability = "publish-video"
	CapPublishSocial  Capability = "publish-social"
	CapSchedule       Capability = "schedule"
)

// Article is a provider-neutral publishable item. It is intentionally minimal;
// the platform's richer publishing package (internal/publishing) handles the full
// production flow — this is the extension seam future platforms plug into.
type Article struct {
	Title        string
	Body         string
	Tags         []string
	CanonicalURL string
	CoverImage   string
	Draft        bool
}

// PublishResult is the outcome of a publish.
type PublishResult struct {
	Provider string
	ID       string
	URL      string
	Status   string // published | scheduled | draft
}

// PublishingProvider is the common interface every publishing destination
// implements (current: YouTube, Medium, Dev.to, Hashnode; future: LinkedIn, X,
// TikTok, Instagram, Facebook, Reddit, Discord, Slack, Ghost, WordPress,
// Substack, Beehiiv, …). A new platform is a new implementation only.
type PublishingProvider interface {
	Provider
	Publish(ctx context.Context, a Article) (PublishResult, error)
}

// dryRunPublisher is the reference publisher: it validates and echoes without
// making network calls, so publishing flows are testable offline.
type dryRunPublisher struct{ id string }

func (d *dryRunPublisher) ID() string { return d.id }
func (d *dryRunPublisher) Kind() Kind { return KindPublishing }
func (d *dryRunPublisher) Capabilities() []Capability {
	return []Capability{CapPublishArticle, CapSchedule}
}
func (d *dryRunPublisher) Health(context.Context) Health { return OK() }

func (d *dryRunPublisher) Publish(_ context.Context, a Article) (PublishResult, error) {
	if a.Title == "" {
		return PublishResult{}, fmt.Errorf("%s: empty title", d.id)
	}
	status := "published"
	if a.Draft {
		status = "draft"
	}
	return PublishResult{Provider: d.id, ID: "dryrun-1", URL: "https://example.test/" + a.Title, Status: status}, nil
}

// NewDryRunPublisher builds the reference publisher.
func NewDryRunPublisher(id string) PublishingProvider { return &dryRunPublisher{id: id} }

func registerPublishing(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "dryrun", Kind: KindPublishing, Name: "Dry-run (reference)", Version: "1.0.0",
			Priority: 1, Description: "Reference publisher (no network)",
			Capabilities: []Capability{CapPublishArticle, CapSchedule},
		},
		Factory: func(ConfigSource) (Provider, error) { return &dryRunPublisher{id: "dryrun"}, nil },
	})
}
