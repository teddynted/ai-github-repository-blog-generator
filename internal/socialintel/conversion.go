package socialintel

import "sort"

// SubscriberConversion analyzes how effectively each YouTube video acquired
// subscribers, ranking all videos by net subscriber effectiveness. Grounded in
// the per-video subscribersGained/lost snapshots — never estimated.
func (a Analytics) SubscriberConversion(p Platform, date Date) []SubscriberConversion {
	snaps := a.Repo.ContentSnapshots(p, date)
	out := make([]SubscriberConversion, 0, len(snaps))
	for _, s := range snaps {
		if s.Kind != KindVideo && s.Kind != KindShort {
			continue
		}
		net := s.SubscribersGained - s.SubscribersLost
		conv := 0.0
		vps := 0.0
		if s.Views > 0 {
			conv = round2(float64(net) / float64(s.Views) * 100)
		}
		if net > 0 {
			vps = round2(float64(s.Views) / float64(net))
		}
		out = append(out, SubscriberConversion{
			ContentID:         s.ContentID,
			Title:             s.Title,
			Views:             s.Views,
			SubscribersGained: s.SubscribersGained,
			SubscribersLost:   s.SubscribersLost,
			NetSubscribers:    net,
			ConversionRate:    conv,
			ViewsPerSub:       vps,
		})
	}
	// Rank by net subscribers, then conversion rate.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NetSubscribers != out[j].NetSubscribers {
			return out[i].NetSubscribers > out[j].NetSubscribers
		}
		return out[i].ConversionRate > out[j].ConversionRate
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// BestSubscriberVideos / WorstSubscriberVideos return the top/bottom of the
// ranking.
func BestSubscriberVideos(conv []SubscriberConversion, n int) []SubscriberConversion {
	return topN(conv, n)
}

func WorstSubscriberVideos(conv []SubscriberConversion, n int) []SubscriberConversion {
	if len(conv) <= n {
		return conv
	}
	worst := append([]SubscriberConversion{}, conv[len(conv)-n:]...)
	// Reverse so the worst is first.
	for i, j := 0, len(worst)-1; i < j; i, j = i+1, j-1 {
		worst[i], worst[j] = worst[j], worst[i]
	}
	return worst
}
