package contentanalytics

import (
	"context"
	"time"
)

// MediumProvider collects article analytics for Medium. Medium does NOT offer a
// public analytics API, so this adapter reads a stats feed (JSON) whose URL is
// configured via MEDIUM_STATS_URL — e.g. an exported stats endpoint or a
// self-hosted proxy. The response is normalized into the unified model. The
// adapter is structured exactly like the API-backed providers so that, if Medium
// ships an analytics API, only the request URL changes.
type MediumProvider struct {
	HTTP     HTTPDoer
	Creds    Credentials
	StatsURL string // per-provider override; else MEDIUM_STATS_URL
}

func NewMediumProvider(http HTTPDoer, creds Credentials) *MediumProvider {
	return &MediumProvider{HTTP: http, Creds: creds}
}

func (p *MediumProvider) Name() Platform { return PlatformMedium }

func (p *MediumProvider) Supports(t ContentType) bool {
	return t == TypeBlog || t == TypeArticle
}

func (p *MediumProvider) statsURL() (string, error) {
	if p.StatsURL != "" {
		return p.StatsURL, nil
	}
	if u := p.Creds.get(envMediumStatsURL); u != "" {
		return u, nil
	}
	return "", errMissing("MEDIUM_STATS_URL is not set (Medium has no public analytics API)")
}

func (p *MediumProvider) headers() map[string]string {
	if tok := p.Creds.get(envMediumToken); tok != "" {
		return map[string]string{"Authorization": "Bearer " + tok}
	}
	return nil
}

// mediumStat is the normalized stats shape the feed is expected to return per post.
type mediumStat struct {
	PostID   string `json:"postId"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Views    int64  `json:"views"`
	Reads    int64  `json:"reads"`
	Fans     int64  `json:"fans"`  // claps-givers
	Claps    int64  `json:"claps"` // total claps → reactions
	Comments int64  `json:"comments"`
}

type mediumStatsResp struct {
	Posts []mediumStat `json:"posts"`
}

func (p *MediumProvider) fetch(ctx context.Context, pub Publication) (mediumStat, error) {
	url, err := p.statsURL()
	if err != nil {
		return mediumStat{}, err
	}
	var out mediumStatsResp
	if err := getJSON(ctx, p.HTTP, url, p.headers(), &out); err != nil {
		return mediumStat{}, err
	}
	want := idOf(pub)
	for _, st := range out.Posts {
		if st.PostID == want {
			return st, nil
		}
	}
	return mediumStat{}, errMissing("medium: no stats for post " + want)
}

func (p *MediumProvider) FetchPublication(ctx context.Context, pub Publication) (Publication, error) {
	st, err := p.fetch(ctx, pub)
	if err != nil {
		return pub, err
	}
	if st.URL != "" {
		pub.URL = st.URL
	}
	if st.Title != "" {
		pub.Title = st.Title
	}
	pub.Status = "published"
	return pub, nil
}

func (p *MediumProvider) FetchMetrics(ctx context.Context, pub Publication, date Date) (MetricsSnapshot, error) {
	st, err := p.fetch(ctx, pub)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	s := MetricsSnapshot{
		Reach:       Reach{Views: st.Views, UniqueViewers: st.Reads},
		Engagement:  Engagement{Reactions: st.Claps, Comments: st.Comments},
		CollectedAt: time.Now(),
	}
	// Medium "read ratio" maps naturally to completion rate.
	if st.Views > 0 {
		s.Watch.CompletionRate = round2(float64(st.Reads) / float64(st.Views) * 100)
	}
	return finalizeSnapshot(s, PlatformMedium, pub, date), nil
}
