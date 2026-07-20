package linkedin

import "fmt"

// planCTA returns a concise, professional call to action. Links are grounded in
// the Release Context / SEO (repo URL, blog canonical). CTAs vary by type but
// always point to the long-form content or the repository.
func planCTA(pkg ReleasePackage, c postCandidate) string {
	repoURLv := repoURL(pkg)
	blog := blogURL(pkg)

	switch c.Type {
	case "Architecture Deep Dive", "AWS Best Practice":
		if blog != "" {
			return "Full architecture write-up: " + blog
		}
		if repoURLv != "" {
			return "Explore the architecture in the repo: " + repoURLv
		}
	case "Feature Spotlight", "Developer Productivity Tip":
		if repoURLv != "" {
			return "Try it — the repo is here: " + repoURLv
		}
	case "Release Announcement", "Behind-the-Build":
		if blog != "" {
			return "The full technical breakdown is on the blog: " + blog
		}
		if repoURLv != "" {
			return "Repo and release notes: " + repoURLv
		}
	}
	// Grounded generic fallback.
	if blog != "" {
		return "Read the full write-up: " + blog
	}
	if repoURLv != "" {
		return "Explore the repository: " + repoURLv
	}
	return fmt.Sprintf("Follow along as %s grows, release by release.", repoShort(pkg))
}
