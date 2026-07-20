package contentanalytics

import (
	"context"
	"time"
)

// HashnodeProvider collects post analytics via the Hashnode GraphQL API
// (https://gql.hashnode.com/). It queries a post's views, reaction count, and
// response (comment) count.
type HashnodeProvider struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://gql.hashnode.com
}

func NewHashnodeProvider(http HTTPDoer, creds Credentials) *HashnodeProvider {
	return &HashnodeProvider{HTTP: http, Creds: creds, BaseURL: "https://gql.hashnode.com"}
}

func (p *HashnodeProvider) Name() Platform { return PlatformHashnode }

func (p *HashnodeProvider) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeArticle
}

func (p *HashnodeProvider) headers() (map[string]string, error) {
	if !p.Creds.has(envHashnodeToken) {
		return nil, errAuth("HASHNODE_TOKEN is not set")
	}
	return map[string]string{"Authorization": p.Creds.get(envHashnodeToken)}, nil
}

const hashnodePostQuery = `query Post($id: ID!) { post(id: $id) { id title url views reactionCount responseCount publishedAt updatedAt } }`

type hashnodePostResp struct {
	Data struct {
		Post struct {
			ID            string `json:"id"`
			Title         string `json:"title"`
			URL           string `json:"url"`
			Views         int64  `json:"views"`
			ReactionCount int64  `json:"reactionCount"`
			ResponseCount int64  `json:"responseCount"`
			PublishedAt   string `json:"publishedAt"`
			UpdatedAt     string `json:"updatedAt"`
		} `json:"post"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *HashnodeProvider) query(ctx context.Context, pub Publication) (hashnodePostResp, error) {
	h, err := p.headers()
	if err != nil {
		return hashnodePostResp{}, err
	}
	body := map[string]any{"query": hashnodePostQuery, "variables": map[string]string{"id": idOf(pub)}}
	var out hashnodePostResp
	if err := postJSON(ctx, p.HTTP, p.BaseURL+"/", h, body, &out); err != nil {
		return out, err
	}
	if len(out.Errors) > 0 {
		return out, errMissing("hashnode: " + out.Errors[0].Message)
	}
	return out, nil
}

func (p *HashnodeProvider) FetchPublication(ctx context.Context, pub Publication) (Publication, error) {
	out, err := p.query(ctx, pub)
	if err != nil {
		return pub, err
	}
	post := out.Data.Post
	if post.URL != "" {
		pub.URL = post.URL
	}
	if post.Title != "" {
		pub.Title = post.Title
	}
	pub.Status = "published"
	if t := parseRFC3339(post.PublishedAt); !t.IsZero() {
		pub.PublishedAt = t
	}
	if t := parseRFC3339(post.UpdatedAt); !t.IsZero() {
		pub.LastUpdated = t
	}
	return pub, nil
}

func (p *HashnodeProvider) FetchMetrics(ctx context.Context, pub Publication, date Date) (MetricsSnapshot, error) {
	out, err := p.query(ctx, pub)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	post := out.Data.Post
	s := MetricsSnapshot{
		Reach:       Reach{Views: post.Views},
		Engagement:  Engagement{Reactions: post.ReactionCount, Comments: post.ResponseCount},
		CollectedAt: time.Now(),
	}
	return finalizeSnapshot(s, PlatformHashnode, pub, date), nil
}
