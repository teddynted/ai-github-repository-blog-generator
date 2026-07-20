package publishing

import "context"

// DevToPublisher publishes articles to Dev.to (Forem) via its REST API.
// https://developers.forem.com/api — POST /api/articles with the "api-key" header.
type DevToPublisher struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://dev.to/api
}

// NewDevToPublisher builds a Dev.to publisher.
func NewDevToPublisher(http HTTPDoer, creds Credentials) *DevToPublisher {
	return &DevToPublisher{HTTP: http, Creds: creds, BaseURL: "https://dev.to/api"}
}

func (p *DevToPublisher) Name() Platform { return PlatformDevTo }

func (p *DevToPublisher) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeDevToArticle
}

func (p *DevToPublisher) Validate(c Content, m PlatformMetadata) error {
	if !p.Creds.has(envDevToKey) {
		return errAuth("DEVTO_API_KEY is not set")
	}
	if m.Title == "" || m.Body == "" {
		return errInvalid("Dev.to article needs a title and body")
	}
	return nil
}

type devtoArticle struct {
	Title        string   `json:"title"`
	BodyMarkdown string   `json:"body_markdown"`
	Published    bool     `json:"published"`
	Tags         []string `json:"tags,omitempty"`
	CanonicalURL string   `json:"canonical_url,omitempty"`
	MainImage    string   `json:"main_image,omitempty"`
	Description  string   `json:"description,omitempty"`
}

type devtoResp struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

func (p *DevToPublisher) Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error) {
	body := map[string]devtoArticle{"article": {
		Title:        m.Title,
		BodyMarkdown: m.Body,
		Published:    m.Visibility != "draft",
		Tags:         topStrings(m.Tags, devtoTagMax),
		CanonicalURL: m.CanonicalURL,
		MainImage:    m.CoverImage,
		Description:  truncateChars(m.Description, 140),
	}}
	var out devtoResp
	_, err := doJSON(ctx, p.HTTP, "POST", p.BaseURL+"/articles",
		map[string]string{"api-key": p.Creds.get(envDevToKey)}, body, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Platform: PlatformDevTo, PlatformID: itoa(out.ID), URL: out.URL}, nil
}

func (p *DevToPublisher) Update(ctx context.Context, id string, c Content, m PlatformMetadata) (PublicationResult, error) {
	body := map[string]devtoArticle{"article": {Title: m.Title, BodyMarkdown: m.Body, Published: m.Visibility != "draft", Tags: topStrings(m.Tags, devtoTagMax), CanonicalURL: m.CanonicalURL, MainImage: m.CoverImage}}
	var out devtoResp
	_, err := doJSON(ctx, p.HTTP, "PUT", p.BaseURL+"/articles/"+id,
		map[string]string{"api-key": p.Creds.get(envDevToKey)}, body, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Platform: PlatformDevTo, PlatformID: id, URL: out.URL}, nil
}

// Delete — Dev.to has no delete endpoint; unpublish by setting published=false.
func (p *DevToPublisher) Delete(ctx context.Context, id string) error {
	body := map[string]devtoArticle{"article": {Published: false}}
	_, err := doJSON(ctx, p.HTTP, "PUT", p.BaseURL+"/articles/"+id,
		map[string]string{"api-key": p.Creds.get(envDevToKey)}, body, nil)
	return err
}

func (p *DevToPublisher) GetStatus(ctx context.Context, id string) (string, error) {
	var out struct {
		Published bool `json:"published"`
	}
	_, err := doJSON(ctx, p.HTTP, "GET", p.BaseURL+"/articles/"+id,
		map[string]string{"api-key": p.Creds.get(envDevToKey)}, nil, &out)
	if err != nil {
		return "", err
	}
	if out.Published {
		return "published", nil
	}
	return "draft", nil
}
