package linkedin

// planCTA returns a concise, professional call to action. To keep post bodies
// EVERGREEN (no repo name or release version), the CTA points at the topic-slug
// blog write-up — never the repository URL, which carries the repo name. When no
// blog URL exists, a neutral text CTA is used that names nothing repo-specific.
func planCTA(pkg ReleasePackage, c postCandidate) string {
	blog := blogURL(pkg)
	if blog == "" {
		return "More AWS infrastructure patterns in the full write-up."
	}
	switch c.Type {
	case "Architecture Deep Dive", "AWS Best Practice":
		return "Full architecture write-up: " + blog
	case "Feature Spotlight", "Developer Productivity Tip":
		return "More detail in the write-up: " + blog
	case "Release Announcement", "Behind-the-Build":
		return "The full technical breakdown: " + blog
	default:
		return "Read the full write-up: " + blog
	}
}
