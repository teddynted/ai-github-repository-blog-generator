package publishing

import "context"

// MediumPublisher publishes to Medium via its REST API.
// https://github.com/Medium/medium-api-docs — POST /v1/users/{userId}/posts,
// Authorization: Bearer <token>.
type MediumPublisher struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://api.medium.com/v1
}

func NewMediumPublisher(http HTTPDoer, creds Credentials) *MediumPublisher {
	return &MediumPublisher{HTTP: http, Creds: creds, BaseURL: "https://api.medium.com/v1"}
}

func (p *MediumPublisher) Name() Platform { return PlatformMedium }

func (p *MediumPublisher) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeMediumArticle
}

func (p *MediumPublisher) Validate(c Content, m PlatformMetadata) error {
	if !p.Creds.has(envMediumToken) {
		return errAuth("MEDIUM_TOKEN is not set")
	}
	if !p.Creds.has(envMediumUserID) {
		return errAuth("MEDIUM_USER_ID is not set")
	}
	if m.Title == "" || m.Body == "" {
		return errInvalid("Medium post needs a title and content")
	}
	return nil
}

type mediumPost struct {
	Title         string   `json:"title"`
	ContentFormat string   `json:"contentFormat"`
	Content       string   `json:"content"`
	Tags          []string `json:"tags,omitempty"`
	CanonicalURL  string   `json:"canonicalUrl,omitempty"`
	PublishStatus string   `json:"publishStatus"`
}

type mediumResp struct {
	Data struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"data"`
}

func (p *MediumPublisher) Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error) {
	post := mediumPost{
		Title:         m.Title,
		ContentFormat: "markdown",
		Content:       m.Body,
		Tags:          topStrings(m.Tags, mediumTagMax),
		CanonicalURL:  m.CanonicalURL,
		PublishStatus: firstNonEmpty(m.Extra["publishStatus"], "public"),
	}
	url := p.BaseURL + "/users/" + p.Creds.get(envMediumUserID) + "/posts"
	var out mediumResp
	_, err := doJSON(ctx, p.HTTP, "POST", url,
		map[string]string{"Authorization": "Bearer " + p.Creds.get(envMediumToken)}, post, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Platform: PlatformMedium, PlatformID: out.Data.ID, URL: out.Data.URL}, nil
}

// Update — Medium's API does not support editing published posts.
func (p *MediumPublisher) Update(context.Context, string, Content, PlatformMetadata) (PublicationResult, error) {
	return PublicationResult{}, errUnsupported("Medium API does not support updating posts")
}

// Delete — Medium's API does not support deleting posts.
func (p *MediumPublisher) Delete(context.Context, string) error {
	return errUnsupported("Medium API does not support deleting posts")
}

func (p *MediumPublisher) GetStatus(context.Context, string) (string, error) {
	return "published", nil // Medium exposes no post-status read endpoint
}
