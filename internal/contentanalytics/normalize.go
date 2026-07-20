package contentanalytics

import (
	"context"
	"errors"
	"time"
)

// composeMetrics builds a normalized snapshot from a provider's finer-grained
// capabilities (FetchReach/FetchEngagement/FetchWatchTime/FetchAudience). A
// provider that does not implement a capability contributes zero for it and the
// snapshot is flagged Partial — metrics are never fabricated. This lets a
// provider implement only what its platform exposes while the collector still
// gets one unified snapshot.
func composeMetrics(ctx context.Context, prov AnalyticsProvider, p Publication, date Date) (MetricsSnapshot, error) {
	s := MetricsSnapshot{Platform: prov.Name(), PublicationID: p.ID, ContentID: p.ContentID, Date: date}
	got := false

	if rf, ok := prov.(ReachFetcher); ok {
		r, err := rf.FetchReach(ctx, p, date)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			return s, err
		}
		if err == nil {
			s.Reach, got = r, true
		} else {
			s.Partial = true
		}
	}
	if ef, ok := prov.(EngagementFetcher); ok {
		e, err := ef.FetchEngagement(ctx, p, date)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			return s, err
		}
		if err == nil {
			s.Engagement, got = e, true
		} else {
			s.Partial = true
		}
	}
	if wf, ok := prov.(WatchFetcher); ok {
		w, err := wf.FetchWatchTime(ctx, p, date)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			return s, err
		}
		if err == nil {
			s.Watch = w
		} else {
			s.Partial = true
		}
	}
	if af, ok := prov.(AudienceFetcher); ok {
		g, ts, err := af.FetchAudience(ctx, p, date)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			return s, err
		}
		if err == nil {
			s.Growth, s.TrafficSources = g, ts
		} else {
			s.Partial = true
		}
	}
	if !got {
		return s, ErrUnsupported
	}
	return finalizeSnapshot(s, prov.Name(), p, date), nil
}

// finalizeSnapshot stamps identity fields, derives CTR when a platform reports
// clicks but not CTR, and clamps rate fields to sane ranges. It only derives
// from present values — it never invents data.
func finalizeSnapshot(s MetricsSnapshot, platform Platform, p Publication, date Date) MetricsSnapshot {
	s.Platform = platform
	s.PublicationID = p.ID
	s.ContentID = p.ContentID
	s.Date = date
	if s.Period == "" {
		s.Period = PeriodDaily
	}
	if s.CollectedAt.IsZero() {
		s.CollectedAt = time.Now()
	}
	// Derive CTR from link clicks / views when the platform didn't report it.
	if s.Click.CTR == 0 && s.Click.LinkClicks > 0 && s.Reach.Views > 0 {
		s.Click.CTR = round2(float64(s.Click.LinkClicks) / float64(s.Reach.Views) * 100)
	}
	s.Watch.RetentionPct = clampFloat(s.Watch.RetentionPct, 0, 100)
	s.Watch.CompletionRate = clampFloat(s.Watch.CompletionRate, 0, 100)
	s.Click.CTR = clampFloat(s.Click.CTR, 0, 100)
	s.Click.ThumbnailCTR = clampFloat(s.Click.ThumbnailCTR, 0, 100)
	return s
}
