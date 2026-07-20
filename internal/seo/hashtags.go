package seo

// Per-platform hashtag caps (platform norms differ).
const (
	ytHashtagMax       = 5
	tiktokHashtagMax   = 6
	linkedinHashtagMax = 5
	xHashtagMax        = 4
)

// planHashtags builds platform-specific hashtag sets, grounded in the release's
// keywords and reusing what the video generators already produced. It dedupes
// and caps per platform (spammy tag walls hurt reach).
func planHashtags(pkg ReleasePackage, k Keywords) PlatformTags {
	// Grounded tokens shared across platforms.
	grounded := dedupe(append(append([]string{}, k.AWS...), append(k.Technology, k.Primary...)...))
	groundedTags := prependHash(grounded)

	yt := reuseHashtags(youtubeHashtags(pkg), groundedTags, ytHashtagMax)
	tk := reuseHashtags(tiktokHashtags(pkg), append([]string{"#TechTok"}, groundedTags...), tiktokHashtagMax)
	li := reuseHashtags(nil, append([]string{"#SoftwareEngineering", "#CloudComputing", "#AIEngineering"}, groundedTags...), linkedinHashtagMax)
	x := reuseHashtags(nil, append([]string{"#DevOps", "#AWS"}, groundedTags...), xHashtagMax)

	all := dedupe(append(append(append(append([]string{}, yt...), tk...), li...), x...))
	return PlatformTags{YouTube: yt, TikTok: tk, LinkedIn: li, X: x, All: all}
}

// reuseHashtags prefers already-generated hashtags, then fills from grounded
// tags, deduped and capped.
func reuseHashtags(existing, fallback []string, max int) []string {
	tags := dedupe(append(append([]string{}, existing...), fallback...))
	return topStrings(tags, max)
}

func youtubeHashtags(pkg ReleasePackage) []string {
	// The YouTube script itself carries chapter/tag data; hashtags come from tags.
	return prependHash(topStrings(pkg.YouTube.ContentIntelligence.SuggestedTags, 5))
}

func tiktokHashtags(pkg ReleasePackage) []string {
	if len(pkg.TikTok.Videos) > 0 {
		return pkg.TikTok.Videos[0].Hashtags // already #-prefixed and grounded
	}
	if len(pkg.Shorts.Shorts) > 0 {
		return pkg.Shorts.Shorts[0].Hashtags
	}
	return nil
}
