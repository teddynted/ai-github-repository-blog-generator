package contentanalytics

import (
	"context"
	"time"
)

// DevToProvider collects article analytics via the Dev.to (Forem) API. The
// authenticated /articles/{id} endpoint returns page views, public reactions,
// and comment counts for the owner's articles.
type DevToProvider struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://dev.to/api
}

func NewDevToProvider(http HTTPDoer, creds Credentials) *DevToProvider {
	return &DevToProvider{HTTP: http, Creds: creds, BaseURL: "https://dev.to/api"}
}

func (p *DevToProvider) Name() Platform { return PlatformDevTo }

func (p *DevToProvider) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeArticle
}

func (p *DevToProvider) headers() (map[string]string, error) {
	if !p.Creds.has(envDevToKey) {
		return nil, errAuth("DEVTO_API_KEY is not set")
	}
	return map[string]string{"api-key": p.Creds.get(envDevToKey)}, nil
}

type devtoArticleResp struct {
	ID                   int64  `json:"id"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	PageViewsCount       int64  `json:"page_views_count"`
	PublicReactionsCount int64  `json:"public_reactions_count"`
	CommentsCount        int64  `json:"comments_count"`
	PublishedAt          string `json:"published_at"`
	EditedAt             string `json:"edited_at"`
}

func (p *DevToProvider) FetchPublication(ctx context.Context, pub Publication) (Publication, error) {
	h, err := p.headers()
	if err != nil {
		return pub, err
	}
	var out devtoArticleResp
	if err := getJSON(ctx, p.HTTP, p.BaseURL+"/articles/"+idOf(pub), h, &out); err != nil {
		return pub, err
	}
	if out.URL != "" {
		pub.URL = out.URL
	}
	if out.Title != "" {
		pub.Title = out.Title
	}
	pub.Status = "published"
	if t := parseRFC3339(out.PublishedAt); !t.IsZero() {
		pub.PublishedAt = t
	}
	if t := parseRFC3339(out.EditedAt); !t.IsZero() {
		pub.LastUpdated = t
	}
	return pub, nil
}

func (p *DevToProvider) FetchMetrics(ctx context.Context, pub Publication, date Date) (MetricsSnapshot, error) {
	h, err := p.headers()
	if err != nil {
		return MetricsSnapshot{}, err
	}
	var out devtoArticleResp
	if err := getJSON(ctx, p.HTTP, p.BaseURL+"/articles/"+idOf(pub), h, &out); err != nil {
		return MetricsSnapshot{}, err
	}
	s := MetricsSnapshot{
		Reach:       Reach{Views: out.PageViewsCount},
		Engagement:  Engagement{Reactions: out.PublicReactionsCount, Comments: out.CommentsCount},
		CollectedAt: time.Now(),
	}
	return finalizeSnapshot(s, PlatformDevTo, pub, date), nil
}

// idOf returns the platform-side id (falls back to content id).
func idOf(pub Publication) string {
	if pub.PlatformID != "" {
		return pub.PlatformID
	}
	return pub.ContentID
}

func parseRFC3339(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
