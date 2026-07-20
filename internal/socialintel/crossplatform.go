package socialintel

// CrossPlatform compares every platform on the given date and identifies the
// best, fastest-growing, highest-engagement, highest-conversion, and
// highest-retention platforms. Grounded entirely in stored snapshots.
func (a Analytics) CrossPlatform(date Date) CrossPlatform {
	var comps []PlatformComparison
	best := struct {
		byGrowth, byEngage, byConv, byRet Platform
		growth, engage, conv, ret         float64
	}{}

	for _, p := range a.Repo.Platforms() {
		series := a.Repo.AccountSeries(p)
		if len(series) == 0 {
			continue
		}
		latest := series[len(series)-1]
		growth := a.Growth(p, "followers").WeekOverWeek.Percent
		content := a.Repo.ContentSnapshots(p, date)
		engage, conv, ret := aggregateContent(content)
		freq := publishingFreq(series, len(content))

		comps = append(comps, PlatformComparison{
			Platform:          p,
			Followers:         latest.Followers,
			FollowerGrowthPct: growth,
			EngagementRate:    engage,
			ConversionRate:    conv,
			AvgRetentionPct:   ret,
			PublishingFreq:    freq,
		})

		if best.byGrowth == "" || growth > best.growth {
			best.byGrowth, best.growth = p, growth
		}
		if best.byEngage == "" || engage > best.engage {
			best.byEngage, best.engage = p, engage
		}
		if conv > best.conv {
			best.byConv, best.conv = p, conv
		}
		if ret > best.ret {
			best.byRet, best.ret = p, ret
		}
	}

	return CrossPlatform{
		Platforms:         comps,
		BestPlatform:      bestOverall(comps),
		FastestGrowing:    best.byGrowth,
		HighestEngagement: best.byEngage,
		HighestConversion: best.byConv,
		HighestRetention:  best.byRet,
	}
}

// aggregateContent computes average engagement, conversion, and retention.
func aggregateContent(content []ContentSnapshot) (engage, conv, ret float64) {
	if len(content) == 0 {
		return 0, 0, 0
	}
	var totViews, totEng, totNet, totRet float64
	retN := 0
	for _, s := range content {
		totViews += float64(s.Views)
		totEng += float64(s.Likes + s.Comments + s.Shares + s.Saves)
		totNet += float64(s.SubscribersGained - s.SubscribersLost)
		if s.RetentionPct > 0 {
			totRet += s.RetentionPct
			retN++
		}
	}
	if totViews > 0 {
		engage = round2(totEng / totViews * 100)
		conv = round2(totNet / totViews * 100)
	}
	if retN > 0 {
		ret = round2(totRet / float64(retN))
	}
	return engage, conv, ret
}

// publishingFreq estimates content published per day over the snapshot window.
func publishingFreq(series []AccountSnapshot, contentCount int) float64 {
	if len(series) < 2 {
		return float64(contentCount)
	}
	days := daysBetween(series[0].Date, series[len(series)-1].Date)
	if days <= 0 {
		return float64(contentCount)
	}
	return round2(float64(contentCount) / float64(days))
}

// bestOverall picks the platform with the strongest blended standing.
func bestOverall(comps []PlatformComparison) Platform {
	var best Platform
	bestScore := -1e18
	for _, c := range comps {
		score := c.FollowerGrowthPct*2 + c.EngagementRate*3 + c.ConversionRate*4 + c.AvgRetentionPct*0.5
		if score > bestScore {
			bestScore, best = score, c.Platform
		}
	}
	return best
}
