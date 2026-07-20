package xthread

// planSummary returns a concise thread summary, grounded in the seed / release.
func planSummary(pkg ReleasePackage, c threadCandidate) string {
	seed := firstSentences(c.Seed, 1)
	if seed == "" {
		seed = summary(pkg)
	}
	return fitChars(seed, 200)
}
