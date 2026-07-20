package socialintel

import (
	"context"
	"time"
)

// InstagramProvider collects account + per-post metrics via the Instagram Graph
// API. Credentials come from the environment.
type InstagramProvider struct {
	HTTP     HTTPDoer
	Creds    Credentials
	BaseURL  string // default https://graph.facebook.com/v19.0
	MediaIDs []string
}

func NewInstagramProvider(http HTTPDoer, creds Credentials, mediaIDs []string) *InstagramProvider {
	return &InstagramProvider{HTTP: http, Creds: creds, BaseURL: "https://graph.facebook.com/v19.0", MediaIDs: mediaIDs}
}

func (p *InstagramProvider) Name() Platform { return PlatformInstagram }

func (p *InstagramProvider) token() (string, error) {
	if !p.Creds.has(envInstagramToken) {
		return "", errAuth("INSTAGRAM_ACCESS_TOKEN is not set")
	}
	if !p.Creds.has(envInstagramUser) {
		return "", errAuth("INSTAGRAM_USER_ID is not set")
	}
	return p.Creds.get(envInstagramToken), nil
}

type igAccountResp struct {
	FollowersCount int64 `json:"followers_count"`
	FollowsCount   int64 `json:"follows_count"`
	MediaCount     int64 `json:"media_count"`
}

func (p *InstagramProvider) CollectAccount(ctx context.Context, date Date) (AccountSnapshot, error) {
	tok, err := p.token()
	if err != nil {
		return AccountSnapshot{}, err
	}
	url := p.BaseURL + "/" + p.Creds.get(envInstagramUser) + "?fields=followers_count,follows_count,media_count&access_token=" + tok
	var out igAccountResp
	if err := getJSON(ctx, p.HTTP, url, nil, &out); err != nil {
		return AccountSnapshot{}, err
	}
	return AccountSnapshot{
		Platform:    PlatformInstagram,
		Date:        date,
		Followers:   out.FollowersCount,
		Following:   out.FollowsCount,
		CollectedAt: time.Now(),
	}, nil
}

type igMediaResp struct {
	ID            string `json:"id"`
	Caption       string `json:"caption"`
	Timestamp     string `json:"timestamp"`
	LikeCount     int64  `json:"like_count"`
	CommentsCount int64  `json:"comments_count"`
}

func (p *InstagramProvider) CollectContent(ctx context.Context, date Date) ([]ContentSnapshot, error) {
	tok, err := p.token()
	if err != nil {
		return nil, err
	}
	var snaps []ContentSnapshot
	for _, id := range p.MediaIDs {
		url := p.BaseURL + "/" + id + "?fields=id,caption,timestamp,like_count,comments_count&access_token=" + tok
		var m igMediaResp
		if err := getJSON(ctx, p.HTTP, url, nil, &m); err != nil {
			return nil, err
		}
		snaps = append(snaps, ContentSnapshot{
			Platform:    PlatformInstagram,
			ContentID:   m.ID,
			Kind:        KindPost,
			Title:       truncate(m.Caption, 80),
			PublishedAt: dateFromRFC3339(m.Timestamp),
			Date:        date,
			Likes:       m.LikeCount,
			Comments:    m.CommentsCount,
			CollectedAt: time.Now(),
		})
	}
	return snaps, nil
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
