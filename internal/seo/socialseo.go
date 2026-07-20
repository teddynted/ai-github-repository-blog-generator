package seo

import "fmt"

// planSocial assembles SEO for social platforms and developer communities. Each
// item is grounded in the release summary and the keyword taxonomy; summaries
// are bounded for readability. LinkedIn/X reuse the YouTube LinkedIn/Shorts copy
// where available.
func planSocial(pkg ReleasePackage, k Keywords, tags PlatformTags) SocialSEO {
	repo := repoShortName(pkg)
	tag := releaseTag(pkg)
	sum := summary(pkg)
	feat := lowerFirst(firstSentences(featureName(pkg), 1))

	linkedInSummary := firstNonEmpty(contextLinkedIn(pkg), fmt.Sprintf(
		"%s %s ships %s. Here's how it's built — grounded in the repository's own release analysis.", repo, tag, feat))

	items := []SocialItem{
		{
			Platform:         "LinkedIn",
			Title:            fmt.Sprintf("%s %s: %s", repo, tag, feat),
			Summary:          truncateChars(linkedInSummary, SocialSummaryMax),
			Excerpt:          truncateWords(sum, 40),
			Keywords:         topStrings(k.Primary, 6),
			Hashtags:         tags.LinkedIn,
			EngagementPrompt: "How would you have approached this? I'd love to hear your take.",
		},
		{
			Platform:         "X",
			Title:            fmt.Sprintf("%s %s is out", repo, tag),
			Summary:          truncateChars(fmt.Sprintf("%s %s ships %s. Full write-up + video below. 🧵", repo, tag, feat), SocialSummaryMax),
			Excerpt:          truncateWords(sum, 25),
			Keywords:         topStrings(k.Primary, 5),
			Hashtags:         tags.X,
			EngagementPrompt: "What would you build with this?",
		},
		{
			Platform:         "Facebook",
			Title:            fmt.Sprintf("%s %s — new release", repo, tag),
			Summary:          truncateChars(sum, SocialSummaryMax),
			Excerpt:          truncateWords(sum, 40),
			Keywords:         topStrings(k.Primary, 5),
			Hashtags:         topStrings(tags.All, 5),
			EngagementPrompt: "Have you tried something like this?",
		},
		{
			Platform:         "GitHub",
			Title:            fmt.Sprintf("Release %s", tag),
			Summary:          truncateChars(sum, SocialSummaryMax),
			Excerpt:          truncateWords(sum, 60),
			Keywords:         topStrings(k.Primary, 6),
			Hashtags:         nil, // GitHub releases don't use hashtags
			EngagementPrompt: "Issues and PRs welcome.",
		},
		{
			Platform:         "Developer Communities",
			Title:            fmt.Sprintf("How %s builds %s", repo, feat),
			Summary:          truncateChars(fmt.Sprintf("A grounded, end-to-end look at %s in %s %s.", feat, repo, tag), SocialSummaryMax),
			Excerpt:          truncateWords(sum, 50),
			Keywords:         topStrings(k.Technical, 6),
			Hashtags:         topStrings(tags.All, 4),
			EngagementPrompt: "Curious what the community would optimize first.",
		},
	}
	return SocialSEO{Items: items}
}

// contextLinkedIn reuses a LinkedIn post the release context may already carry.
func contextLinkedIn(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.LinkedInPost != "" {
		return firstSentences(pkg.Context.ContentIntelligence.LinkedInPost, 2)
	}
	return ""
}
