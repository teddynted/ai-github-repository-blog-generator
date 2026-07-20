package contentanalytics

import (
	"context"
	"time"
)

// YouTubeProvider collects video analytics via two APIs: the YouTube Data API
// v3 (public statistics: views, likes, comments) and the YouTube Analytics API
// (watch time, average view duration, audience retention). The Analytics call is
// best-effort — if the OAuth token is absent or the call fails as unsupported,
// the snapshot is flagged Partial rather than fabricating watch metrics.
type YouTubeProvider struct {
	HTTP         HTTPDoer
	Creds        Credentials
	DataURL      string // default https://www.googleapis.com/youtube/v3
	AnalyticsURL string // default https://youtubeanalytics.googleapis.com/v2
}

func NewYouTubeProvider(http HTTPDoer, creds Credentials) *YouTubeProvider {
	return &YouTubeProvider{
		HTTP:         http,
		Creds:        creds,
		DataURL:      "https://www.googleapis.com/youtube/v3",
		AnalyticsURL: "https://youtubeanalytics.googleapis.com/v2",
	}
}

func (p *YouTubeProvider) Name() Platform { return PlatformYouTube }

func (p *YouTubeProvider) Supports(t ContentType) bool {
	return t == TypeYouTubeVideo || t == TypeYouTubeShorts
}

type ytVideoResp struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			PublishedAt string `json:"publishedAt"`
		} `json:"snippet"`
		Statistics struct {
			ViewCount     string `json:"viewCount"`
			LikeCount     string `json:"likeCount"`
			CommentCount  string `json:"commentCount"`
			FavoriteCount string `json:"favoriteCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func (p *YouTubeProvider) fetchData(ctx context.Context, pub Publication) (ytVideoResp, error) {
	if !p.Creds.has(envYouTubeKey) {
		return ytVideoResp{}, errAuth("YOUTUBE_API_KEY is not set")
	}
	url := p.DataURL + "/videos?part=statistics,snippet&id=" + idOf(pub) + "&key=" + p.Creds.get(envYouTubeKey)
	var out ytVideoResp
	if err := getJSON(ctx, p.HTTP, url, nil, &out); err != nil {
		return out, err
	}
	if len(out.Items) == 0 {
		return out, errMissing("youtube: video not found: " + idOf(pub))
	}
	return out, nil
}

type ytAnalyticsResp struct {
	Rows [][]float64 `json:"rows"`
}

// fetchWatch retrieves watch metrics via the Analytics API. Returns ErrUnsupported
// when no OAuth token is configured so the collector marks the snapshot partial.
func (p *YouTubeProvider) fetchWatch(ctx context.Context, pub Publication, date Date) (Watch, error) {
	if !p.Creds.has(envYouTubeOAuth) {
		return Watch{}, ErrUnsupported
	}
	url := p.AnalyticsURL + "/reports?ids=channel==MINE" +
		"&metrics=estimatedMinutesWatched,averageViewDuration,averageViewPercentage" +
		"&filters=video==" + idOf(pub) +
		"&startDate=" + addDays(date, -1) + "&endDate=" + date
	h := map[string]string{"Authorization": "Bearer " + p.Creds.get(envYouTubeOAuth)}
	var out ytAnalyticsResp
	if err := getJSON(ctx, p.HTTP, url, h, &out); err != nil {
		return Watch{}, err
	}
	if len(out.Rows) == 0 || len(out.Rows[0]) < 3 {
		return Watch{}, ErrUnsupported
	}
	row := out.Rows[0]
	return Watch{
		WatchTimeMinutes: int64(row[0]),
		AvgViewSeconds:   round2(row[1]),
		RetentionPct:     round2(row[2]),
		CompletionRate:   round2(row[2]),
	}, nil
}

func (p *YouTubeProvider) FetchPublication(ctx context.Context, pub Publication) (Publication, error) {
	out, err := p.fetchData(ctx, pub)
	if err != nil {
		return pub, err
	}
	item := out.Items[0]
	if item.Snippet.Title != "" {
		pub.Title = item.Snippet.Title
	}
	if pub.URL == "" {
		pub.URL = "https://www.youtube.com/watch?v=" + idOf(pub)
	}
	pub.Status = "published"
	if t := parseRFC3339(item.Snippet.PublishedAt); !t.IsZero() {
		pub.PublishedAt = t
	}
	return pub, nil
}

func (p *YouTubeProvider) FetchMetrics(ctx context.Context, pub Publication, date Date) (MetricsSnapshot, error) {
	out, err := p.fetchData(ctx, pub)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	st := out.Items[0].Statistics
	s := MetricsSnapshot{
		Reach:       Reach{Views: atoiSafe(st.ViewCount)},
		Engagement:  Engagement{Likes: atoiSafe(st.LikeCount), Comments: atoiSafe(st.CommentCount)},
		CollectedAt: time.Now(),
	}
	// Best-effort watch metrics; absence marks the snapshot partial, not fabricated.
	if w, werr := p.fetchWatch(ctx, pub, date); werr == nil {
		s.Watch = w
	} else if isRecoverable(werr) {
		return MetricsSnapshot{}, werr // a transient analytics failure should retry the whole item
	} else {
		s.Partial = true
	}
	return finalizeSnapshot(s, PlatformYouTube, pub, date), nil
}
