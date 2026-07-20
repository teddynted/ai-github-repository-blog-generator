package platform

import (
	"context"
	"fmt"
)

// ContentTypeID identifies a pluggable content format. New formats are added as
// new ContentModule implementations — content types are plug-ins, not a switch.
type ContentTypeID string

const (
	ContentBlog          ContentTypeID = "technical-blog"
	ContentDocumentation ContentTypeID = "documentation"
	ContentWhitePaper    ContentTypeID = "white-paper"
	ContentBook          ContentTypeID = "book"
	ContentNewsletter    ContentTypeID = "newsletter"
	ContentEmail         ContentTypeID = "email-campaign"
	ContentPresentation  ContentTypeID = "presentation"
	ContentInfographic   ContentTypeID = "infographic"
	ContentPodcast       ContentTypeID = "podcast"
	ContentVideoCourse   ContentTypeID = "video-course"
	ContentWebinar       ContentTypeID = "webinar-script"
	ContentKnowledgeBase ContentTypeID = "knowledge-base"
	ContentProductDoc    ContentTypeID = "product-documentation"
	ContentReport        ContentTypeID = "technical-report"
)

// ContentRequest is a provider-neutral content-build request.
type ContentRequest struct {
	Type   ContentTypeID
	Title  string
	Topic  string
	Params map[string]string
}

// Artifact is a built content artifact.
type Artifact struct {
	Type   ContentTypeID
	Title  string
	Format string // markdown | html | pdf | ...
	Body   string
}

// ContentModule is a pluggable content-format generator. It embeds Provider so
// modules register like any plug-in (Kind = KindContent), keyed additionally by
// ContentType.
type ContentModule interface {
	Provider
	ContentType() ContentTypeID
	Build(ctx context.Context, req ContentRequest) (Artifact, error)
}

// simpleModule is a reference content module usable for any text format; it
// demonstrates the contract. Real modules add structure/prompts per type.
type simpleModule struct {
	id     string
	ctype  ContentTypeID
	format string
}

func (m *simpleModule) ID() string                    { return m.id }
func (m *simpleModule) Kind() Kind                    { return KindContent }
func (m *simpleModule) Capabilities() []Capability    { return []Capability{Capability(m.ctype)} }
func (m *simpleModule) Health(context.Context) Health { return OK() }
func (m *simpleModule) ContentType() ContentTypeID    { return m.ctype }

func (m *simpleModule) Build(_ context.Context, req ContentRequest) (Artifact, error) {
	if req.Title == "" {
		return Artifact{}, fmt.Errorf("%s: empty title", m.id)
	}
	body := fmt.Sprintf("# %s\n\n_%s_\n\nTopic: %s\n", req.Title, m.ctype, req.Topic)
	return Artifact{Type: m.ctype, Title: req.Title, Format: m.format, Body: body}, nil
}

// NewContentModule builds a reference content module for a type.
func NewContentModule(id string, ctype ContentTypeID) ContentModule {
	return &simpleModule{id: id, ctype: ctype, format: "markdown"}
}

// --- Podcast (interfaces + reference; architecture only) ---

// PodcastSpec describes an episode to assemble.
type PodcastSpec struct {
	Title      string
	Hosts      []string
	Segments   []string
	IntroMusic string
	OutroMusic string
	Metadata   map[string]string
}

// PodcastEpisode is an assembled episode plus its RSS item.
type PodcastEpisode struct {
	Title      string
	AudioURI   string
	DurationMs int
	RSSItem    string
}

// PodcastAssembler is the abstraction for podcast generation (AI hosts, multiple
// speakers, intro/outro music, episode metadata, RSS, publishing). The reference
// implementation is architecture-only, per the milestone.
type PodcastAssembler interface {
	Provider
	Assemble(ctx context.Context, spec PodcastSpec) (PodcastEpisode, error)
	RSS(channelTitle string, episodes []PodcastEpisode) string
}

// stubPodcast is the reference podcast assembler.
type stubPodcast struct{ id string }

func (p *stubPodcast) ID() string                    { return p.id }
func (p *stubPodcast) Kind() Kind                    { return KindContent }
func (p *stubPodcast) Capabilities() []Capability    { return []Capability{Capability(ContentPodcast)} }
func (p *stubPodcast) Health(context.Context) Health { return OK() }

func (p *stubPodcast) Assemble(_ context.Context, spec PodcastSpec) (PodcastEpisode, error) {
	if spec.Title == "" {
		return PodcastEpisode{}, fmt.Errorf("%s: empty episode title", p.id)
	}
	item := fmt.Sprintf("<item><title>%s</title><enclosure url=\"s3://podcasts/%s.mp3\"/></item>", spec.Title, spec.Title)
	return PodcastEpisode{Title: spec.Title, AudioURI: "s3://podcasts/" + spec.Title + ".mp3", DurationMs: 60000, RSSItem: item}, nil
}

func (p *stubPodcast) RSS(channelTitle string, episodes []PodcastEpisode) string {
	body := "<rss version=\"2.0\"><channel><title>" + channelTitle + "</title>"
	for _, e := range episodes {
		body += e.RSSItem
	}
	return body + "</channel></rss>"
}

// NewStubPodcast builds the reference podcast assembler.
func NewStubPodcast(id string) PodcastAssembler { return &stubPodcast{id: id} }

func registerContent(r *Registry) {
	// A couple of reference content modules demonstrate pluggable formats.
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "blog", Kind: KindContent, Name: "Blog module (reference)", Version: "1.0.0",
			Priority: 2, Description: "Reference technical-blog content module",
			Capabilities: []Capability{Capability(ContentBlog)},
		},
		Factory: func(ConfigSource) (Provider, error) { return NewContentModule("blog", ContentBlog), nil },
	})
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "newsletter", Kind: KindContent, Name: "Newsletter module (reference)", Version: "1.0.0",
			Priority: 1, Description: "Reference newsletter content module",
			Capabilities: []Capability{Capability(ContentNewsletter)},
		},
		Factory: func(ConfigSource) (Provider, error) { return NewContentModule("newsletter", ContentNewsletter), nil },
	})
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "podcast", Kind: KindContent, Name: "Podcast assembler (reference)", Version: "1.0.0",
			Priority: 1, Description: "Reference podcast assembler (architecture only)",
			Capabilities: []Capability{Capability(ContentPodcast)},
		},
		Factory: func(ConfigSource) (Provider, error) { return NewStubPodcast("podcast"), nil },
	})
}
