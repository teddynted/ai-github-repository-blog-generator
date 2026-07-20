package publishing

import (
	"context"
	"strings"
)

// HashnodePublisher publishes to Hashnode via its GraphQL API.
// https://apidocs.hashnode.com — POST https://gql.hashnode.com/ with the
// publishPost mutation, Authorization: <personal access token>.
type HashnodePublisher struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://gql.hashnode.com/
}

func NewHashnodePublisher(http HTTPDoer, creds Credentials) *HashnodePublisher {
	return &HashnodePublisher{HTTP: http, Creds: creds, BaseURL: "https://gql.hashnode.com/"}
}

func (p *HashnodePublisher) Name() Platform { return PlatformHashnode }

func (p *HashnodePublisher) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeHashnodeArticle
}

func (p *HashnodePublisher) Validate(c Content, m PlatformMetadata) error {
	if !p.Creds.has(envHashnodeToken) {
		return errAuth("HASHNODE_TOKEN is not set")
	}
	if m.Extra["publicationId"] == "" {
		return errInvalid("Hashnode requires a publicationId (metadata.hashnodePublicationId)")
	}
	if m.Title == "" || m.Body == "" {
		return errInvalid("Hashnode post needs a title and content")
	}
	return nil
}

const hashnodeMutation = `mutation PublishPost($input: PublishPostInput!) {
  publishPost(input: $input) { post { id slug url } }
}`

type hashnodeReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type hashnodeResp struct {
	Data struct {
		PublishPost struct {
			Post struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"post"`
		} `json:"publishPost"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *HashnodePublisher) Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error) {
	input := map[string]any{
		"title":           m.Title,
		"publicationId":   m.Extra["publicationId"],
		"contentMarkdown": m.Body,
		"tags":            hashnodeTags(m.Tags),
		"slug":            m.Extra["slug"],
	}
	if m.CoverImage != "" {
		input["coverImageOptions"] = map[string]any{"coverImageURL": m.CoverImage}
	}
	if m.CanonicalURL != "" {
		input["originalArticleURL"] = m.CanonicalURL
	}
	req := hashnodeReq{Query: hashnodeMutation, Variables: map[string]any{"input": input}}

	var out hashnodeResp
	_, err := doJSON(ctx, p.HTTP, "POST", p.BaseURL,
		map[string]string{"Authorization": p.Creds.get(envHashnodeToken)}, req, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	if len(out.Errors) > 0 {
		return PublicationResult{}, errInvalid("Hashnode GraphQL error: " + out.Errors[0].Message)
	}
	post := out.Data.PublishPost.Post
	if post.ID == "" {
		return PublicationResult{}, errInvalid("Hashnode returned no post id")
	}
	return PublicationResult{Platform: PlatformHashnode, PlatformID: post.ID, URL: post.URL}, nil
}

func (p *HashnodePublisher) Update(context.Context, string, Content, PlatformMetadata) (PublicationResult, error) {
	return PublicationResult{}, errUnsupported("Hashnode update mutation not wired yet")
}

func (p *HashnodePublisher) Delete(context.Context, string) error {
	return errUnsupported("Hashnode delete mutation not wired yet")
}

func (p *HashnodePublisher) GetStatus(context.Context, string) (string, error) {
	return "published", nil
}

// hashnodeTags converts tag names into Hashnode's {name, slug} tag objects.
func hashnodeTags(tags []string) []map[string]string {
	out := make([]map[string]string, 0, len(tags))
	for _, t := range topStrings(tags, hashnodeTagMax) {
		out = append(out, map[string]string{"name": t, "slug": slugify(strings.ToLower(t))})
	}
	return out
}
