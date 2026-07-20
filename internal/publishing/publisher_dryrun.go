package publishing

import "context"

// DryRunPublisher is a no-network publisher for local runs and tests. It records
// what it would publish and returns a synthetic result. FailTimes lets tests
// exercise the retry engine: the first N Publish calls return FailWith.
type DryRunPublisher struct {
	Platform   Platform
	SupportsFn func(ContentType) bool
	FailTimes  int
	FailWith   error
	Published  []Content
	calls      int
}

// NewDryRunPublisher builds a dry-run publisher for a platform.
func NewDryRunPublisher(p Platform) *DryRunPublisher {
	return &DryRunPublisher{Platform: p}
}

func (d *DryRunPublisher) Name() Platform { return d.Platform }

func (d *DryRunPublisher) Supports(t ContentType) bool {
	if d.SupportsFn != nil {
		return d.SupportsFn(t)
	}
	return true
}

func (d *DryRunPublisher) Validate(Content, PlatformMetadata) error { return nil }

func (d *DryRunPublisher) Publish(_ context.Context, c Content, _ PlatformMetadata) (PublicationResult, error) {
	d.calls++
	if d.calls <= d.FailTimes {
		if d.FailWith != nil {
			return PublicationResult{}, d.FailWith
		}
		return PublicationResult{}, errNetwork(nil)
	}
	d.Published = append(d.Published, c)
	id := slugify(c.ID)
	return PublicationResult{
		Platform:   d.Platform,
		PlatformID: id,
		URL:        "https://example.test/" + string(d.Platform) + "/" + id,
	}, nil
}

func (d *DryRunPublisher) Update(_ context.Context, id string, c Content, _ PlatformMetadata) (PublicationResult, error) {
	return PublicationResult{Platform: d.Platform, PlatformID: id, URL: "https://example.test/" + string(d.Platform) + "/" + id}, nil
}

func (d *DryRunPublisher) Delete(context.Context, string) error { return nil }

func (d *DryRunPublisher) GetStatus(context.Context, string) (string, error) { return "published", nil }
