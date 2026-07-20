package xthread

import "fmt"

// planCTA returns a concise, professional call to action grounded in the Release
// Context / SEO (blog canonical URL, repo URL).
func planCTA(pkg ReleasePackage, c threadCandidate) string {
	repoURLv := repoURL(pkg)
	blog := blogURL(pkg)

	switch c.Type {
	case "Architecture Walkthrough", "AWS Best Practices":
		if blog != "" {
			return "Full architecture write-up 👇 " + blog
		}
		if repoURLv != "" {
			return "Dig into the architecture: " + repoURLv
		}
	case "Implementation Deep Dive", "Feature Breakdown", "Developer Tips":
		if repoURLv != "" {
			return "Code's on GitHub 👉 " + repoURLv
		}
	case "Open Source Update":
		if repoURLv != "" {
			return "Star / contribute 👉 " + repoURLv
		}
	}
	if blog != "" {
		return "Full breakdown on the blog 👇 " + blog
	}
	if repoURLv != "" {
		return "Explore the repo 👉 " + repoURLv
	}
	return fmt.Sprintf("Follow for more on %s.", repoShort(pkg))
}
