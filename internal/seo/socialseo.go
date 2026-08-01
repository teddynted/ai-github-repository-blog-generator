package seo

import "fmt"

// planSocial assembles SEO for social platforms and developer communities. Each
// item is grounded in the release summary and the keyword taxonomy; summaries
// are bounded for readability. LinkedIn/X reuse the YouTube LinkedIn/Shorts copy
// where available.
func planSocial(pkg ReleasePackage, k Keywords, tags PlatformTags) SocialSEO {
	tag := releaseTag(pkg)
	// Grounded, topic-led sources — the article's own headline and description —
	// instead of repo-name-led, generic "release is out" templates.
	headline := socialHeadline(pkg)
	base := socialSummaryBase(pkg)

	linkedInSummary := firstNonEmpty(contextLinkedIn(pkg), base)

	items := []SocialItem{
		{
			Platform:         "LinkedIn",
			Title:            headline,
			Summary:          truncateChars(linkedInSummary, SocialSummaryMax),
			Excerpt:          truncateWords(base, 40),
			Keywords:         topStrings(k.Primary, 6),
			Hashtags:         tags.LinkedIn,
			EngagementPrompt: "How would you have approached this? I'd love to hear your take.",
		},
		{
			Platform:         "X",
			Title:            headline,
			Summary:          truncateChars(base, SocialSummaryMax),
			Excerpt:          truncateWords(base, 25),
			Keywords:         topStrings(k.Primary, 5),
			Hashtags:         tags.X,
			EngagementPrompt: "What would you build with this?",
		},
		{
			Platform:         "Facebook",
			Title:            headline,
			Summary:          truncateChars(base, SocialSummaryMax),
			Excerpt:          truncateWords(base, 40),
			Keywords:         topStrings(k.Primary, 5),
			Hashtags:         topStrings(tags.All, 5),
			EngagementPrompt: "Have you tried something like this?",
		},
		{
			Platform:         "GitHub",
			Title:            fmt.Sprintf("Release %s", tag), // version-titled by convention
			Summary:          truncateChars(base, SocialSummaryMax),
			Excerpt:          truncateWords(base, 60),
			Keywords:         topStrings(k.Primary, 6),
			Hashtags:         nil, // GitHub releases don't use hashtags
			EngagementPrompt: "Issues and PRs welcome.",
		},
		{
			Platform:         "Developer Communities",
			Title:            headline,
			Summary:          truncateChars(base, SocialSummaryMax),
			Excerpt:          truncateWords(base, 50),
			Keywords:         topStrings(k.Technical, 6),
			Hashtags:         topStrings(tags.All, 4),
			EngagementPrompt: "Curious what the community would optimize first.",
		},
	}
	return SocialSEO{Items: items}
}

// socialHeadline is the grounded, topic-led headline for social posts: the blog
// title (the article's own headline). It never leads with the repository name or
// generic "release is out" language. Falls back to a grounded feature or repo+tag
// only when no blog title is available.
func socialHeadline(pkg ReleasePackage) string {
	if t := collapse(pkg.Blog.Title); t != "" {
		return t
	}
	if f := firstSentences(featureName(pkg), 1); !isGenericKeyword(f) {
		return f
	}
	return repoShortName(pkg) + " " + releaseTag(pkg)
}

// socialSummaryBase is the grounded summary source for social copy: the blog meta
// description (topic-rich) preferred over the generic release-stats summary.
func socialSummaryBase(pkg ReleasePackage) string {
	if d := collapse(pkg.Blog.MetaDescription); d != "" {
		return d
	}
	return summary(pkg)
}

// contextLinkedIn reuses a LinkedIn post the release context may already carry.
func contextLinkedIn(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.LinkedInPost != "" {
		return firstSentences(pkg.Context.ContentIntelligence.LinkedInPost, 2)
	}
	return ""
}
