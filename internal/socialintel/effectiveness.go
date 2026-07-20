package socialintel

import "sort"

// Effectiveness ranks content by a blend of engagement, virality, evergreen
// staying power, and retention. All inputs come from stored snapshots.
func (a Analytics) Effectiveness(p Platform, date Date) []ContentEffectiveness {
	snaps := a.Repo.ContentSnapshots(p, date)
	out := make([]ContentEffectiveness, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, a.scoreContent(s))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func (a Analytics) scoreContent(s ContentSnapshot) ContentEffectiveness {
	eng := engagementRate(s)
	vir := viralityScore(s)
	ever := a.evergreenScore(s)
	ret := s.RetentionPct

	// Overall: engagement + virality + evergreen + retention, weighted.
	score := clampFloat(eng*2.0, 0, 40) + vir*0.3 + ever*0.2 + ret*0.1
	return ContentEffectiveness{
		ContentID:      s.ContentID,
		Platform:       s.Platform,
		Kind:           s.Kind,
		Title:          s.Title,
		Views:          s.Views,
		EngagementRate: round2(eng),
		ViralityScore:  round2(vir),
		EvergreenScore: round2(ever),
		RetentionPct:   ret,
		Score:          round2(clampFloat(score, 0, 100)),
	}
}

// engagementRate is (likes+comments+shares+saves)/views as a percentage.
func engagementRate(s ContentSnapshot) float64 {
	if s.Views == 0 {
		return 0
	}
	eng := float64(s.Likes + s.Comments + s.Shares + s.Saves)
	return round2(eng / float64(s.Views) * 100)
}

// viralityScore rewards shares/saves relative to views (0–100). Shares are the
// strongest virality signal.
func viralityScore(s ContentSnapshot) float64 {
	if s.Views == 0 {
		return 0
	}
	shareRate := float64(s.Shares) / float64(s.Views)
	saveRate := float64(s.Saves) / float64(s.Views)
	return clampFloat((shareRate*3+saveRate)*100*20, 0, 100)
}

// evergreenScore rewards content that keeps accruing views across snapshots
// (sustained daily view velocity) rather than spiking and dying.
func (a Analytics) evergreenScore(s ContentSnapshot) float64 {
	hist := a.Repo.ContentHistory(s.Platform, s.ContentID)
	if len(hist) < 3 {
		return 30 // not enough history to be considered evergreen yet
	}
	// Average daily view gain over the second half of the history.
	mid := len(hist) / 2
	early := float64(hist[mid].Views - hist[0].Views)
	recent := float64(hist[len(hist)-1].Views - hist[mid].Views)
	if early <= 0 {
		if recent > 0 {
			return 60
		}
		return 20
	}
	ratio := recent / early
	// A ratio near/above 1 means views are still coming in → evergreen.
	return clampFloat(ratio*60, 0, 100)
}

// TrendingTags aggregates the most-used tags across a platform's content for a
// date (grounded — only tags that actually appear).
func (a Analytics) TrendingTags(p Platform, date Date, n int) []string {
	counts := map[string]int{}
	order := []string{}
	for _, s := range a.Repo.ContentSnapshots(p, date) {
		for _, t := range s.Tags {
			if counts[t] == 0 {
				order = append(order, t)
			}
			counts[t]++
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	return topN(order, n)
}
