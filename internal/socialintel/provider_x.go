package socialintel

import (
	"context"
	"time"
)

// XProvider collects account + per-post metrics via the X API v2. Credentials
// come from the environment.
type XProvider struct {
	HTTP     HTTPDoer
	Creds    Credentials
	BaseURL  string // default https://api.twitter.com/2
	TweetIDs []string
}

func NewXProvider(http HTTPDoer, creds Credentials, tweetIDs []string) *XProvider {
	return &XProvider{HTTP: http, Creds: creds, BaseURL: "https://api.twitter.com/2", TweetIDs: tweetIDs}
}

func (p *XProvider) Name() Platform { return PlatformX }

func (p *XProvider) auth() (map[string]string, error) {
	if !p.Creds.has(envXBearer) {
		return nil, errAuth("X_BEARER_TOKEN is not set")
	}
	if !p.Creds.has(envXUserID) {
		return nil, errAuth("X_USER_ID is not set")
	}
	return map[string]string{"Authorization": "Bearer " + p.Creds.get(envXBearer)}, nil
}

type xUserResp struct {
	Data struct {
		PublicMetrics struct {
			FollowersCount int64 `json:"followers_count"`
			FollowingCount int64 `json:"following_count"`
			TweetCount     int64 `json:"tweet_count"`
		} `json:"public_metrics"`
	} `json:"data"`
}

func (p *XProvider) CollectAccount(ctx context.Context, date Date) (AccountSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return AccountSnapshot{}, err
	}
	url := p.BaseURL + "/users/" + p.Creds.get(envXUserID) + "?user.fields=public_metrics"
	var out xUserResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return AccountSnapshot{}, err
	}
	m := out.Data.PublicMetrics
	return AccountSnapshot{
		Platform:    PlatformX,
		Date:        date,
		Followers:   m.FollowersCount,
		Following:   m.FollowingCount,
		CollectedAt: time.Now(),
	}, nil
}

type xTweetsResp struct {
	Data []struct {
		ID            string `json:"id"`
		Text          string `json:"text"`
		CreatedAt     string `json:"created_at"`
		PublicMetrics struct {
			LikeCount       int64 `json:"like_count"`
			ReplyCount      int64 `json:"reply_count"`
			RetweetCount    int64 `json:"retweet_count"`
			QuoteCount      int64 `json:"quote_count"`
			ImpressionCount int64 `json:"impression_count"`
			BookmarkCount   int64 `json:"bookmark_count"`
		} `json:"public_metrics"`
	} `json:"data"`
}

func (p *XProvider) CollectContent(ctx context.Context, date Date) ([]ContentSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return nil, err
	}
	if len(p.TweetIDs) == 0 {
		return nil, nil
	}
	url := p.BaseURL + "/tweets?ids=" + join(p.TweetIDs) + "&tweet.fields=public_metrics,created_at"
	var out xTweetsResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return nil, err
	}
	snaps := make([]ContentSnapshot, 0, len(out.Data))
	for _, tw := range out.Data {
		m := tw.PublicMetrics
		snaps = append(snaps, ContentSnapshot{
			Platform:    PlatformX,
			ContentID:   tw.ID,
			Kind:        KindPost,
			Title:       truncate(tw.Text, 80),
			PublishedAt: dateFromRFC3339(tw.CreatedAt),
			Date:        date,
			Likes:       m.LikeCount,
			Comments:    m.ReplyCount,
			Shares:      m.RetweetCount + m.QuoteCount,
			Saves:       m.BookmarkCount,
			Impressions: m.ImpressionCount,
			CollectedAt: time.Now(),
		})
	}
	return snaps, nil
}
