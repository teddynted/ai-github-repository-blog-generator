package socialintel

import (
	"context"
	"time"
)

// SyntheticProvider produces deterministic, grounded-looking metrics for a
// platform. It is used for local demos and tests (no network, no credentials) —
// it never contacts a real API. Metrics grow deterministically with the date so
// trend analysis has something to work with.
type SyntheticProvider struct {
	Platform      Platform
	StartDate     Date
	BaseFollowers int64
	DailyGrowth   int64
	Videos        int
	FailFirst     int // fail the first N account collections with a recoverable error (tests)
	Recoverable   bool
	calls         int
}

// NewSyntheticProvider builds a synthetic provider.
func NewSyntheticProvider(p Platform, start Date, baseFollowers, dailyGrowth int64, videos int) *SyntheticProvider {
	return &SyntheticProvider{Platform: p, StartDate: start, BaseFollowers: baseFollowers, DailyGrowth: dailyGrowth, Videos: videos, Recoverable: true}
}

func (s *SyntheticProvider) Name() Platform { return s.Platform }

func (s *SyntheticProvider) offset(date Date) int64 {
	o := daysBetween(s.StartDate, date)
	if o < 0 {
		o = 0
	}
	return int64(o)
}

func (s *SyntheticProvider) CollectAccount(_ context.Context, date Date) (AccountSnapshot, error) {
	s.calls++
	if s.calls <= s.FailFirst {
		if s.Recoverable {
			return AccountSnapshot{}, errRateLimit("synthetic transient failure")
		}
		return AccountSnapshot{}, errAuth("synthetic auth failure")
	}
	o := s.offset(date)
	followers := s.BaseFollowers + s.DailyGrowth*o
	return AccountSnapshot{
		Platform:    s.Platform,
		Date:        date,
		Followers:   followers,
		Views:       followers * 12,
		Impressions: followers * 20,
		CollectedAt: time.Now(),
	}, nil
}

func (s *SyntheticProvider) CollectContent(_ context.Context, date Date) ([]ContentSnapshot, error) {
	o := s.offset(date)
	var snaps []ContentSnapshot
	for i := 0; i < s.Videos; i++ {
		views := int64(1000+(i*250)) + o*int64(50+i*10)
		likes := views / 20
		kind := KindVideo
		if s.Platform == PlatformInstagram || s.Platform == PlatformX {
			kind = KindPost
		}
		snaps = append(snaps, ContentSnapshot{
			Platform:          s.Platform,
			ContentID:         string(s.Platform) + "-c" + itoa(int64(i+1)),
			Kind:              kind,
			Title:             string(s.Platform) + " content " + itoa(int64(i+1)),
			PublishedAt:       addDays(s.StartDate, i),
			Date:              date,
			Views:             views,
			Likes:             likes,
			Comments:          likes / 4,
			Shares:            likes / 6,
			Saves:             likes / 8,
			Impressions:       views * 3,
			RetentionPct:      float64(45 + i*3),
			SubscribersGained: int64(10 + i*5),
			SubscribersLost:   int64(i),
			Tags:              []string{"aws", "golang", "devops"},
			CollectedAt:       time.Now(),
		})
	}
	return snaps, nil
}
