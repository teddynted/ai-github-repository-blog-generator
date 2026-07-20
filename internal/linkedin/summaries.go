package linkedin

// planSummary returns a concise professional summary for a post, reusing the
// SEO LinkedIn summary when available, else a grounded fallback.
func planSummary(pkg ReleasePackage, c postCandidate) string {
	if s := seoLinkedInSummary(pkg); s != "" && c.Type == "Release Announcement" {
		return s
	}
	seed := firstSentences(c.Seed, 1)
	if seed == "" {
		seed = summary(pkg)
	}
	return truncateWords(collapse(seed), 40)
}

// seoLinkedInSummary pulls the LinkedIn summary the SEO engine already produced.
func seoLinkedInSummary(pkg ReleasePackage) string {
	for _, it := range pkg.SEO.Social.Items {
		if it.Platform == "LinkedIn" {
			return firstNonEmpty(it.Summary, it.Excerpt)
		}
	}
	return ""
}
