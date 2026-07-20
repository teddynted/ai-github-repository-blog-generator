package socialintel

import (
	"context"
	"time"
)

// TikTokProvider collects account + per-video metrics via the TikTok APIs
// (Display / Business). Credentials come from the environment.
type TikTokProvider struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://open.tiktokapis.com/v2
}

func NewTikTokProvider(http HTTPDoer, creds Credentials) *TikTokProvider {
	return &TikTokProvider{HTTP: http, Creds: creds, BaseURL: "https://open.tiktokapis.com/v2"}
}

func (p *TikTokProvider) Name() Platform { return PlatformTikTok }

func (p *TikTokProvider) auth() (map[string]string, error) {
	if !p.Creds.has(envTikTokToken) {
		return nil, errAuth("TIKTOK_ACCESS_TOKEN is not set")
	}
	return map[string]string{"Authorization": "Bearer " + p.Creds.get(envTikTokToken)}, nil
}

type ttUserResp struct {
	Data struct {
		User struct {
			FollowerCount  int64 `json:"follower_count"`
			FollowingCount int64 `json:"following_count"`
			LikesCount     int64 `json:"likes_count"`
			VideoCount     int64 `json:"video_count"`
		} `json:"user"`
	} `json:"data"`
}

func (p *TikTokProvider) CollectAccount(ctx context.Context, date Date) (AccountSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return AccountSnapshot{}, err
	}
	url := p.BaseURL + "/user/info/?fields=follower_count,following_count,likes_count,video_count"
	var out ttUserResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return AccountSnapshot{}, err
	}
	u := out.Data.User
	return AccountSnapshot{
		Platform:    PlatformTikTok,
		Date:        date,
		Followers:   u.FollowerCount,
		Following:   u.FollowingCount,
		Extra:       map[string]float64{"totalLikes": float64(u.LikesCount)},
		CollectedAt: time.Now(),
	}, nil
}

type ttVideoResp struct {
	Data struct {
		Videos []struct {
			ID           string `json:"id"`
			Title        string `json:"video_description"`
			CreateTime   int64  `json:"create_time"`
			ViewCount    int64  `json:"view_count"`
			LikeCount    int64  `json:"like_count"`
			CommentCount int64  `json:"comment_count"`
			ShareCount   int64  `json:"share_count"`
		} `json:"videos"`
	} `json:"data"`
}

func (p *TikTokProvider) CollectContent(ctx context.Context, date Date) ([]ContentSnapshot, error) {
	h, err := p.auth()
	if err != nil {
		return nil, err
	}
	url := p.BaseURL + "/video/list/?fields=id,video_description,create_time,view_count,like_count,comment_count,share_count"
	var out ttVideoResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return nil, err
	}
	snaps := make([]ContentSnapshot, 0, len(out.Data.Videos))
	for _, v := range out.Data.Videos {
		snaps = append(snaps, ContentSnapshot{
			Platform:    PlatformTikTok,
			ContentID:   v.ID,
			Kind:        KindVideo,
			Title:       truncate(v.Title, 80),
			PublishedAt: dateOf(time.Unix(v.CreateTime, 0)),
			Date:        date,
			Views:       v.ViewCount,
			Likes:       v.LikeCount,
			Comments:    v.CommentCount,
			Shares:      v.ShareCount,
			CollectedAt: time.Now(),
		})
	}
	return snaps, nil
}
