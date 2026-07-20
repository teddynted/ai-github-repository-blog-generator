package socialintel

import (
	"context"
	"time"
)

// YouTubeProvider collects account + per-video metrics via the YouTube Data API
// v3 (channels.list) and Analytics API. It builds requests via the HTTPDoer so
// it is testable with a mock. Credentials come from the environment.
type YouTubeProvider struct {
	HTTP     HTTPDoer
	Creds    Credentials
	DataURL  string // default https://www.googleapis.com/youtube/v3
	VideoIDs []string
}

func NewYouTubeProvider(http HTTPDoer, creds Credentials, videoIDs []string) *YouTubeProvider {
	return &YouTubeProvider{HTTP: http, Creds: creds, DataURL: "https://www.googleapis.com/youtube/v3", VideoIDs: videoIDs}
}

func (p *YouTubeProvider) Name() Platform { return PlatformYouTube }

func (p *YouTubeProvider) auth() (map[string]string, error) {
	if !p.Creds.has(envYouTubeToken) {
		return nil, errAuth("YOUTUBE_ACCESS_TOKEN is not set")
	}
	return map[string]string{"Authorization": "Bearer " + p.Creds.get(envYouTubeToken)}, nil
}

type ytChannelResp struct {
	Items []struct {
		Statistics struct {
			SubscriberCount string `json:"subscriberCount"`
			ViewCount       string `json:"viewCount"`
			VideoCount      string `json:"videoCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func (p *YouTubeProvider) CollectAccount(ctx context.Context, date Date) (AccountSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return AccountSnapshot{}, err
	}
	var out ytChannelResp
	if err := getJSON(ctx, p.HTTP, p.DataURL+"/channels?part=statistics&mine=true", h, &out); err != nil {
		return AccountSnapshot{}, err
	}
	if len(out.Items) == 0 {
		return AccountSnapshot{}, errPartial("no channel returned")
	}
	st := out.Items[0].Statistics
	return AccountSnapshot{
		Platform:    PlatformYouTube,
		Date:        date,
		Followers:   atoiSafe(st.SubscriberCount),
		Views:       atoiSafe(st.ViewCount),
		CollectedAt: time.Now(),
	}, nil
}

type ytVideoResp struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string   `json:"title"`
			PublishedAt string   `json:"publishedAt"`
			Tags        []string `json:"tags"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func (p *YouTubeProvider) CollectContent(ctx context.Context, date Date) ([]ContentSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return nil, err
	}
	if len(p.VideoIDs) == 0 {
		return nil, nil // nothing to collect (account-only run)
	}
	url := p.DataURL + "/videos?part=snippet,statistics&id=" + join(p.VideoIDs)
	var out ytVideoResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return nil, err
	}
	snaps := make([]ContentSnapshot, 0, len(out.Items))
	for _, v := range out.Items {
		snaps = append(snaps, ContentSnapshot{
			Platform:    PlatformYouTube,
			ContentID:   v.ID,
			Kind:        KindVideo,
			Title:       v.Snippet.Title,
			PublishedAt: dateFromRFC3339(v.Snippet.PublishedAt),
			Date:        date,
			Views:       atoiSafe(v.Statistics.ViewCount),
			Likes:       atoiSafe(v.Statistics.LikeCount),
			Comments:    atoiSafe(v.Statistics.CommentCount),
			Tags:        v.Snippet.Tags,
			CollectedAt: time.Now(),
		})
	}
	return snaps, nil
}

func dateFromRFC3339(s string) Date {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return dateOf(t)
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return ""
}
