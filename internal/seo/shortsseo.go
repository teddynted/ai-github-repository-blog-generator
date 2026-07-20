package seo

// planShorts assembles SEO for short-form videos by reusing the YouTube Shorts
// (M7) and TikTok (M8) artifacts — their titles, descriptions, captions,
// hashtags, engagement prompts, and CTAs. Nothing is regenerated.
func planShorts(pkg ReleasePackage, k Keywords) ShortsSEO {
	var items []ShortItem

	for _, s := range pkg.Shorts.Shorts {
		items = append(items, ShortItem{
			Platform:    "YouTube Shorts",
			Title:       s.Title,
			Description: firstNonEmpty(s.SEO.Description, s.CoreExplanation),
			Keywords:    topStrings(dedupe(append([]string{s.Angle}, k.Primary...)), 6),
			Hashtags:    s.Hashtags,
			Caption:     firstNonEmpty(s.Hook, s.Title),
			CTA:         s.CTA,
		})
	}

	for _, v := range pkg.TikTok.Videos {
		items = append(items, ShortItem{
			Platform:         "TikTok",
			Title:            v.Title,
			Description:      firstNonEmpty(v.SEO.Caption, v.Takeaway),
			Keywords:         topStrings(dedupe(append([]string{v.Topic}, k.Primary...)), 6),
			Hashtags:         v.Hashtags,
			Caption:          firstNonEmpty(v.SEO.Caption, v.Hook),
			EngagementPrompt: v.EngagementPrompt,
			CTA:              v.CTA,
		})
	}

	// If no short-form artifacts were provided, synthesize one grounded entry so
	// the channel is still represented.
	if len(items) == 0 {
		items = append(items, ShortItem{
			Platform:    "YouTube Shorts",
			Title:       "The architecture in under a minute",
			Description: summary(pkg),
			Keywords:    topStrings(k.Primary, 5),
			Hashtags:    prependHash(topStrings(k.AWS, 3)),
			Caption:     lowerFirst(featureName(pkg)),
			CTA:         "Watch the full video — link in the description.",
		})
	}
	return ShortsSEO{Items: items}
}
